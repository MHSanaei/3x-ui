package email

import (
	"net"
	"testing"
	"time"
)

// A server that accepts the TCP connection but then never speaks must not block
// the sender goroutine indefinitely: sendPlain arms a connection deadline so the
// SMTP greeting read fails instead of hanging until the OS TCP timeout, long
// after the caller's own 30s budget has passed.
func TestSendPlainReturnsOnStalledServer(t *testing.T) {
	orig := smtpDeadline
	smtpDeadline = 300 * time.Millisecond
	t.Cleanup(func() { smtpDeadline = orig })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	stall := make(chan struct{})
	defer close(stall)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-stall
	}()

	s := &EmailService{}
	done := make(chan error, 1)
	go func() {
		done <- s.sendPlain(ln.Addr().String(), nil, "from@example.com",
			[]string{"to@example.com"}, []byte("body"), "example.com")
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a silent SMTP server, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("sendPlain did not return on a stalled server within the deadline")
	}
}
