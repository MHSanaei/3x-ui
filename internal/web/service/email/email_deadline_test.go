package email

import (
	"net"
	"testing"
	"time"
)

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
