package amneziawgnet

import (
	"testing"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
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
