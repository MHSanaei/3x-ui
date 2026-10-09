package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestWriteUDPReplyIPv6MTU(t *testing.T) {
	for _, tc := range []struct {
		name       string
		size       int
		fragmented bool
	}{
		{"small", 80, false}, {"boundary", 1232, false}, {"fragmented", 1452, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("fd00::1")}, 1280)
			if err != nil {
				t.Fatal(err)
			}
			defer tun.Close()
			enablePromiscuousRouting(gstack)
			from := netip.MustParseAddrPort("[2001:db8::abcd]:5353")
			to := netip.MustParseAddrPort("[fd00::2]:49152")
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i*51 + 7)
			}
			if err := WriteUDPReply(gstack, from, to, payload); err != nil {
				t.Fatal(err)
			}
			var data []byte
			var id uint32
			count := 0
			for {
				select {
				case v := <-tun.(*stackTun).incomingPacket:
					packet := append([]byte(nil), v.AsSlice()...)
					v.Release()
					if len(packet) > 1280 {
						t.Fatalf("IPv6 TUN packet length %d exceeds MTU", len(packet))
					}
					ip := header.IPv6(packet)
					if ip.SourceAddress().String() != from.Addr().String() || ip.DestinationAddress().String() != to.Addr().String() || int(ip.PayloadLength())+40 != len(packet) {
						t.Fatalf("IPv6 addresses/length invalid: %s -> %s, %d", ip.SourceAddress(), ip.DestinationAddress(), ip.PayloadLength())
					}
					var offset int
					more := false
					part := packet[40:]
					if tc.fragmented {
						if ip.TransportProtocol() != header.IPv6FragmentHeader {
							t.Fatalf("IPv6 NextHeader %d, want fragment", ip.TransportProtocol())
						}
						frag := header.IPv6Fragment(part)
						if frag.NextHeader() != uint8(header.UDPProtocolNumber) {
							t.Fatalf("fragment next header %d", frag.NextHeader())
						}
						offset = int(frag.FragmentOffset()) * 8
						more = frag.More()
						if count == 0 {
							id = frag.ID()
						} else if frag.ID() != id {
							t.Fatalf("fragment ID %d != %d", frag.ID(), id)
						}
						part = part[8:]
					} else if ip.TransportProtocol() != header.UDPProtocolNumber {
						t.Fatalf("IPv6 NextHeader %d", ip.TransportProtocol())
					}
					if offset != len(data) {
						t.Fatalf("fragment offset %d, want %d", offset, len(data))
					}
					data = append(data, part...)
					count++
					if !more {
						goto assembled
					}
				case <-time.After(time.Second):
					t.Fatal("IPv6 reply timed out")
				}
			}
		assembled:
			if tc.fragmented && count < 2 {
				t.Fatal("no IPv6 fragments")
			}
			if len(data) != len(payload)+8 || binary.BigEndian.Uint16(data[:2]) != from.Port() || binary.BigEndian.Uint16(data[2:4]) != to.Port() || binary.BigEndian.Uint16(data[4:6]) != uint16(len(data)) || !bytes.Equal(data[8:], payload) {
				t.Fatal("reassembled IPv6 UDP bytes/ports/length differ")
			}
			if checksum.Checksum(data, header.PseudoHeaderChecksum(header.UDPProtocolNumber, tcpip.AddrFromSlice(from.Addr().AsSlice()), tcpip.AddrFromSlice(to.Addr().AsSlice()), uint16(len(data)))) != 0xffff {
				t.Fatal("IPv6 UDP checksum invalid")
			}
		})
	}
}

func TestWriteUDPReplyRejectsInvalidDatagrams(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.77.0.1"), netip.MustParseAddr("fd00::1")}, 1280)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	enablePromiscuousRouting(gstack)
	for _, tc := range []struct {
		name, from, to string
		size           int
		want           string
	}{
		{"mixed", "203.0.113.9:53", "[fd00::2]:4000", 1, "invalid UDP reply addresses"},
		{"zone", "[fe80::1%eth0]:53", "[fd00::2]:4000", 1, "invalid UDP reply addresses"},
		{"v4 length", "203.0.113.9:53", "10.77.0.2:4000", 65508, "IPv4 reply too large"},
		{"v6 length", "[2001:db8::1]:53", "[fd00::2]:4000", 65528, "UDP reply too large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := WriteUDPReply(gstack, netip.MustParseAddrPort(tc.from), netip.MustParseAddrPort(tc.to), make([]byte, tc.size))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			select {
			case v := <-tun.(*stackTun).incomingPacket:
				v.Release()
				t.Fatal("invalid reply emitted packet")
			default:
			}
		})
	}
}

func TestWriteUDPReplyIPv6ZeroChecksumIsEncodedAsOnes(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("fd00::1")}, 1280)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	enablePromiscuousRouting(gstack)
	from := netip.MustParseAddrPort("[2001:db8::abcd]:5353")
	to := netip.MustParseAddrPort("[fd00::2]:49152")
	src := tcpip.AddrFromSlice(from.Addr().AsSlice())
	dst := tcpip.AddrFromSlice(to.Addr().AsSlice())
	udp := make([]byte, 10)
	binary.BigEndian.PutUint16(udp[0:2], from.Port())
	binary.BigEndian.PutUint16(udp[2:4], to.Port())
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	pseudo := header.PseudoHeaderChecksum(header.UDPProtocolNumber, src, dst, uint16(len(udp)))
	var found bool
	for n := 0; n <= 65535; n++ {
		binary.BigEndian.PutUint16(udp[8:], uint16(n))
		if checksum.Checksum(udp, pseudo) == 0xffff {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no zero-complement checksum payload found")
	}
	if err := WriteUDPReply(gstack, from, to, udp[8:]); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-tun.(*stackTun).incomingPacket:
		packet := append([]byte(nil), v.AsSlice()...)
		v.Release()
		if got := binary.BigEndian.Uint16(packet[46:48]); got != 0xffff {
			t.Fatalf("IPv6 UDP checksum = %#04x, want 0xffff (zero is invalid)", got)
		}
	case <-time.After(time.Second):
		t.Fatal("reply timed out")
	}
}
