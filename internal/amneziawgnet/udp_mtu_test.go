package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// TestWriteUDPReplyMTU exercises the actual channel-backed TUN output, not a packet builder.
func TestWriteUDPReplyMTU(t *testing.T) {
	for _, tc := range []struct {
		name      string
		payload   int
		fragments bool
	}{
		{"small", 32, false}, {"boundary", 1252, false}, {"oversize", 1452, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.77.0.1")}, 1280)
			if err != nil {
				t.Fatal(err)
			}
			defer tun.Close()
			enablePromiscuousRouting(gstack)
			from := netip.MustParseAddrPort("203.0.113.9:5353")
			to := netip.MustParseAddrPort("10.77.0.2:49152")
			payload := make([]byte, tc.payload)
			for i := range payload {
				payload[i] = byte(i*37 + 11)
			}
			if err := WriteUDPReply(gstack, from, to, payload); err != nil {
				t.Fatal(err)
			}
			st := tun.(*stackTun)
			var data []byte
			var received int
			var id uint16
			for {
				select {
				case v := <-st.incomingPacket:
					p := v.AsSlice()
					packet := append([]byte(nil), p...)
					v.Release()
					if len(packet) > 1280 {
						t.Fatalf("TUN packet length %d exceeds MTU 1280", len(packet))
					}
					ip := header.IPv4(packet)
					if ip.SourceAddress().String() != from.Addr().String() || ip.DestinationAddress().String() != to.Addr().String() {
						t.Fatalf("IP addresses %s -> %s", ip.SourceAddress(), ip.DestinationAddress())
					}
					if ip.TotalLength() != uint16(len(packet)) || checksum.Checksum(packet[:ip.HeaderLength()], 0) != 0xffff {
						t.Fatalf("bad IPv4 header, len %d", len(packet))
					}
					if received == 0 {
						id = ip.ID()
					} else if id != ip.ID() {
						t.Fatalf("fragment IDs differ: %d != %d", id, ip.ID())
					}
					offset := int(ip.FragmentOffset())
					part := packet[ip.HeaderLength():]
					if offset != received {
						t.Fatalf("fragment offset %d, expected %d", offset, received)
					}
					data = append(data, part...)
					received += len(part)
					if ip.Flags()&header.IPv4FlagMoreFragments == 0 {
						if tc.fragments && offset == 0 {
							t.Fatal("oversize reply was not fragmented")
						}
						goto reassembled
					}
				case <-time.After(time.Second):
					t.Fatal("timed out waiting for TUN packets")
				}
			}
		reassembled:
			if len(data) != header.UDPMinimumSize+len(payload) {
				t.Fatalf("UDP bytes %d, want %d", len(data), 8+len(payload))
			}
			if binary.BigEndian.Uint16(data[:2]) != from.Port() || binary.BigEndian.Uint16(data[2:4]) != to.Port() || binary.BigEndian.Uint16(data[4:6]) != uint16(len(data)) {
				t.Fatalf("UDP ports/length incorrect: %x", data[:8])
			}
			if !bytes.Equal(data[8:], payload) {
				t.Fatal("UDP payload changed")
			}
			if checksum.Checksum(data, header.PseudoHeaderChecksum(header.UDPProtocolNumber, tcpip.AddrFromSlice(from.Addr().AsSlice()), tcpip.AddrFromSlice(to.Addr().AsSlice()), uint16(len(data)))) != 0xffff {
				t.Fatal("UDP checksum invalid")
			}
		})
	}
}
