package amneziawgnet

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestWriteUDPReplyConcurrentSources(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.77.0.1")}, 1280)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	enablePromiscuousRouting(gstack)
	const replies = 32
	to := netip.MustParseAddrPort("10.77.0.2:49152")
	var wg sync.WaitGroup
	errs := make(chan error, replies)
	for i := range replies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			from := netip.AddrPortFrom(netip.MustParseAddr("203.0.113.9"), uint16(10000+i))
			var payload [2]byte
			binary.BigEndian.PutUint16(payload[:], uint16(i))
			errs <- WriteUDPReply(gstack, from, to, payload[:])
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[uint16]bool)
	for range replies {
		select {
		case v := <-tun.(*stackTun).incomingPacket:
			packet := append([]byte(nil), v.AsSlice()...)
			v.Release()
			if len(packet) > 1280 {
				t.Fatalf("packet length %d", len(packet))
			}
			ip := header.IPv4(packet)
			udp := packet[ip.HeaderLength():]
			port := binary.BigEndian.Uint16(udp[0:2])
			index := binary.BigEndian.Uint16(udp[8:10])
			if index >= replies || port != 10000+index || seen[index] || binary.BigEndian.Uint16(udp[2:4]) != to.Port() {
				t.Fatalf("duplicate/corrupt concurrent reply: port %d index %d", port, index)
			}
			seen[index] = true
		case <-time.After(time.Second):
			t.Fatal("concurrent replies missing")
		}
	}
}
