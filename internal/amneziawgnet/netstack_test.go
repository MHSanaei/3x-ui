package amneziawgnet

import (
	"bytes"
	"context"
	"io"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	awgtun "github.com/amnezia-vpn/amneziawg-go/v3/tun"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/seqnum"
)

// TestStackTunReadDrainsBufferedBatch is a regression test for a real
// throughput bug: Read used to always return exactly one packet per call
// no matter how many were already queued, forcing amneziawg-go's TUN
// reader to pay a full peer-lookup+staging+syscall cycle per packet on the
// download path while the upload path (via the UDP bind's own
// recvmmsg/sendmmsg batching) amortized that cost across up to 128
// packets. Confirmed live: this alone took real download throughput from
// 30-40 Mbit/s to 130-250 Mbit/s on a real test connection (see commit
// 6436fd9c's message and internal/amneziawgnet/netstack.go's own comment
// on tunQueueDepth for the full story) -- this test locks in the second,
// finer-grained fix on top of that: Read must actually drain what's
// already buffered instead of returning after the first packet.
func TestStackTunReadDrainsBufferedBatch(t *testing.T) {
	t.Parallel()

	tun := &stackTun{incomingPacket: make(chan *buffer.View, tunQueueDepth)}
	packets := [][]byte{{1, 2, 3}, {4, 5}, {6, 7, 8, 9}}
	for _, p := range packets {
		tun.incomingPacket <- buffer.NewViewWithData(p)
	}

	buf := make([][]byte, 8)
	sizes := make([]int, 8)
	for i := range buf {
		buf[i] = make([]byte, 64)
	}

	n, err := tun.Read(buf, sizes, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(packets) {
		t.Fatalf("Read returned %d packets, want %d (all buffered packets in one call)", n, len(packets))
	}
	for i, want := range packets {
		got := buf[i][:sizes[i]]
		if string(got) != string(want) {
			t.Errorf("packet %d = %v, want %v", i, got, want)
		}
	}
}

// TestStackTunReadStopsAtBufCapacity confirms Read never returns more
// packets than the caller's buf can hold, and that whatever didn't fit is
// still there (in order) for the next call -- draining must respect the
// caller's batch size, not just gulp everything queued.
func TestStackTunReadStopsAtBufCapacity(t *testing.T) {
	t.Parallel()

	tun := &stackTun{incomingPacket: make(chan *buffer.View, tunQueueDepth)}
	packets := [][]byte{{1}, {2}, {3}}
	for _, p := range packets {
		tun.incomingPacket <- buffer.NewViewWithData(p)
	}

	buf := make([][]byte, 2)
	sizes := make([]int, 2)
	for i := range buf {
		buf[i] = make([]byte, 64)
	}

	n, err := tun.Read(buf, sizes, 0)
	if err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if n != 2 {
		t.Fatalf("first Read returned %d, want 2 (buf capacity)", n)
	}

	n, err = tun.Read(buf, sizes, 0)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if n != 1 {
		t.Fatalf("second Read returned %d, want 1 (the leftover packet)", n)
	}
	if got := buf[0][:sizes[0]]; string(got) != "\x03" {
		t.Errorf("leftover packet = %v, want [3]", got)
	}
}

// TestStackTunWriteReturnsPacketBuffersToPool locks in gVisor's ownership rule
// for the upload path: whoever calls InjectInbound must DecRef the packet.
func TestStackTunWriteReturnsPacketBuffersToPool(t *testing.T) {
	tun := &stackTun{ep: channel.New(tunQueueDepth, 1420, ""), mtu: 1420}
	defer tun.ep.Close()

	packet := make([]byte, 1400)
	packet[0] = 0x45 // IPv4, version nibble is all Write inspects
	bufs := [][]byte{packet}

	allocs := testing.AllocsPerRun(1000, func() {
		if _, err := tun.Write(bufs, 0); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	// 0 once pooled, 4 when every packet buffer is stranded; -race adds ~1.
	if allocs > 1 {
		t.Fatalf("Write allocates %v times per packet, want <=1: injected packet buffers are not being returned to gVisor's pools", allocs)
	}
}

// TestStackTunReadReturnsViewsToPool is the download-path counterpart: a view
// that is copied out but never released strands its pooled chunk.
func TestStackTunReadReturnsViewsToPool(t *testing.T) {
	tun := &stackTun{incomingPacket: make(chan *buffer.View, tunQueueDepth)}
	packet := make([]byte, 1400)
	buf := [][]byte{make([]byte, 2048)}
	sizes := make([]int, 1)

	allocs := testing.AllocsPerRun(1000, func() {
		tun.incomingPacket <- buffer.NewViewWithData(packet)
		if _, err := tun.Read(buf, sizes, 0); err != nil {
			t.Fatalf("Read: %v", err)
		}
	})
	// 0 once the drained view goes back to viewPool, 3 when it does not.
	if allocs > 1 {
		t.Fatalf("Read allocates %v times per packet, want <=1: drained views are not being released", allocs)
	}
}

// TestStackTunWriteSurvivesFullOutboundQueue: Write runs on amneziawg-go's per-peer
// receiver, so a reply the stack cannot queue must be dropped rather than wait for Read.
func TestStackTunWriteSurvivesFullOutboundQueue(t *testing.T) {
	tun, _, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.9.0.1")}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	defer tun.Close()

	ping := make([]byte, header.IPv4MinimumSize+header.ICMPv4MinimumSize)
	ip := header.IPv4(ping)
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(ping)),
		TTL:         64,
		Protocol:    uint8(header.ICMPv4ProtocolNumber),
		SrcAddr:     tcpip.AddrFrom4([4]byte{10, 9, 0, 2}),
		DstAddr:     tcpip.AddrFrom4([4]byte{10, 9, 0, 1}),
	})
	ip.SetChecksum(^ip.CalculateChecksum())
	icmp := header.ICMPv4(ping[header.IPv4MinimumSize:])
	icmp.SetType(header.ICMPv4Echo)
	icmp.SetChecksum(^checksum.Checksum(icmp, 0))

	// Nobody calls Read, so the echo replies fill incomingPacket and overflow it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range tunQueueDepth + 8 {
			_, _ = tun.Write([][]byte{ping}, 0)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Write is still waiting for room in the outbound queue: the receiver would deadlock a rebind")
	}
	if got := len(tun.(*stackTun).incomingPacket); got != tunQueueDepth {
		t.Fatalf("incomingPacket holds %d replies, want %d: the queue never filled", got, tunQueueDepth)
	}
}

// tailBlackoutTun drops all server data from the hole on, tail probes included, until the
// sender's RTO resends the hole, as an outage on the way that outlasts the RTO does.
type tailBlackoutTun struct {
	awgtun.Device
	after, passed  int
	timeouts       func() uint64
	holeSeq        seqnum.Value
	started, over  bool
	timeoutsAtHole uint64
	dropped        atomic.Int64
}

func (b *tailBlackoutTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	for {
		n, err := b.Device.Read(bufs, sizes, offset)
		if err != nil {
			return n, err
		}
		kept := 0
		for i := range n {
			if b.drop(bufs[i][offset : offset+sizes[i]]) {
				continue
			}
			bufs[kept], bufs[i] = bufs[i], bufs[kept]
			sizes[kept] = sizes[i]
			kept++
		}
		if kept > 0 {
			return kept, nil
		}
	}
}

func (b *tailBlackoutTun) drop(pkt []byte) bool {
	ip := header.IPv4(pkt)
	if !ip.IsValid(len(pkt)) || ip.TransportProtocol() != header.TCPProtocolNumber {
		return false
	}
	seg := header.TCP(ip.Payload())
	n, seq := len(seg.Payload()), seqnum.Value(seg.SequenceNumber())
	switch {
	case n == 0 || b.over:
		return false
	case !b.started:
		if b.passed < b.after {
			b.passed += n
			return false
		}
		b.started, b.holeSeq, b.timeoutsAtHole = true, seq, b.timeouts()
	case seq == b.holeSeq && b.timeouts() > b.timeoutsAtHole:
		b.over = true
		return false
	}
	b.dropped.Add(1)
	return true
}

// delayShuttle forwards src to dst, each packet delay later, so RACK sees a real RTT; the
// returned channel closes once delivery has stopped for good after stop.
func delayShuttle(stop <-chan struct{}, src, dst awgtun.Device, delay time.Duration) <-chan struct{} {
	type queued struct {
		pkt []byte
		at  time.Time
	}
	line := make(chan queued, tunQueueDepth)
	delivered := make(chan struct{})
	go func() {
		defer close(delivered)
		for {
			select {
			case <-stop:
				return
			case q := <-line:
				select {
				case <-stop:
					return
				case <-time.After(time.Until(q.at)):
				}
				if _, err := dst.Write([][]byte{q.pkt}, 0); err != nil {
					return
				}
			}
		}
	}()
	go func() {
		bufs, sizes := [][]byte{make([]byte, 65536)}, make([]int, 1)
		for {
			n, err := src.Read(bufs, sizes, 0)
			if err != nil {
				return
			}
			for i := range n {
				select {
				case line <- queued{pkt: bytes.Clone(bufs[i][:sizes[i]]), at: time.Now().Add(delay)}:
				case <-stop:
					return
				}
			}
		}
	}()
	return delivered
}

// After a blackout gVisor's RACK kept the lost tail in pipe and resent it 2 segments per
// RTO, so downloads through the tunnel froze for seconds.
func TestTCPDownloadRecoversTailBlackoutWithinOneRTO(t *testing.T) {
	const (
		payloadSize   = 2 << 20
		blackoutAfter = 512 << 10
		oneWayDelay   = 10 * time.Millisecond
		minLostTail   = 150
		deadline      = 30 * time.Second
	)
	stun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.77.0.2")}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	ctun, cnet, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr("10.77.0.1")}, nil, 1420)
	if err != nil {
		t.Fatalf("client netstack: %v", err)
	}

	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	AttachTCPForwarder(gstack, ownerBySource, func(conn *gonet.TCPConn, _, _ netip.AddrPort) {
		defer conn.Close()
		_, _ = conn.Write(payload)
		_, _ = io.Copy(io.Discard, conn)
	})
	timeouts := gstack.Stats().TCP.Timeouts.Value
	lossy := &tailBlackoutTun{Device: stun, after: blackoutAfter, timeouts: timeouts}
	stop := make(chan struct{})
	up := delayShuttle(stop, ctun, stun, oneWayDelay)
	down := delayShuttle(stop, lossy, ctun, oneWayDelay)
	defer func() {
		close(stop)
		<-up
		<-down
		_ = ctun.Close()
		_ = stun.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := cnet.DialContextTCPAddrPort(ctx, netip.MustParseAddrPort("203.0.113.7:443"))
	if err != nil {
		t.Fatalf("dial through the tunnel: %v", err)
	}
	defer conn.Close()

	start := time.Now()
	_ = conn.SetReadDeadline(start.Add(deadline))
	got := make([]byte, payloadSize)
	n, err := io.ReadFull(conn, got)
	if err != nil {
		t.Fatalf("received %d of %d bytes in %v (%d segments blacked out, %d RTOs): the lost tail is resent an RTO at a time: %v",
			n, payloadSize, time.Since(start), lossy.dropped.Load(), timeouts(), err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("download arrived corrupted or out of order")
	}
	if d := lossy.dropped.Load(); d < minLostTail {
		t.Fatalf("blackout dropped %d data segments, want >= %d: the lost tail no longer exceeds RACK's recovery cwnd", d, minLostTail)
	}
	if got := timeouts(); got != 1 {
		t.Fatalf("recovery took %d RTOs, want 1: after the RTO the tail must be resent without waiting for more", got)
	}
	t.Logf("recovered a %d-segment tail in %v", lossy.dropped.Load(), time.Since(start))
}
