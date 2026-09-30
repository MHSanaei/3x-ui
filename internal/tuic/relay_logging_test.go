package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	clientquic "github.com/quic-go/quic-go"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func audit3LogsStart(t *testing.T, level, marker, relayAddr string) (*Server, *clientquic.Conn, uuid.UUID, string, []byte) {
	t.Helper()
	cert, key := generateTestCert(t)
	id := uuid.New()
	password := "PASSWORD-CANARY-" + marker
	s, err := NewServer(Instance{
		Id: 192301, Tag: marker, Listen: "127.0.0.1", Port: 0,
		Certificate: string(cert), PrivateKey: string(key), ALPN: []string{"h3"},
		AuthenticationTimeout: 2, MaxIdleTime: 30, LogLevel: level,
		Clients: []TuicClientSettings{{UUID: id.String(), Password: password, Email: "audit3-log@example.test"}},
	}, &SocksRelay{Addr: relayAddr, Password: "audit3-socks-pass"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := clientquic.DialAddr(ctx, s.packetConn.LocalAddr().String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}, &clientquic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseWithError(0, "audit3 finished") })
	tlsState := c.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(id[:]), []byte(password), 32)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := c.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	frame := append([]byte{5, 0}, id[:]...)
	frame = append(frame, token...)
	if _, err := auth.Write(frame); err != nil {
		t.Fatal(err)
	}
	if err := auth.Close(); err != nil {
		t.Fatal(err)
	}
	_, _ = authenticatedServerConnection(t, s, id)
	return s, c, id, password, token
}

func audit3LogsFor(marker string) string {
	var lines []string
	for _, line := range logger.GetLogs(10000, "DEBUG") {
		if strings.Contains(line, marker) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func TestAudit3RealEventsRespectThresholdAndDoNotExposeSecrets(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			marker := fmt.Sprintf("audit3-logs-%s-%d", level, time.Now().UnixNano())
			relayAddr, cleanup := startMockSocks5Server(t, "audit3-log@example.test", "audit3-socks-pass")
			defer cleanup()
			s, c, id, password, token := audit3LogsStart(t, level, marker, relayAddr)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			payload := "PAYLOAD-CANARY-" + marker
			u, err := c.OpenStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var frame bytes.Buffer
			frame.Write([]byte{5, 1})
			injectedDomain := "audit.invalid-FORGED-ENTRY-" + marker
			if err := WriteAddress(&frame, &Address{Type: AddrTypeDomain, Host: injectedDomain, Port: 443}); err != nil {
				t.Fatal(err)
			}
			frame.WriteString(payload)
			if _, err := u.Write(frame.Bytes()); err != nil {
				t.Fatal(err)
			}
			if err := u.Close(); err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(payload))
			if _, err := io.ReadFull(u, got); err != nil || string(got) != payload {
				t.Fatalf("TCP echo %q, %v", got, err)
			}
			if _, err := io.Copy(io.Discard, u); err != nil {
				t.Fatal(err)
			}
			u.CancelRead(0)
			var udp bytes.Buffer
			if err := WritePacket(&udp, 23456, 1, 1, 0, &Address{Type: AddrTypeIPv4, IP: net.IPv4(8, 8, 8, 8), Port: 53}, []byte(payload)); err != nil {
				t.Fatal(err)
			}
			if err := c.SendDatagram(udp.Bytes()); err != nil {
				t.Fatal(err)
			}
			if _, err := c.ReceiveDatagram(ctx); err != nil {
				t.Fatal(err)
			}
			dissociate, err := c.OpenUniStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := dissociate.Write([]byte{5, 3, 0x5b, 0xa0}); err != nil {
				t.Fatal(err)
			}
			_ = dissociate.Close()
			// The malformed frame includes traffic content as a canary; it must remain absent from logs.
			if err := c.SendDatagram(append([]byte{5, 2}, []byte(payload)...)); err != nil {
				t.Fatal(err)
			}
			bad, err := c.OpenStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bad.Write([]byte{5, 1, 0xff}); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(io.Discard, bad); err != nil {
				t.Fatal(err)
			}
			_ = bad.Close()
			// Trigger a rejected Authenticate event using a changed token on the authenticated connection.
			badAuth, err := c.OpenUniStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			wrongToken := bytes.Repeat([]byte{0x6d}, 32)
			badFrame := append([]byte{5, 0}, id[:]...)
			badFrame = append(badFrame, wrongToken...)
			if _, err := badAuth.Write(badFrame); err != nil {
				t.Fatal(err)
			}
			_ = badAuth.Close()
			select {
			case <-c.Context().Done():
			case <-ctx.Done():
				t.Fatal("bad auth did not close connection")
			}
			_ = s.packetConn.Close()
			deadline := time.Now().Add(2 * time.Second)
			for s.IsRunning() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			_ = s.Close()
			logs := audit3LogsFor(marker)
			for _, secret := range []string{password, id.String(), hex.EncodeToString(id[:]), hex.EncodeToString(token), hex.EncodeToString(wrongToken), payload, injectedDomain} {
				if strings.Contains(logs, secret) {
					t.Fatalf("logs expose canary %q", secret)
				}
			}
			wantInfo := level == "debug" || level == "info"
			wantWarn := level != "error"
			for _, event := range []string{"listener started", "client authenticated", "TCP relay started", "UDP association 23456 started", "listener stopped"} {
				if got := strings.Contains(logs, "): "+event); got != wantInfo {
					t.Errorf("event %q present=%t, want %t\n%s", event, got, wantInfo, logs)
				}
			}
			for _, event := range []string{"TCP relay failed", "client authentication rejected"} {
				if got := strings.Contains(logs, event); got != wantWarn {
					t.Errorf("event %q present=%t, want %t\n%s", event, got, wantWarn, logs)
				}
			}
			if got := strings.Contains(logs, "configured bbr congestion controller"); got != (level == "debug") {
				t.Errorf("debug controller event=%t", got)
			}
			if !strings.Contains(logs, "QUIC listener stopped accepting connections") {
				t.Errorf("actual listener error event missing at %s\n%s", level, logs)
			}
			t.Logf("actual logger events at %s: %d", level, strings.Count(logs, "tuic: inbound"))
		})
	}
}

func TestAudit3TCPFailuresMustNotFloodPanelLogs(t *testing.T) {
	marker := fmt.Sprintf("audit3-flood-%d", time.Now().UnixNano())
	relayAddr, cleanup := startMockSocks5Server(t, "audit3-log@example.test", "audit3-socks-pass")
	defer cleanup()
	_, c, _, _, _ := audit3LogsStart(t, "warn", marker, relayAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 110; i++ {
		u, err := c.OpenStreamSync(ctx)
		if err != nil {
			t.Fatalf("CONNECT%d: %v", i, err)
		}
		if _, err := u.Write([]byte{5, 1, 0xff}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, u); err != nil {
			t.Fatal(err)
		}
		_ = u.Close()
	}
	count := strings.Count(audit3LogsFor(marker), "TCP relay failed")
	if count != 1 {
		t.Fatalf("one authenticated QUIC connection emitted %d TCP failure warnings for 110 commands; expected a bounded warning category", count)
	}
}

func TestAudit3BiStreamCreditKeepsTCPEchoUsable(t *testing.T) {
	marker := fmt.Sprintf("audit3-credit-%d", time.Now().UnixNano())
	relayAddr, cleanup := startMockSocks5Server(t, "audit3-log@example.test", "audit3-socks-pass")
	defer cleanup()
	_, c, _, _, _ := audit3LogsStart(t, "error", marker, relayAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 110; i++ {
		openCtx, openCancel := context.WithTimeout(ctx, 700*time.Millisecond)
		u, err := c.OpenStreamSync(openCtx)
		openCancel()
		if err != nil {
			t.Fatalf("malformed stream%d open: %v", i, err)
		}
		if _, err := u.Write([]byte{5, 0xff}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, u); err != nil {
			t.Fatal(err)
		}
		_ = u.Close()
	}
	u, err := c.OpenStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var frame bytes.Buffer
	frame.Write([]byte{5, 1})
	if err := WriteAddress(&frame, &Address{Type: AddrTypeIPv4, IP: net.IPv4(8, 8, 8, 8), Port: 443}); err != nil {
		t.Fatal(err)
	}
	frame.WriteString("after-credit-errors")
	if _, err := u.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = u.Close()
	result, err := io.ReadAll(u)
	if err != nil || string(result) != "after-credit-errors" {
		t.Fatalf("subsequent real TCP relay result=%q err=%v", result, err)
	}
}
