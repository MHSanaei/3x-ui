package amneziawgnet

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"runtime"
	"testing"
	"time"
)

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

// TestDialUpstreamGivesUpOnSilentBridge: a bridge that accepts but never answers (a hung
// Xray) must fail the CONNECT within its bound instead of pinning the relay.
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

// TestDialUpstreamLeavesNoDeadlineAfterHandshake: the connect bound covers the handshake
// only, so writes and reads after it has passed still go through.
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
		served <- serveSocks5Connect(c, []byte("ping"), []byte("late"))
	}()

	relay := SocksRelay{Addr: ln.Addr().String(), Password: "x"}
	conn, err := relay.dialUpstream("peer@example", netip.MustParseAddrPort("192.0.2.1:443"))
	if err != nil {
		t.Fatalf("dialUpstream: %v", err)
	}
	defer conn.Close()
	guard := time.AfterFunc(5*time.Second, func() { _ = conn.Close() })
	defer guard.Stop()
	time.Sleep(3 * socks5ConnectTimeout)
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write after the connect bound passed: %v (the handshake deadline leaked onto the relay)", err)
	}
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

// serveSocks5Connect answers one username/password SOCKS5 CONNECT, reads request, then writes reply.
func serveSocks5Connect(c net.Conn, request, reply []byte) error {
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
	got, err := read(len(request))
	if err != nil {
		return err
	}
	if string(got) != string(request) {
		return errors.New("bridge read an unexpected request")
	}
	_, err = c.Write(reply)
	return err
}
