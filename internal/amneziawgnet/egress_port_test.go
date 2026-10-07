package amneziawgnet

import (
	"encoding/json"
	"net"
	"strconv"
	"testing"
)

// holdEgressBasePort occupies EgressBasePort the way another service would; a
// port the OS already refuses, such as a Windows reservation, needs no holder.
func holdEgressBasePort(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)))
	if err != nil {
		return
	}
	t.Cleanup(func() { ln.Close() })
}

// bridgePort is the port a socks bridge generated right now dials.
func bridgePort(t *testing.T) int {
	t.Helper()
	out, ok := BuildSocksBridge([]byte(`{"protocol":"amneziawg","tag":"awg-hop","settings":{}}`))
	if !ok {
		t.Fatal("bridge rejected")
	}
	var got struct {
		Settings struct {
			Port int `json:"port"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	return got.Settings.Port
}

// Windows can reserve a port range covering EgressBasePort, and any host can run
// another service on it; the egress must still come up and report where.
func TestEgressListenFallsBackWhenBasePortIsTaken(t *testing.T) {
	srv := GetEgressServer()
	srv.Close()
	t.Cleanup(srv.Close)
	holdEgressBasePort(t)

	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen with EgressBasePort taken: %v", err)
	}
	port := srv.Port()
	if port == EgressBasePort {
		t.Fatalf("Port() = %d, the taken EgressBasePort", port)
	}
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("egress not accepting on Port() %d: %v", port, err)
	}
	conn.Close()
}

// Xray's bridges are generated apart from the listener, so a listener that came
// up elsewhere must be reported until the bridges are regenerated for it.
func TestBridgesStaleUntilRegeneratedForTheBoundPort(t *testing.T) {
	srv := GetEgressServer()
	srv.Close()
	t.Cleanup(srv.Close)
	holdEgressBasePort(t)

	if got := bridgePort(t); got != EgressBasePort {
		t.Fatalf("bridge generated before Listen dials %d, want %d", got, EgressBasePort)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	if !BridgesStale() {
		t.Fatalf("bridges dial %d while the egress listens on %d, but BridgesStale() = false", EgressBasePort, srv.Port())
	}
	if got := bridgePort(t); got != srv.Port() || BridgesStale() {
		t.Fatalf("regenerated bridge dials %d with BridgesStale() = %v, want %d and false", got, BridgesStale(), srv.Port())
	}
}
