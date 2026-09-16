package tuic

import (
	"bytes"
	"net"
	"reflect"
	"testing"
)

func TestAddressEncodingDecoding(t *testing.T) {
	tests := []struct {
		name string
		addr *Address
	}{
		{
			name: "IPv4",
			addr: &Address{
				Type: AddrTypeIPv4,
				IP:   net.ParseIP("1.2.3.4").To4(),
				Host: "1.2.3.4",
				Port: 443,
			},
		},
		{
			name: "IPv6",
			addr: &Address{
				Type: AddrTypeIPv6,
				IP:   net.ParseIP("2001:db8::1"),
				Host: "2001:db8::1",
				Port: 8080,
			},
		},
		{
			name: "Domain",
			addr: &Address{
				Type: AddrTypeDomain,
				Host: "example.com",
				Port: 8443,
			},
		},
		{
			name: "None",
			addr: &Address{
				Type: AddrTypeNone,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteAddress(&buf, tc.addr); err != nil {
				t.Fatalf("WriteAddress error: %v", err)
			}

			decoded, err := ReadAddress(&buf)
			if err != nil {
				t.Fatalf("ReadAddress error: %v", err)
			}

			if tc.addr.Type == AddrTypeNone {
				if decoded.Type != AddrTypeNone {
					t.Fatalf("expected None type, got %v", decoded.Type)
				}
				return
			}

			if decoded.Type != tc.addr.Type {
				t.Errorf("Type mismatch: got %v, want %v", decoded.Type, tc.addr.Type)
			}
			if decoded.Port != tc.addr.Port {
				t.Errorf("Port mismatch: got %v, want %v", decoded.Port, tc.addr.Port)
			}
			if tc.addr.Type == AddrTypeDomain {
				if decoded.Host != tc.addr.Host {
					t.Errorf("Host mismatch: got %v, want %v", decoded.Host, tc.addr.Host)
				}
			} else {
				if !decoded.IP.Equal(tc.addr.IP) {
					t.Errorf("IP mismatch: got %v, want %v", decoded.IP, tc.addr.IP)
				}
			}
		})
	}
}

func TestCommandHeader(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x05, 0x01})
	ver, cmd, err := ReadCommand(buf)
	if err != nil {
		t.Fatalf("ReadCommand error: %v", err)
	}
	if ver != ProtocolVersion || cmd != CmdConnect {
		t.Fatalf("got ver=%d, cmd=%d; want ver=5, cmd=1", ver, cmd)
	}

	invalidBuf := bytes.NewBuffer([]byte{0x04, 0x01})
	_, _, err = ReadCommand(invalidBuf)
	if err == nil {
		t.Fatal("expected error on invalid version, got nil")
	}
}

func TestPacketHeaderAndPayload(t *testing.T) {
	var buf bytes.Buffer
	target := &Address{
		Type: AddrTypeDomain,
		Host: "dns.google",
		Port: 53,
	}
	payload := []byte("hello-udp")

	err := WritePacket(&buf, 100, 1, 1, 0, target, payload)
	if err != nil {
		t.Fatalf("WritePacket error: %v", err)
	}

	ver, cmd, err := ReadCommand(&buf)
	if err != nil {
		t.Fatalf("ReadCommand error: %v", err)
	}
	if ver != ProtocolVersion || cmd != CmdPacket {
		t.Fatalf("got ver=%d cmd=%d, want 5 and 2", ver, cmd)
	}

	ph, err := ReadPacketHeader(&buf)
	if err != nil {
		t.Fatalf("ReadPacketHeader error: %v", err)
	}

	if ph.AssocID != 100 || ph.PktID != 1 || ph.FragTotal != 1 || ph.FragID != 0 {
		t.Fatalf("PacketHeader mismatch: %+v", ph)
	}
	if ph.Addr.Host != "dns.google" || ph.Addr.Port != 53 {
		t.Fatalf("Packet address mismatch: %+v", ph.Addr)
	}

	readPayload := make([]byte, ph.Size)
	if _, err := buf.Read(readPayload); err != nil {
		t.Fatalf("reading payload error: %v", err)
	}
	if !reflect.DeepEqual(readPayload, payload) {
		t.Fatalf("payload mismatch: got %s, want %s", readPayload, payload)
	}
}
