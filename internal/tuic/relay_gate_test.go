package tuic

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func testRelayGate() *RelayGate {
	return &RelayGate{writer: make(chan struct{}, 1), changed: make(chan struct{})}
}

func waitRelayWaiters(t *testing.T, gate *RelayGate, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		gate.mu.Lock()
		got := gate.waiting
		gate.mu.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending relay opens = %d, want %d", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRelayWaitQueueRejectsOverflowAndCancelsEveryWaiter(t *testing.T) {
	gate := testRelayGate()
	relay := &SocksRelay{Gate: gate, InboundID: 17, Addr: "127.0.0.1:1"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, maxPendingRelayOpens)
	target := &Address{Type: AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 9}
	for range maxPendingRelayOpens {
		go func() {
			conn, err := relay.DialTCP(ctx, "waiting@test", target)
			if conn != nil {
				_ = conn.Close()
			}
			done <- err
		}()
	}
	waitRelayWaiters(t, gate, maxPendingRelayOpens)
	if _, err := relay.DialTCP(ctx, "overflow@test", target); !errors.Is(err, ErrRelayQueueFull) {
		t.Fatalf("overflow error = %v, want %v", err, ErrRelayQueueFull)
	}
	cancel()
	for range maxPendingRelayOpens {
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled pending open returned %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("canceled relay open remained blocked")
		}
	}
	waitRelayWaiters(t, gate, 0)
}

func TestRelayUpdateDrainsAuthenticationAndCancellationRestoresAdmission(t *testing.T) {
	gate := testRelayGate()
	initial, err := gate.BeginUpdate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	initial.Finish(17, []string{"active@test"}, true)
	release, err := gate.acquire(context.Background(), 17, "active@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan error, 1)
	go func() {
		update, err := gate.BeginUpdate(ctx)
		if update != nil {
			update.Cancel()
		}
		updates <- err
	}()
	select {
	case err := <-updates:
		t.Fatalf("relay replacement overtook active authentication: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-updates; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel update = %v, want context.Canceled", err)
	}
	release()
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err = gate.acquire(ctx, 17, "active@test")
	if err != nil {
		t.Fatalf("canceled update left admission paused: %v", err)
	}
	release()
	if _, err := gate.acquire(ctx, 18, "active@test"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reused relay slot admitted another inbound: %v", err)
	}
}

func TestServerCloseCancelsQueuedUDPAssociation(t *testing.T) {
	gate := testRelayGate()
	server, client, id, _ := startLifecycleTestServerWithRelay(t, &SocksRelay{
		Gate: gate, InboundID: 99101, Addr: "127.0.0.1:1", Password: "lifecycle-socks-password",
	}, "closing@test")
	_, _ = authenticatedServerConnection(t, server, id)
	audit3SendPacket(t, client, packetTransportDatagram, 99, 1, "queued")
	waitRelayWaiters(t, gate, 1)
	closed := make(chan error, 1)
	go func() { closed <- server.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server shutdown waited for relay update timeout")
	}
	waitRelayWaiters(t, gate, 0)
	if bytes := server.pendingUdpBudget.bytes.Load(); bytes != 0 {
		t.Fatalf("shutdown retained %d bytes of queued UDP packets", bytes)
	}
}
