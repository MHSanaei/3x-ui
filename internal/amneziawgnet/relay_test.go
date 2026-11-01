package amneziawgnet

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"runtime"
	"testing"
	"time"
)

// newDeadUDPSession builds a socks5UDPSession over real but already-closed
// sockets, so pump's receive fails immediately and its teardown runs at once.
func newDeadUDPSession(t *testing.T) *socks5UDPSession {
	t.Helper()
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	udpConn, err := net.DialUDP("udp", nil, peer.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	ctrlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	t.Cleanup(func() { _ = ctrlLn.Close() })
	ctrl, err := net.Dial("tcp", ctrlLn.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	_ = udpConn.Close()
	return &socks5UDPSession{ctrl: ctrl, udpConn: udpConn}
}

// TestUDPRelayPumpOnlyRetiresItsOwnSession pins the flow that survives a
// duplicate associate: a losing pump must not evict the published session.
func TestUDPRelayPumpOnlyRetiresItsOwnSession(t *testing.T) {
	relay := NewUDPRelay(SocksRelay{Addr: "127.0.0.1:1", Password: "x"}, nil)
	src := netip.MustParseAddrPort("10.8.1.5:51820")

	live := newDeadUDPSession(t)
	superseded := newDeadUDPSession(t)
	relay.sessions[src] = live

	// Returns as soon as receive fails on the closed socket, so no wait is needed.
	relay.pump(src, superseded)

	got, ok := relay.sessions[src]
	if !ok {
		t.Fatal("live session was evicted: a retiring pump deleted src's entry regardless of which session held it")
	}
	if got != live {
		t.Fatalf("sessions[%v] = %p, want the live session %p", src, got, live)
	}
}

// TestUDPRelayCloseDropsEverySession keeps Close's contract explicit now that
// pump's teardown is conditional on still owning the key.
func TestUDPRelayCloseDropsEverySession(t *testing.T) {
	relay := NewUDPRelay(SocksRelay{Addr: "127.0.0.1:1", Password: "x"}, nil)
	for _, s := range []string{"10.8.1.5:51820", "10.8.1.6:2000"} {
		relay.sessions[netip.MustParseAddrPort(s)] = newDeadUDPSession(t)
	}

	relay.Close()

	if n := len(relay.sessions); n != 0 {
		t.Fatalf("Close left %d sessions behind, want 0", n)
	}
}

// tcpPair returns a connected pair of real loopback TCP conns; net.Pipe would
// not do, since these tests turn on CloseWrite, which it does not implement.
func tcpPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	type accepted struct {
		conn net.Conn
		err  error
	}
	ch := make(chan accepted, 1)
	go func() {
		c, err := ln.Accept()
		ch <- accepted{c, err}
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	got := <-ch
	if got.err != nil {
		t.Fatalf("accept: %v", got.err)
	}
	t.Cleanup(func() { _ = client.Close(); _ = got.conn.Close() })
	return client, got.conn
}

// TestPipeBothWaysDeliversReplyAfterHalfClose is the half-close regression: a
// client that shuts down its write side must still receive the full response.
func TestPipeBothWaysDeliversReplyAfterHalfClose(t *testing.T) {
	const request = "GET / HTTP/1.0\r\n\r\n"
	const response = "the reply that arrives only after the request is complete"

	client, a := tcpPair(t)
	b, server := tcpPair(t)
	go pipeBothWays(a, b)

	if _, err := client.Write([]byte(request)); err != nil {
		t.Fatalf("client write: %v", err)
	}
	// The half-close the old relay treated as "tear the whole pair down".
	if err := client.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatalf("client CloseWrite: %v", err)
	}

	_ = server.SetReadDeadline(time.Now().Add(10 * time.Second))
	gotReq, err := io.ReadAll(server)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if string(gotReq) != request {
		t.Fatalf("server got request %q, want %q", gotReq, request)
	}

	if _, err := server.Write([]byte(response)); err != nil {
		t.Fatalf("server write: %v", err)
	}
	if err := server.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatalf("server CloseWrite: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(10 * time.Second))
	gotResp, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(gotResp) != response {
		t.Fatalf("client got response %q, want %q: the reply was cut off by the half-close", gotResp, response)
	}
}

// TestPipeBothWaysClosesWhenBothSidesFinish keeps the teardown contract: both
// directions ending must return, not hang on the idle bound.
func TestPipeBothWaysClosesWhenBothSidesFinish(t *testing.T) {
	client, a := tcpPair(t)
	b, server := tcpPair(t)

	done := make(chan struct{})
	go func() { defer close(done); pipeBothWays(a, b) }()

	_ = client.(*net.TCPConn).CloseWrite()
	_, _ = io.ReadAll(server)
	_ = server.(*net.TCPConn).CloseWrite()
	_, _ = io.ReadAll(client)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("pipeBothWays did not return after both directions ended")
	}
}

// liveUDPSession returns a session whose udpConn is connected to the returned
// peer, so a test can hand receive() one exact reply datagram.
func liveUDPSession(t *testing.T) (*socks5UDPSession, *net.UDPConn) {
	t.Helper()
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	udpConn, err := net.DialUDP("udp", nil, peer.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	t.Cleanup(func() { _ = udpConn.Close() })
	return &socks5UDPSession{udpConn: udpConn}, peer
}

// sendReply delivers one raw datagram to sess's socket.
func sendReply(t *testing.T, sess *socks5UDPSession, peer *net.UDPConn, datagram []byte) {
	t.Helper()
	if _, err := peer.WriteToUDP(datagram, sess.udpConn.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	_ = sess.udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
}

// TestSocks5ReceiveDecodesReplyAddressTypes covers all three ATYP forms; the
// domain form used to misread its own length byte and never skip the name.
func TestSocks5ReceiveDecodesReplyAddressTypes(t *testing.T) {
	tests := []struct {
		name     string
		addrPart []byte
		wantAddr string
	}{
		{"IPv4", []byte{0x01, 10, 0, 0, 7}, "10.0.0.7"},
		{"IPv6", append([]byte{0x04}, netip.MustParseAddr("2001:db8::5").AsSlice()...), "2001:db8::5"},
		{"domain holding a literal", append([]byte{0x03, 8}, []byte("10.0.0.9")...), "10.0.0.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, peer := liveUDPSession(t)
			payload := []byte("the-actual-datagram-payload")
			datagram := append([]byte{0x00, 0x00, 0x00}, tt.addrPart...)
			datagram = append(datagram, 0x1f, 0x90) // port 8080
			datagram = append(datagram, payload...)
			sendReply(t, sess, peer, datagram)

			buf := make([]byte, 4096)
			from, got, err := sess.receive(buf)
			if err != nil {
				t.Fatalf("receive: %v", err)
			}
			want := netip.AddrPortFrom(netip.MustParseAddr(tt.wantAddr), 8080)
			if from != want {
				t.Errorf("source = %v, want %v", from, want)
			}
			if string(got) != string(payload) {
				t.Errorf("payload = %q, want %q", got, payload)
			}
		})
	}
}

// TestSocks5ReceiveRejectsTruncatedReplies pins that a short datagram is an
// error, not a slice-bounds panic in the relay's own pump goroutine.
func TestSocks5ReceiveRejectsTruncatedReplies(t *testing.T) {
	tests := []struct {
		name     string
		datagram []byte
	}{
		{"header only, IPv4 announced", []byte{0x00, 0x00, 0x00, 0x01}},
		{"IPv4 address cut short", []byte{0x00, 0x00, 0x00, 0x01, 10, 0}},
		{"IPv6 address cut short", []byte{0x00, 0x00, 0x00, 0x04, 0x20, 0x01}},
		{"domain length past the end", []byte{0x00, 0x00, 0x00, 0x03, 40, 'a', 'b'}},
		{"address complete but port missing", []byte{0x00, 0x00, 0x00, 0x01, 10, 0, 0, 7}},
		{"unsupported address type", []byte{0x00, 0x00, 0x00, 0x09, 1, 2, 3, 4, 0, 80}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, peer := liveUDPSession(t)
			sendReply(t, sess, peer, tt.datagram)
			buf := make([]byte, 4096)
			if _, _, err := sess.receive(buf); err == nil {
				t.Fatal("receive accepted a malformed datagram instead of returning an error")
			}
		})
	}
}

// TestNewSocks5UDPSessionGivesUpOnSilentServer pins that a control connection
// the kernel accepts but nobody answers returns within the associate deadline
// instead of parking Handle -- and with it the tunnel's delivery path -- forever.
func TestNewSocks5UDPSessionGivesUpOnSilentServer(t *testing.T) {
	// Never accepted: the backlog completes the TCP handshake, the greeting
	// lands in the socket buffer, and no reply ever comes -- a hung Xray.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	saved := socks5AssociateTimeout
	socks5AssociateTimeout = 150 * time.Millisecond
	t.Cleanup(func() { socks5AssociateTimeout = saved })

	type result struct {
		sess *socks5UDPSession
		err  error
	}
	done := make(chan result, 1)
	go func() {
		sess, err := newSocks5UDPSession(ln.Addr().String(), "peer@example", "x")
		done <- result{sess, err}
	}()

	select {
	case got := <-done:
		if got.err == nil {
			got.sess.Close()
			t.Fatal("associate succeeded against a server that never answered")
		}
		if !errors.Is(got.err, os.ErrDeadlineExceeded) {
			t.Fatalf("associate error = %v, want one wrapping os.ErrDeadlineExceeded", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("newSocks5UDPSession still blocked 2s past the associate deadline: a silent SOCKS5 server parks the tunnel's delivery path")
	}
}

// A burst of short connections through AmneziaWG used to allocate a fresh 32 KiB copy buffer
// per direction per connection (gigabytes per hour under churn); the pair must be reused.
func TestPipeBothWaysReusesCopyBuffersAcrossConnections(t *testing.T) {
	if raceEnabled {
		t.Skip("under -race sync.Pool drops a share of Puts at random")
	}
	payload := make([]byte, 16*1024)
	sink := make([]byte, 16*1024)
	ping := make([]byte, len("ping")) // the server goroutine's own buffer
	relay := func() {
		tunnelClient, tunnelSide := tcpPair(t)
		upstreamSide, upstreamServer := tcpPair(t)
		done := make(chan struct{})
		go func() { pipeBothWays(tunnelSide, upstreamSide); close(done) }()
		go func() {
			_, _ = io.ReadFull(upstreamServer, ping)
			_, _ = upstreamServer.Write(payload)
			_ = upstreamServer.Close()
		}()
		_, _ = tunnelClient.Write([]byte("ping"))
		_ = tunnelClient.(*net.TCPConn).CloseWrite()
		if _, err := io.ReadFull(tunnelClient, sink); err != nil {
			t.Fatalf("reply: %v", err)
		}
		<-done
	}
	for range 5 {
		relay()
	}
	const conns = 100
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range conns {
		relay()
	}
	runtime.ReadMemStats(&after)
	perConn := (after.TotalAlloc - before.TotalAlloc) / conns
	if perConn >= 32*1024 {
		t.Fatalf("each relayed connection allocated %d bytes, want the copy buffers reused (< 32 KiB)", perConn)
	}
}

// TestDialUpstreamGivesUpOnSilentBridge pins that a SOCKS5 bridge the kernel accepts but
// nobody answers -- a hung Xray -- fails the CONNECT within its bound instead of pinning
// the relay and its endpoint forever.
func TestDialUpstreamGivesUpOnSilentBridge(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0") // never accepted: no greeting reply ever comes
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	saved := socks5ConnectTimeout
	socks5ConnectTimeout = 150 * time.Millisecond
	t.Cleanup(func() { socks5ConnectTimeout = saved })

	relay := SocksRelay{Addr: ln.Addr().String(), Password: "x"}
	done := make(chan error, 1)
	go func() {
		conn, err := relay.dialUpstream("peer@example", netip.MustParseAddrPort("192.0.2.1:443"))
		if conn != nil {
			_ = conn.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("CONNECT succeeded through a bridge that never answered")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dialUpstream still blocked 2s past the connect bound: a silent bridge pins the relay")
	}
}

// TestDialUpstreamLeavesNoDeadlineAfterHandshake pins that the connect bound covers the
// SOCKS5 handshake only: a destination that answers later than the bound still reaches
// the client through the returned connection.
func TestDialUpstreamLeavesNoDeadlineAfterHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	saved := socks5ConnectTimeout
	socks5ConnectTimeout = 150 * time.Millisecond
	t.Cleanup(func() { socks5ConnectTimeout = saved })

	served := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			served <- err
			return
		}
		defer c.Close()
		served <- serveSocks5Connect(c, 3*socks5ConnectTimeout, []byte("late"))
	}()

	relay := SocksRelay{Addr: ln.Addr().String(), Password: "x"}
	conn, err := relay.dialUpstream("peer@example", netip.MustParseAddrPort("192.0.2.1:443"))
	if err != nil {
		t.Fatalf("dialUpstream: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // test guard only
	got := make([]byte, 4)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read after the connect bound passed: %v (the handshake deadline leaked onto the relay)", err)
	}
	if string(got) != "late" {
		t.Fatalf("read %q, want %q", got, "late")
	}
	if err := <-served; err != nil {
		t.Fatalf("fake bridge: %v", err)
	}
}

// serveSocks5Connect answers one username/password SOCKS5 CONNECT, waits, then writes payload.
func serveSocks5Connect(c net.Conn, wait time.Duration, payload []byte) error {
	buf := make([]byte, 512)
	read := func(n int) ([]byte, error) { _, err := io.ReadFull(c, buf[:n]); return buf[:n], err }
	hdr, err := read(2) // VER NMETHODS
	if err != nil {
		return err
	}
	if _, err := read(int(hdr[1])); err != nil {
		return err
	}
	if _, err := c.Write([]byte{5, 2}); err != nil { // username/password
		return err
	}
	ver, err := read(2) // VER ULEN
	if err != nil {
		return err
	}
	if ver[0] != 1 {
		return errors.New("bad auth version")
	}
	if _, err := read(int(ver[1])); err != nil {
		return err
	}
	plen, err := read(1)
	if err != nil {
		return err
	}
	if _, err := read(int(plen[0])); err != nil {
		return err
	}
	if _, err := c.Write([]byte{1, 0}); err != nil {
		return err
	}
	req, err := read(4) // VER CMD RSV ATYP
	if err != nil {
		return err
	}
	addrLen := map[byte]int{1: 4, 4: 16}[req[3]]
	if req[3] == 3 {
		l, err := read(1)
		if err != nil {
			return err
		}
		addrLen = int(l[0])
	}
	if _, err := read(addrLen + 2); err != nil {
		return err
	}
	if _, err := c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return err
	}
	time.Sleep(wait)
	_, err = c.Write(payload)
	return err
}
