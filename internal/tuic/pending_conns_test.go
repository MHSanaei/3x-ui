package tuic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
	"testing"
	"time"

	clientquic "github.com/quic-go/quic-go"
)

// Connections awaiting authentication are counted from their first packet: past the
// threshold they must prove their address, and past the ceilings they are refused.
func TestPendingConnsRetryAndCeilings(t *testing.T) {
	var p pendingConns
	verified := func(ip string) connInfo {
		return connInfo{remote: &net.UDPAddr{IP: net.ParseIP(ip), Port: 1}, addressVerified: true}
	}
	ctx, cancel := context.WithCancel(context.Background())
	for i := range pendingRetryThreshold {
		if p.needsRetry(nil) {
			t.Fatalf("Retry demanded with only %d pending", i)
		}
		if _, err := p.admit(ctx, connInfo{remote: &net.UDPAddr{IP: net.IPv4(203, 0, 113, byte(i)), Port: 1}}); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	if !p.needsRetry(nil) {
		t.Fatal("no Retry demanded at the threshold")
	}
	for range maxPendingPerSource {
		if _, err := p.admit(ctx, verified("2001:db8::1")); err != nil {
			t.Fatalf("admit from one source: %v", err)
		}
	}
	if _, err := p.admit(ctx, verified("2001:db8::2")); !errors.Is(err, errTooManyPendingConns) {
		t.Fatalf("one /64 past its share = %v, want refused", err)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		p.mu.Lock()
		total := p.total
		p.mu.Unlock()
		if total == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d slots still held after their connections ended", total)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A peer that never authenticates must not get a handler goroutine per stream it opens.
func TestUnauthenticatedStreamsShareASmallHandlerBudget(t *testing.T) {
	server, _, _, _ := startLifecycleTestServer(t, "127.0.0.1:1", "preauth@x")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer, err := clientquic.DialAddr(ctx, server.packetConn.LocalAddr().String(),
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}, &clientquic.Config{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = peer.CloseWithError(0, "") }()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()
	for range 90 {
		stream, err := peer.OpenStreamSync(ctx)
		if err != nil {
			break
		}
		_, _ = stream.Write([]byte{ProtocolVersion, CmdConnect})
	}
	time.Sleep(300 * time.Millisecond)
	if grown := runtime.NumGoroutine() - before; grown > 2*maxPreAuthStreams {
		t.Fatalf("90 unauthenticated streams started %d goroutines", grown)
	}
}
