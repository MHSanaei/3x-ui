package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func echoTestPacket(src, dst string, id, seq uint16, payload []byte) []byte {
	p := make([]byte, 28+len(payload))
	p[0] = 0x45
	p[8] = 64
	p[9] = 1
	binary.BigEndian.PutUint16(p[2:4], uint16(len(p)))
	s, d := netip.MustParseAddr(src).As4(), netip.MustParseAddr(dst).As4()
	copy(p[12:16], s[:])
	copy(p[16:20], d[:])
	p[20] = 8
	binary.BigEndian.PutUint16(p[24:26], id)
	binary.BigEndian.PutUint16(p[26:28], seq)
	copy(p[28:], payload)
	binary.BigEndian.PutUint16(p[22:24], echoChecksum(p[20:]))
	binary.BigEndian.PutUint16(p[10:12], echoChecksum(p[:20]))
	return p
}

func TestEchoParserRejectsUntrustedPackets(t *testing.T) {
	good := echoTestPacket("192.0.2.2", "198.51.100.7", 123, 17, []byte("monitor"))
	for name, mutate := range map[string]func([]byte) []byte{
		"truncated":              func(p []byte) []byte { return p[:27] },
		"length":                 func(p []byte) []byte { p[3]--; return p },
		"ip-checksum":            func(p []byte) []byte { p[8]--; return p },
		"icmp-checksum":          func(p []byte) []byte { p[28]++; return p },
		"fragment":               func(p []byte) []byte { p[6] = 0x20; return p },
		"fragment-offset":        func(p []byte) []byte { p[7] = 1; return p },
		"reserved-fragment-flag": func(p []byte) []byte { p[6] = 0x80; return p },
		"options":                func(p []byte) []byte { p[0] = 0x46; return p },
		"ipv6":                   func(p []byte) []byte { p[0] = 0x60; return p },
		"non-echo":               func(p []byte) []byte { p[20] = 3; return p },
		"code":                   func(p []byte) []byte { p[21] = 1; return p },
		"oversize":               func(p []byte) []byte { return append(p, make([]byte, EchoMaxPacket)...) },
	} {
		t.Run(name, func(t *testing.T) {
			packet := mutate(bytes.Clone(good))
			switch name {
			case "fragment", "fragment-offset", "reserved-fragment-flag", "length":
				packet[10] = 0
				packet[11] = 0
				binary.BigEndian.PutUint16(packet[10:12], echoChecksum(packet[:20]))
			case "non-echo", "code":
				packet[22] = 0
				packet[23] = 0
				binary.BigEndian.PutUint16(packet[22:24], echoChecksum(packet[20:]))
			case "oversize":
				binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
				packet[10] = 0
				packet[11] = 0
				binary.BigEndian.PutUint16(packet[10:12], echoChecksum(packet[:20]))
			}
			if _, err := parseEcho(packet); err == nil {
				t.Fatal("accepted malformed or unsupported packet")
			}
		})
	}
	req, err := parseEcho(good)
	if err != nil {
		t.Fatal(err)
	}
	if req.src.String() != "192.0.2.2" || req.dst.String() != "198.51.100.7" || !bytes.Equal(req.message[8:], []byte("monitor")) {
		t.Fatalf("lost request: %#v", req)
	}
}

type echoFakeSocket struct {
	writes chan []byte
	done   chan struct{}
	once   sync.Once
}

func newEchoFakeSocket() *echoFakeSocket {
	return &echoFakeSocket{writes: make(chan []byte, 8192), done: make(chan struct{})}
}

func (s *echoFakeSocket) ReadFrom([]byte) (int, net.Addr, error) {
	<-s.done
	return 0, nil, net.ErrClosed
}

func (s *echoFakeSocket) WriteTo(p []byte, _ net.Addr) (int, error) {
	s.writes <- bytes.Clone(p)
	return len(p), nil
}
func (s *echoFakeSocket) Close() error { s.once.Do(func() { close(s.done) }); return nil }

func testEchoRelay(t *testing.T, gate EchoGate) (*echoRelay, *echoFakeSocket, chan []byte) {
	t.Helper()
	socket := newEchoFakeSocket()
	replies := make(chan []byte, 4096)
	lookup := func(a netip.Addr) (amneziawg.Peer, bool) {
		if a.String() == "192.0.2.2" || a.String() == "192.0.2.3" {
			return amneziawg.Peer{Email: t.Name() + a.String(), PublicKey: "synthetic-" + a.String()}, true
		}
		return amneziawg.Peer{}, false
	}
	relay := newEchoRelay(7, gate, lookup, func(p []byte) bool { replies <- bytes.Clone(p); return true }, func(a netip.Addr) bool { return a.String() == "192.0.2.1" })
	relay.open = func() (echoSocket, error) { return socket, nil }
	t.Cleanup(func() {
		relay.close()
		select {
		case <-relay.workerDone:
		case <-time.After(time.Second):
			t.Error("worker did not stop")
		}
		relay.wg.Wait()
	})
	return relay, socket, replies
}

func echoResponse(wire []byte) []byte {
	p := bytes.Clone(wire)
	p[0] = 0
	p[2] = 0
	p[3] = 0
	binary.BigEndian.PutUint16(p[2:4], echoChecksum(p))
	return p
}

func TestEchoRealReplyCorrelationAndIdentity(t *testing.T) {
	var up, down int64
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, u, d int64, expected string) (string, error) {
		if expected != "" && expected != "identity-v1" {
			return "", errors.New("wrong identity")
		}
		up += u
		down += d
		return "identity-v1", nil
	}
	relay, socket, replies := testEchoRelay(t, gate)
	payload := []byte("synthetic monitoring payload")
	for _, src := range []string{"192.0.2.2", "192.0.2.3"} {
		relay.process(echoTestPacket(src, "198.51.100.7", 123, 17, payload))
	}
	first, second := <-socket.writes, <-socket.writes
	if bytes.Equal(first[4:8], second[4:8]) {
		t.Fatal("colliding clients share wire identity")
	}
	forged := echoResponse(first)
	forged[len(forged)-1] ^= 1
	forged[2] = 0
	forged[3] = 0
	binary.BigEndian.PutUint16(forged[2:4], echoChecksum(forged))
	relay.receive(netip.MustParseAddr("198.51.100.7"), forged)
	relay.receive(netip.MustParseAddr("198.51.100.8"), echoResponse(first))
	if len(replies) != 0 || down != 0 {
		t.Fatal("forged reply delivered or charged")
	}
	for _, wire := range [][]byte{second, first} {
		relay.receive(netip.MustParseAddr("198.51.100.7"), echoResponse(wire))
	}
	for _, want := range []string{"192.0.2.3", "192.0.2.2"} {
		p := <-replies
		if netip.AddrFrom4([4]byte(p[16:20])).String() != want || p[20] != 0 || binary.BigEndian.Uint16(p[24:26]) != 123 || binary.BigEndian.Uint16(p[26:28]) != 17 || !bytes.Equal(p[28:], payload) || echoChecksum(p[:20]) != 0 || echoChecksum(p[20:]) != 0 {
			t.Fatalf("incorrect reply: %x", p)
		}
	}
	relay.receive(netip.MustParseAddr("198.51.100.7"), echoResponse(first))
	if len(replies) != 0 {
		t.Fatal("duplicate reply delivered")
	}
	want := int64(2 * (28 + len(payload)))
	if up != want || down != want {
		t.Fatalf("traffic = %d/%d want %d/%d", up, down, want, want)
	}
}

func TestEchoDeniedRevokedExpiredAndFailedAccountingNeverSends(t *testing.T) {
	allow := false
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, _, _ int64, _ string) (string, error) {
		if !allow {
			return "", errors.New("denied or accounting failed")
		}
		return "v1", nil
	}
	relay, socket, replies := testEchoRelay(t, gate)
	packet := echoTestPacket("192.0.2.2", "198.51.100.7", 1, 2, nil)
	relay.process(packet)
	relay.process(echoTestPacket("192.0.2.99", "198.51.100.7", 1, 2, nil))
	relay.process(echoTestPacket("192.0.2.2", "192.0.2.1", 1, 2, nil))
	if len(socket.writes) != 0 {
		t.Fatal("denied packet sent")
	}
	allow = true
	relay.process(packet)
	wire := <-socket.writes
	allow = false
	relay.receive(netip.MustParseAddr("198.51.100.7"), echoResponse(wire))
	if len(replies) != 0 {
		t.Fatal("revoked response delivered")
	}
	allow = true
	relay.process(packet)
	wire = <-socket.writes
	relay.mu.Lock()
	for k, p := range relay.pending {
		p.expires = time.Now().Add(-time.Second)
		relay.pending[k] = p
	}
	relay.mu.Unlock()
	relay.receive(netip.MustParseAddr("198.51.100.7"), echoResponse(wire))
	if len(replies) != 0 {
		t.Fatal("late response delivered")
	}
}

func TestEchoCloseDoesNotDeadlockBehindAccounting(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, _, _ int64, _ string) (string, error) {
		close(entered)
		<-release
		return "v1", nil
	}
	relay, socket, _ := testEchoRelay(t, gate)
	relay.handle(echoTestPacket("192.0.2.2", "198.51.100.7", 1, 2, nil))
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("gate not entered")
	}
	finished := make(chan struct{})
	go func() { relay.close(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("teardown blocked on accounting")
	}
	close(release)
	<-relay.workerDone
	if len(socket.writes) != 0 {
		t.Fatal("packet sent after teardown")
	}
}

func TestEchoPendingLimits(t *testing.T) {
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, _, _ int64, _ string) (string, error) { return "v1", nil }
	relay, socket, replies := testEchoRelay(t, gate)
	for i := 0; i < echoMaxPeerPending+64; i++ {
		relay.process(echoTestPacket("192.0.2.2", "198.51.100.7", 1, uint16(i), nil))
	}
	if len(socket.writes) > echoMaxPeerPending || len(replies) != 0 {
		t.Fatalf("unbounded/fabricated packets: %d/%d", len(socket.writes), len(replies))
	}
}

func FuzzEchoPacket(f *testing.F) {
	f.Add(echoTestPacket("192.0.2.2", "198.51.100.7", 1, 2, []byte("probe")))
	f.Fuzz(func(t *testing.T, p []byte) {
		req, err := parseEcho(p)
		if err != nil {
			return
		}
		reply := echoReplyPacket(req)
		if len(reply) != len(p) || echoChecksum(reply[:20]) != 0 || echoChecksum(reply[20:]) != 0 {
			t.Fatal("invalid synthesized reply")
		}
	})
}

func TestEchoUnavailableRawSocketIsClosed(t *testing.T) {
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, _, _ int64, _ string) (string, error) { return "v1", nil }
	relay, socket, replies := testEchoRelay(t, gate)
	relay.open = func() (echoSocket, error) { return nil, errors.New("synthetic missing raw permission") }
	relay.process(echoTestPacket("192.0.2.2", "198.51.100.7", 1, 2, nil))
	if len(socket.writes) != 0 || len(replies) != 0 || len(relay.pending) != 0 {
		t.Fatal("unavailable raw socket leaked a packet or pending probe")
	}
}

func TestEchoMonitoringBatchAndGlobalPerAccountPendingLimit(t *testing.T) {
	gate := func(_ int, _ amneziawg.Peer, _, _ netip.Addr, _, _ int64, _ string) (string, error) { return "v1", nil }
	first, firstSocket, firstReplies := testEchoRelay(t, gate)
	second, secondSocket, _ := testEchoRelay(t, gate)
	const count = 200
	for i := 0; i < count; i++ {
		first.process(echoTestPacket("192.0.2.2", "198.51.100.7", 1, uint16(i), []byte("batch")))
	}
	if len(firstSocket.writes) != count {
		t.Fatalf("monitoring batch dropped %d of %d", count-len(firstSocket.writes), count)
	}
	for i := 0; i < count; i++ {
		second.process(echoTestPacket("192.0.2.2", "198.51.100.7", 1, uint16(i), []byte("batch")))
	}
	if len(firstSocket.writes)+len(secondSocket.writes) > echoMaxPeerPending {
		t.Fatal("account pending budget bypassed across inbounds")
	}
	for i := 0; i < count; i++ {
		first.receive(netip.MustParseAddr("198.51.100.7"), echoResponse(<-firstSocket.writes))
	}
	if len(firstReplies) != count {
		t.Fatal("monitoring batch replies missing")
	}
}
