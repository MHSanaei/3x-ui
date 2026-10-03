package tuic

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	clientquic "github.com/quic-go/quic-go"
)

func audit3RestartableSOCKS(t *testing.T) (string, *net.UDPConn, *net.UDPAddr) {
	t.Helper()
	u, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	udpAddr := u.LocalAddr().(*net.UDPAddr)
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = u.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var greet [4]byte
				if _, err := io.ReadFull(c, greet[:]); err != nil {
					return
				}
				if _, err := c.Write([]byte{5, 0}); err != nil {
					return
				}
				var req [10]byte
				if _, err := io.ReadFull(c, req[:]); err != nil {
					return
				}
				reply := []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}
				binary.BigEndian.PutUint16(reply[8:], uint16(udpAddr.Port))
				if _, err := c.Write(reply); err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, c)
			}(c)
		}
	}()
	return ln.Addr().String(), u, udpAddr
}

func audit3StartUDPEcho(u *net.UDPConn, arrived chan<- struct{}) {
	go func() {
		buf := make([]byte, 2048)
		for {
			n, src, err := u.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = u.WriteToUDP(buf[:n], src)
			if arrived != nil {
				select {
				case arrived <- struct{}{}:
				default:
				}
			}
		}
	}()
}

func audit3SendPacket(t *testing.T, c *clientquic.Conn, mode uint8, assoc, pkt uint16, payload string) {
	t.Helper()
	var b bytes.Buffer
	if err := WritePacket(&b, assoc, pkt, 1, 0, &Address{Type: AddrTypeIPv4, IP: net.IPv4(8, 8, 8, 8), Port: 53}, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	if mode == packetTransportDatagram {
		if err := c.SendDatagram(b.Bytes()); err != nil {
			t.Fatal(err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, err := c.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
}

func audit3ReceivePacket(c *clientquic.Conn, mode uint8, duration time.Duration) (*PacketHeader, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	var r io.Reader
	if mode == packetTransportDatagram {
		b, err := c.ReceiveDatagram(ctx)
		if err != nil {
			return nil, nil, err
		}
		r = bytes.NewReader(b)
	} else {
		s, err := c.AcceptUniStream(ctx)
		if err != nil {
			return nil, nil, err
		}
		defer s.CancelRead(0)
		_ = s.SetReadDeadline(time.Now().Add(duration))
		r = s
	}
	if _, _, err := ReadCommand(r); err != nil {
		return nil, nil, err
	}
	h, err := ReadPacketHeader(r)
	if err != nil {
		return nil, nil, err
	}
	p, err := readPacketPayload(r, h)
	return h, p, err
}

func TestAudit3UDPAssociationMustRecoverAfterBridgeReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Go disables SIO_UDP_CONNRESET on Windows, so a dead UDP bridge never fails a read there")
	}
	for _, mode := range []uint8{packetTransportDatagram, packetTransportStream} {
		name := "datagram"
		if mode == packetTransportStream {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			relayAddr, u, udpAddr := audit3RestartableSOCKS(t)
			audit3StartUDPEcho(u, nil)
			s, c, id, _ := startLifecycleTestServer(t, relayAddr, "audit3-recovery@x")
			_, user := authenticatedServerConnection(t, s, id)
			audit3SendPacket(t, c, mode, 42131, 1, "before")
			if _, p, err := audit3ReceivePacket(c, mode, time.Second); err != nil || string(p) != "before" {
				t.Fatalf("initial echo %q %v", p, err)
			}

			s.UpdateRuntimeSettings("recovery-"+name, "bbr", "warn")
			_ = u.Close()
			audit3SendPacket(t, c, mode, 42131, 2, "while-down")
			deadline := time.Now().Add(2 * time.Second)
			for {
				found := false
				for _, line := range logger.GetLogs(10000, "DEBUG") {
					if strings.Contains(line, "recovery-"+name) && strings.Contains(line, "UDP relay receive failed") {
						found = true
						break
					}
				}
				if found {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("closed UDP bridge did not terminate the response reader")
				}
				time.Sleep(time.Millisecond)
			}
			u2, err := net.ListenUDP("udp4", udpAddr)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = u2.Close() })
			arrived := make(chan struct{}, 4)
			audit3StartUDPEcho(u2, arrived)
			audit3SendPacket(t, c, mode, 42131, 3, "after")
			select {
			case <-arrived:
			case <-time.After(time.Second):
				t.Fatal("restarted bridge did not receive the retained association request")
			}
			_, payload, oldErr := audit3ReceivePacket(c, mode, 300*time.Millisecond)
			// A different association proves QUIC, the restarted SOCKS bridge, and both transport modes remain functional.
			audit3SendPacket(t, c, mode, 42132, 4, "fresh")
			h, p, newErr := audit3ReceivePacket(c, mode, time.Second)
			if newErr != nil || h.AssocID != 42132 || string(p) != "fresh" {
				t.Fatalf("fresh association probe %v %q %v", h, p, newErr)
			}
			if oldErr != nil || string(payload) != "after" {
				t.Fatalf("retained association became receive blackhole after read failure: response %q err=%v; fresh association works; user up=%d down=%d", payload, oldErr, user.Traffic.BytesUp.Load(), user.Traffic.BytesDown.Load())
			}
		})
	}
}

func TestAudit3FirstTransportReturnsPositiveEchoAfterOppositeModePacket(t *testing.T) {
	for _, first := range []uint8{packetTransportDatagram, packetTransportStream} {
		name := "datagram-first"
		if first == packetTransportStream {
			name = "stream-first"
		}
		t.Run(name, func(t *testing.T) {
			relayAddr, u, _ := audit3RestartableSOCKS(t)
			audit3StartUDPEcho(u, nil)
			_, c, _, _ := startLifecycleTestServer(t, relayAddr, "audit3-first-mode@x")
			audit3SendPacket(t, c, first, 43221, 1, "first")
			if _, p, err := audit3ReceivePacket(c, first, time.Second); err != nil || string(p) != "first" {
				t.Fatalf("initial mode echo %q %v", p, err)
			}
			audit3SendPacket(t, c, 1-first, 43221, 2, "opposite")
			if _, p, err := audit3ReceivePacket(c, first, time.Second); err != nil || string(p) != "opposite" {
				t.Fatalf("opposite request response did not keep first transport: %q %v", p, err)
			}
		})
	}
}

func TestUDPReaderCleanupCannotDeleteReplacementAssociation(t *testing.T) {
	registry := newUdpAssociationRegistry(1500)
	header := &PacketHeader{AssocID: 7, FragTotal: 1, Size: 1, Addr: &Address{Type: AddrTypeIPv4, IP: net.IPv4(1, 1, 1, 1), Port: 53}}
	old, _, _, complete := registry.feed(packetTransportDatagram, header, []byte("a"))
	if !complete {
		t.Fatal("first packet incomplete")
	}
	registry.dissociate(7)
	replacement, _, _, complete := registry.feed(packetTransportStream, header, []byte("b"))
	if !complete || replacement == old {
		t.Fatal("association generation was reused")
	}
	registry.release(7, old)
	if !registry.touch(7, replacement, time.Now()) {
		t.Fatal("late reader cleanup deleted the new association")
	}
	registry.release(7, replacement)
	if registry.touch(7, replacement, time.Now()) {
		t.Fatal("current reader cleanup retained dead association")
	}
}
