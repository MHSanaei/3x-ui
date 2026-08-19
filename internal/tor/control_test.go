package tor

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeControlServer accepts exactly one connection and answers "250 OK" to
// every line it reads, closing after the caller-specified number of
// commands -- enough to prove NewIdentity's own framing (AUTHENTICATE with
// the real cookie contents, then SIGNAL NEWNYM, each CRLF-terminated) and
// response parsing are correct without needing a real tor binary.
func fakeControlServer(t *testing.T, wantCommands int) (addr string, gotLines chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	gotLines = make(chan string, wantCommands)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		for i := 0; i < wantCommands; i++ {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			gotLines <- strings.TrimRight(line, "\r\n")
			_, _ = conn.Write([]byte("250 OK\r\n"))
		}
	}()
	return ln.Addr().String(), gotLines
}

func TestNewIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	if err := os.MkdirAll(dataDir(), 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(controlCookiePath(), []byte{0xDE, 0xAD, 0xBE, 0xEF}, 0o600); err != nil {
		t.Fatalf("write fake cookie: %v", err)
	}

	addr, gotLines := fakeControlServer(t, 2)
	origAddr := controlAddr
	controlAddr = addr
	t.Cleanup(func() { controlAddr = origAddr })

	if err := NewIdentity(); err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}

	authLine := <-gotLines
	if !strings.HasPrefix(authLine, "AUTHENTICATE deadbeef") {
		t.Fatalf("first command = %q, want AUTHENTICATE deadbeef...", authLine)
	}
	signalLine := <-gotLines
	if signalLine != "SIGNAL NEWNYM" {
		t.Fatalf("second command = %q, want %q", signalLine, "SIGNAL NEWNYM")
	}
}

func TestNewIdentityMissingCookie(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	// No cookie file written -- controlAuthenticate must fail cleanly, not panic.

	addr, _ := fakeControlServer(t, 1)
	origAddr := controlAddr
	controlAddr = addr
	t.Cleanup(func() { controlAddr = origAddr })

	if err := NewIdentity(); err == nil {
		t.Fatal("NewIdentity with no cookie file: want error, got nil")
	}
}

func TestNewIdentityNoControlPort(t *testing.T) {
	origAddr := controlAddr
	controlAddr = "127.0.0.1:1" // nothing listens on the reserved TCP port 1
	t.Cleanup(func() { controlAddr = origAddr })

	if err := NewIdentity(); err == nil {
		t.Fatal("NewIdentity with nothing listening: want error, got nil")
	}
}

// TestRealTorControlAndExitIP starts a real tor daemon (only where one is on
// PATH -- see TestManagerRealProcessLifecycle for why this is a genuine
// regression test rather than a permanent skip) and exercises the full
// stack: bootstrap, exit-IP check through the real SOCKS port, and a real
// NEWNYM signal over the real control port with the real cookie file tor
// itself wrote.
func TestRealTorControlAndExitIP(t *testing.T) {
	if !IsAvailable() {
		t.Skip("tor binary not found on PATH")
	}
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	controlAddr = fmt.Sprintf("127.0.0.1:%d", ControlPort)
	m := &Manager{}
	t.Cleanup(func() { _ = m.Stop() })
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(45 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", SocksPort), time.Second)
		if err == nil {
			_ = conn.Close()
			lastErr = nil
			break
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("SOCKS port never came up: %v (last log: %s)", lastErr, m.LastResult())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ip, isTor, err := CurrentIP(ctx)
	if err != nil {
		t.Fatalf("CurrentIP: %v (last log: %s)", err, m.LastResult())
	}
	if ip == "" {
		t.Fatal("CurrentIP returned an empty IP with no error")
	}
	if !isTor {
		t.Errorf("check.torproject.org says IsTor=false for exit IP %s -- traffic may not really be going through Tor", ip)
	}
	t.Logf("exit IP: %s (isTor=%v)", ip, isTor)

	if err := NewIdentity(); err != nil {
		t.Fatalf("NewIdentity against the real control port: %v", err)
	}
}
