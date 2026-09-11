package tuic

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// doublingEcho answers every datagram with the payload repeated twice, so a
// relay that mislabels directions or clients cannot pass by accident.
func doublingEcho(t *testing.T) *net.UDPAddr {
	t.Helper()
	echo, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = echo.Close() })
	go func() {
		buf := make([]byte, 65535)
		for {
			n, from, err := echo.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = echo.WriteToUDP(append(append([]byte{}, buf[:n]...), buf[:n]...), from)
		}
	}()
	return echo.LocalAddr().(*net.UDPAddr)
}

func roundTrip(t *testing.T, relay *udpRelay, payload []byte) int {
	t.Helper()
	c, err := net.DialUDP("udp", nil, relay.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 65535)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("no reply through the relay: %v", err)
	}
	return n
}

func collectUntil(t *testing.T, relay *udpRelay, wantUp, wantDown int64) (int64, int64) {
	t.Helper()
	var up, down int64
	deadline := time.Now().Add(2 * time.Second)
	for {
		u, d := relay.CollectTraffic()
		up, down = up+u, down+d
		if (up >= wantUp && down >= wantDown) || time.Now().After(deadline) {
			return up, down
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUDPRelayMetersBothDirectionsPerClient(t *testing.T) {
	relay, err := startUDPRelay("127.0.0.1:0", doublingEcho(t), relayFlowIdle)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(relay.Close)

	if got := roundTrip(t, relay, bytes.Repeat([]byte("a"), 100)); got != 200 {
		t.Fatalf("client A reply = %d bytes, want 200", got)
	}
	if got := roundTrip(t, relay, bytes.Repeat([]byte("b"), 50)); got != 100 {
		t.Fatalf("client B reply = %d bytes, want 100", got)
	}
	if up, down := collectUntil(t, relay, 150, 300); up != 150 || down != 300 {
		t.Fatalf("delta = (%d up, %d down), want (150, 300)", up, down)
	}
	if up, down := relay.CollectTraffic(); up != 0 || down != 0 {
		t.Fatalf("second collect = (%d, %d), want (0, 0): deltas must reset", up, down)
	}
}

func TestUDPRelayExpiresIdleFlows(t *testing.T) {
	relay, err := startUDPRelay("127.0.0.1:0", doublingEcho(t), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(relay.Close)

	roundTrip(t, relay, []byte("hello"))
	deadline := time.Now().Add(2 * time.Second)
	for {
		relay.mu.Lock()
		n := len(relay.flows)
		relay.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d flow(s) still open after the idle window", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := roundTrip(t, relay, []byte("again")); got != 10 {
		t.Fatalf("reply after expiry = %d bytes, want 10", got)
	}
}

func TestUDPRelayRefusesFlowsAfterClose(t *testing.T) {
	relay, err := startUDPRelay("127.0.0.1:0", doublingEcho(t), relayFlowIdle)
	if err != nil {
		t.Fatal(err)
	}
	relay.Close()
	if _, err := relay.flowFor(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}); err == nil {
		t.Fatal("flowFor after Close must refuse: its pump would outlive the relay and hang Close's WaitGroup")
	}
	relay.mu.Lock()
	n := len(relay.flows)
	relay.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d flow(s) registered after Close", n)
	}
}
