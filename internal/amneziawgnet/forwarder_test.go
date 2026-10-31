package amneziawgnet

import (
	"net"
	"net/netip"
	"testing"
	"time"

	awgtun "github.com/amnezia-vpn/amneziawg-go/v3/tun"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func rawTCPSegment(src, dst netip.AddrPort, seq, ack uint32, flags header.TCPFlags) []byte {
	srcIP, dstIP := tcpip.AddrFromSlice(src.Addr().AsSlice()), tcpip.AddrFromSlice(dst.Addr().AsSlice())
	packet := make([]byte, header.IPv4MinimumSize+header.TCPMinimumSize)
	ip := header.IPv4(packet)
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(packet)),
		TTL:         64,
		Protocol:    uint8(header.TCPProtocolNumber),
		SrcAddr:     srcIP,
		DstAddr:     dstIP,
	})
	ip.SetChecksum(^ip.CalculateChecksum())
	tcp := header.TCP(packet[header.IPv4MinimumSize:])
	tcp.Encode(&header.TCPFields{
		SrcPort:    src.Port(),
		DstPort:    dst.Port(),
		SeqNum:     seq,
		AckNum:     ack,
		DataOffset: header.TCPMinimumSize,
		Flags:      flags,
		WindowSize: 65535,
	})
	xsum := header.PseudoHeaderChecksum(header.TCPProtocolNumber, srcIP, dstIP, header.TCPMinimumSize)
	tcp.SetChecksum(^tcp.CalculateChecksum(checksum.Checksum(nil, xsum)))
	return packet
}

func injectFromPeer(t *testing.T, tun awgtun.Device, packet []byte) {
	t.Helper()
	if _, err := tun.Write([][]byte{packet}, 0); err != nil {
		t.Fatalf("inject packet: %v", err)
	}
}

// A peer's RST can beat the handler, and a reset endpoint has no RemoteAddr:
// deriving the source from it panicked the panel on a goroutine with no recover.
func TestTCPForwarderHandsOverSourceOfAnAlreadyResetConnection(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.9.0.1")}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	defer tun.Close()

	peer := netip.MustParseAddrPort("10.9.0.2:40000")
	target := netip.MustParseAddrPort("203.0.113.7:443")

	type handedOver struct {
		src, dest  netip.AddrPort
		remoteAddr net.Addr
	}
	accepted, resetSent := make(chan struct{}), make(chan struct{})
	got := make(chan handedOver, 1)
	AttachTCPForwarder(gstack, ownerBySource, func(conn *gonet.TCPConn, src, dest netip.AddrPort) {
		defer conn.Close()
		close(accepted)
		<-resetSent
		// Returns only once the stack has processed the RST.
		_, _ = conn.Read(make([]byte, 1))
		got <- handedOver{src: src, dest: dest, remoteAddr: conn.RemoteAddr()}
	})

	const peerISN = 1000
	injectFromPeer(t, tun, rawTCPSegment(peer, target, peerISN, 0, header.TCPFlagSyn))
	bufs, sizes := [][]byte{make([]byte, 2048)}, make([]int, 1)
	if _, err := tun.Read(bufs, sizes, 0); err != nil {
		t.Fatalf("read SYN-ACK: %v", err)
	}
	synAck := header.TCP(bufs[0][header.IPv4MinimumSize:sizes[0]])
	if synAck.Flags() != header.TCPFlagSyn|header.TCPFlagAck {
		t.Fatalf("first reply has flags %s, want SYN-ACK", synAck.Flags())
	}
	injectFromPeer(t, tun, rawTCPSegment(peer, target, peerISN+1, synAck.SequenceNumber()+1, header.TCPFlagAck))
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never ran")
	}
	injectFromPeer(t, tun, rawTCPSegment(peer, target, peerISN+1, 0, header.TCPFlagRst))
	close(resetSent)

	select {
	case h := <-got:
		if h.remoteAddr != nil {
			t.Fatalf("RemoteAddr() = %v after the RST, want nil: the test no longer reaches the reset state", h.remoteAddr)
		}
		if h.src != peer {
			t.Errorf("src = %s, want %s", h.src, peer)
		}
		if h.dest != target {
			t.Errorf("dest = %s, want %s", h.dest, target)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler never saw the reset")
	}
}

// ownerBySource counts every tunnel source address as a peer of its own.
func ownerBySource(src netip.Addr) (string, bool) {
	return src.String(), true
}

// The forwarder's in-flight limit is shared by the interface, so one peer's bare SYNs used
// to take every slot and leave the other clients' connections unanswered.
func TestTCPForwarderPeerCannotStarveOthers(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.8.1.1")}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	defer tun.Close()

	flooderRange := netip.MustParsePrefix("10.8.2.0/24")
	AttachTCPForwarder(gstack, func(src netip.Addr) (string, bool) {
		if flooderRange.Contains(src) {
			return "flooder", true
		}
		return "other", true
	}, func(conn *gonet.TCPConn, _, _ netip.AddrPort) { conn.Close() })

	other := netip.MustParseAddrPort("10.8.1.3:40000")
	answered := make(chan struct{})
	go func() {
		bufs, sizes := [][]byte{make([]byte, 2048)}, []int{0}
		for {
			if _, err := tun.Read(bufs, sizes, 0); err != nil {
				return
			}
			reply := header.IPv4(bufs[0][:sizes[0]])
			seg := header.TCP(reply.Payload())
			if reply.DestinationAddress() == tcpip.AddrFrom4(other.Addr().As4()) && seg.Flags() == header.TCPFlagSyn|header.TCPFlagAck {
				close(answered)
				return
			}
		}
	}()

	dst := netip.MustParseAddrPort("10.8.1.1:80")
	for n := range 70000 {
		src := netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 8, 2, byte(n)}), uint16(1024+n/256))
		injectFromPeer(t, tun, rawTCPSegment(src, dst, 1000, 0, header.TCPFlagSyn))
	}
	injectFromPeer(t, tun, rawTCPSegment(other, dst, 1000, 0, header.TCPFlagSyn))
	select {
	case <-answered:
	case <-time.After(5 * time.Second):
		t.Fatalf("no SYN-ACK for %s; dropped at the in-flight limit: %d", other, gstack.Stats().TCP.ForwardMaxInFlightDrop.Value())
	}
}
