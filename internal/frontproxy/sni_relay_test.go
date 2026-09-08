package frontproxy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// TestNewSNIRelayListenerNoTargetsReturnsInnerUnchanged pins the actual
// regression guarantee: with no targets, the wrapper is skipped entirely,
// not just made to behave the same.
func TestNewSNIRelayListenerNoTargetsReturnsInnerUnchanged(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer inner.Close()

	if got := newSNIRelayListener(inner, nil); got != inner {
		t.Fatalf("newSNIRelayListener(inner, nil) = %v, want the exact same listener back", got)
	}
	if got := newSNIRelayListener(inner, map[string]string{}); got != inner {
		t.Fatalf("newSNIRelayListener(inner, {}) = %v, want the exact same listener back", got)
	}
}

// clientHelloBytes returns one real ClientHello's wire bytes for sni, via a
// genuine tls.Client handshake against a net.Pipe() peer that never
// replies -- tls.Client sends exactly one ClientHello, then blocks.
func clientHelloBytes(t *testing.T, sni string) []byte {
	t.Helper()
	readSide, writeSide := net.Pipe()
	captured := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 8192)
		n, _ := readSide.Read(buf)
		captured <- append([]byte(nil), buf[:n]...)
		readSide.Close()
	}()
	go func() {
		//nolint:gosec // test-only handshake against a pipe that never replies
		_ = tls.Client(writeSide, &tls.Config{ServerName: sni, InsecureSkipVerify: true}).Handshake()
	}()
	select {
	case b := <-captured:
		if len(b) == 0 {
			t.Fatal("captured an empty ClientHello")
		}
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("timed out capturing a ClientHello")
		return nil
	}
}

// dialAndSendClientHello dials addr and writes one real ClientHello for sni,
// returning the raw conn with no concurrent goroutine left touching it.
func dialAndSendClientHello(t *testing.T, addr, sni string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Write(clientHelloBytes(t, sni)); err != nil {
		t.Fatalf("write ClientHello to %s: %v", addr, err)
	}
	return conn
}

// drainAccept discards every connection relayLn.Accept() ever returns, so a
// bug that leaks a supposedly-relayed connection through doesn't hang a test.
func drainAccept(relayLn net.Listener) {
	for {
		c, err := relayLn.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}

func TestPeekClientHelloSNIExtractsSNIWithoutLosingBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()

	hello := clientHelloBytes(t, "naive.example.test")
	client, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write(hello); err != nil {
		t.Fatal(err)
	}

	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept never returned")
	}
	defer server.Close()

	sni, prefix := peekClientHelloSNI(server)
	if sni != "naive.example.test" {
		t.Errorf("peekClientHelloSNI sni = %q, want naive.example.test", sni)
	}
	if !bytes.Equal(prefix, hello) {
		t.Errorf("prefix (%d bytes) does not match the ClientHello actually sent (%d bytes)", len(prefix), len(hello))
	}
}

// TestPeekClientHelloSNIDiscardsTheAbortAlert is the load-bearing test for
// peekConn.Write: without it, the aborted sacrificial handshake leaks a real
// TLS alert onto the client's socket (see peekConn's doc comment).
func TestPeekClientHelloSNIDiscardsTheAbortAlert(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()

	client := dialAndSendClientHello(t, ln.Addr().String(), "example.test")

	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept never returned")
	}
	defer server.Close()

	peekClientHelloSNI(server) // the real function under test

	_ = client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 16)
	n, err := client.Read(buf)
	if n > 0 {
		t.Fatalf("peekClientHelloSNI wrote %d bytes back to the client, got: %x", n, buf[:n])
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected a read timeout (nothing ever sent), got: %v", err)
	}
}

func TestSNIRelaySplicesMatchingConnectionToBackend(t *testing.T) {
	hello := clientHelloBytes(t, "naive.example.test")

	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendLn.Close()

	backendResult := make(chan []byte, 1)
	go func() {
		c, err := backendLn.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, len(hello))
		_, err = io.ReadFull(c, buf)
		if err != nil {
			backendResult <- nil
			return
		}
		backendResult <- buf
		_, _ = c.Write([]byte("R"))
	}()

	frontLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer frontLn.Close()
	relayLn := newSNIRelayListener(frontLn, map[string]string{
		"naive.example.test": backendLn.Addr().String(),
	})
	go drainAccept(relayLn)

	client, err := net.DialTimeout("tcp", frontLn.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write(hello); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-backendResult:
		if !bytes.Equal(got, hello) {
			t.Errorf("backend received %d bytes not matching the ClientHello relayRaw's prefix should have carried (got %d bytes)", len(got), len(hello))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("backend never received a connection -- relayRaw did not dial it")
	}

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	reply := make([]byte, 1)
	if _, err := client.Read(reply); err != nil {
		t.Fatalf("reading the backend's echoed reply through the relay: %v", err)
	}
	if reply[0] != 'R' {
		t.Errorf("relayed reply = %q, want %q", reply, "R")
	}
}

// TestSNIRelayPassesThroughNonMatchingSNIUnchanged proves the no-match path
// via a real, complete two-sided TLS handshake: any byte the peek/replay
// machinery disturbs fails it.
func TestSNIRelayPassesThroughNonMatchingSNIUnchanged(t *testing.T) {
	frontLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer frontLn.Close()
	relayLn := newSNIRelayListener(frontLn, map[string]string{
		"naive.example.test": "127.0.0.1:1", // never dialed by this test
	})

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := relayLn.Accept()
		if err == nil {
			accepted <- c
		}
	}()

	clientConn, err := net.DialTimeout("tcp", frontLn.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	clientErrCh := make(chan error, 1)
	go func() {
		//nolint:gosec // test cert, loopback only
		clientErrCh <- tls.Client(clientConn, &tls.Config{ServerName: "some-other-site.test", InsecureSkipVerify: true}).Handshake()
	}()

	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept never returned a connection for a non-matching SNI")
	}
	defer server.Close()

	certFile, keyFile, _ := writeTestCert(t)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	tlsServer := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{cert}})
	if err := tlsServer.Handshake(); err != nil {
		t.Fatalf("server-side handshake over the passed-through connection failed: %v", err)
	}
	select {
	case err := <-clientErrCh:
		if err != nil {
			t.Fatalf("client-side handshake failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client-side handshake never completed")
	}
}

// TestSNIRelayMatchIsCaseInsensitive covers both sides of the
// case-normalization independently: the configured key (lowercased once, at
// construction) and the wire SNI (lowercased on every lookup).
func TestSNIRelayMatchIsCaseInsensitive(t *testing.T) {
	cases := []struct {
		name      string
		configKey string
		wireSNI   string
	}{
		{"mixed-case configured key", "Naive.Example.Test", "naive.example.test"},
		{"mixed-case wire SNI", "naive.example.test", "Naive.Example.Test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backendLn, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer backendLn.Close()
			go func() {
				c, err := backendLn.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				buf := make([]byte, 8192)
				_, _ = c.Read(buf)
				_, _ = c.Write([]byte("R"))
			}()

			frontLn, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer frontLn.Close()
			relayLn := newSNIRelayListener(frontLn, map[string]string{tc.configKey: backendLn.Addr().String()})
			go drainAccept(relayLn)

			client := dialAndSendClientHello(t, frontLn.Addr().String(), tc.wireSNI)
			_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
			reply := make([]byte, 1)
			if _, err := client.Read(reply); err != nil {
				t.Fatalf("reading the backend's echoed reply: %v", err)
			}
			if reply[0] != 'R' {
				t.Errorf("relayed reply = %q, want %q", reply, "R")
			}
		})
	}
}

// TestCloseWhileHandleIsBlockedOnAcceptDoesNotPanic pins the fix for the
// original bug: closing while a handle goroutine is blocked trying to hand
// a non-matching connection to Accept must not panic on a closed channel
// send. No drainAccept here -- nothing ever reads l.out, so handle() is
// still blocked on its send when Close runs.
func TestCloseWhileHandleIsBlockedOnAcceptDoesNotPanic(t *testing.T) {
	frontLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer frontLn.Close()
	relayLn := newSNIRelayListener(frontLn, map[string]string{
		"never-matches.test": "127.0.0.1:1",
	})

	dialAndSendClientHello(t, frontLn.Addr().String(), "some-other-site.test")
	time.Sleep(100 * time.Millisecond) // let handle() reach its blocked send

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := relayLn.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close never returned")
	}
	// A pre-fix send-on-closed-channel panic would have already crashed this
	// whole test binary by now -- reaching this point at all is the proof.
}

// TestSNIRelayDoesNotTruncateOnClientHalfClose pins relayRaw's wait-for-
// both-directions fix: waiting for only the first direction to finish (the
// client's own half-close) would cut the backend's still-in-flight reply.
func TestSNIRelayDoesNotTruncateOnClientHalfClose(t *testing.T) {
	fullReply := []byte("first-chunk|second-chunk-after-a-delay")
	firstChunk, secondChunk := fullReply[:len("first-chunk|")], fullReply[len("first-chunk|"):]

	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendLn.Close()
	go func() {
		c, err := backendLn.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 8192)
		_, _ = c.Read(buf)
		_, _ = c.Write(firstChunk)
		time.Sleep(200 * time.Millisecond) // gives the client's own CloseWrite below time to land first
		_, _ = c.Write(secondChunk)
	}()

	frontLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer frontLn.Close()
	relayLn := newSNIRelayListener(frontLn, map[string]string{
		"naive.example.test": backendLn.Addr().String(),
	})
	go drainAccept(relayLn)

	client := dialAndSendClientHello(t, frontLn.Addr().String(), "naive.example.test")
	tcpClient, ok := client.(*net.TCPConn)
	if !ok {
		t.Fatal("dialAndSendClientHello did not return a *net.TCPConn")
	}
	if err := tcpClient.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("reading the relayed reply: %v", err)
	}
	if !bytes.Equal(got, fullReply) {
		t.Errorf("got %q, want %q -- the client's early half-close must not truncate the backend's still-in-flight reply", got, fullReply)
	}
}
