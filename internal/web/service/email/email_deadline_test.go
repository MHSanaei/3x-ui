package email

import (
	"bufio"
	"bytes"
	"errors"
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

// The flood is capped well above smtpMaxServerBytes: without the cap the client
// buffers it all and then fails on the closed socket, not errSMTPReplyTooLarge.
func TestSendPlainStopsReadingOversizedReply(t *testing.T) {
	tests := []struct {
		name     string
		greeting string
		flood    []byte
	}{
		{"greeting line that never ends", "220 ", bytes.Repeat([]byte("a"), 64<<10)},
		{"EHLO reply that never ends", "220 ready\r\n", bytes.Repeat([]byte("250-X\r\n"), 8<<10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer ln.Close()

			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_, _ = conn.Write([]byte(tt.greeting))
				if tt.greeting[len(tt.greeting)-1] == '\n' {
					_, _ = bufio.NewReader(conn).ReadString('\n')
				}
				for sent := 0; sent < 16<<20; sent += len(tt.flood) {
					if _, err := conn.Write(tt.flood); err != nil {
						return
					}
				}
			}()

			s := &EmailService{}
			err = s.sendPlain(ln.Addr().String(), nil, "from@example.com",
				[]string{"to@example.com"}, []byte("body"), "example.com")
			if !errors.Is(err, errSMTPReplyTooLarge) {
				t.Fatalf("sendPlain error = %v, want %v", err, errSMTPReplyTooLarge)
			}
		})
	}
}
