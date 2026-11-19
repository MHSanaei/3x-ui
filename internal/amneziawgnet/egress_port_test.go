package amneziawgnet

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"
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

// An OS-chosen port can land on the Xray API port or another panel relay range on hosts whose
// ephemeral range reaches that high (Windows), so the fallback must pick below that range.
func TestEgressFallbackStaysBelowThePanelRanges(t *testing.T) {
	srv := GetEgressServer()
	srv.Close()
	t.Cleanup(srv.Close)
	var asked []string
	orig := listenTCP
	listenTCP = func(ctx context.Context, addr string) (net.Listener, error) {
		asked = append(asked, addr)
		if addr == net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)) {
			return nil, errors.New("base port taken")
		}
		return orig(ctx, addr)
	}
	t.Cleanup(func() { listenTCP = orig })

	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	for _, addr := range asked[1:] {
		_, p, _ := net.SplitHostPort(addr)
		if port, _ := strconv.Atoi(p); port == 0 || port >= reservedLoopbackFloor {
			t.Fatalf("fallback asked for %s, want a fixed port below %d", addr, reservedLoopbackFloor)
		}
	}
	if port := srv.Port(); port >= reservedLoopbackFloor {
		t.Fatalf("Port() = %d, inside the panel's reserved range", port)
	}
}

// TestEgressUDPSessionsShareOneBudget: a proxy user opens an association per
// Mux frame, so the session ceiling must hold across all of them together.
func TestEgressUDPSessionsShareOneBudget(t *testing.T) {
	saved := egressMaxUDPSessions
	egressMaxUDPSessions = 64
	t.Cleanup(func() { egressMaxUDPSessions = saved })

	local := netip.MustParseAddr("10.211.4.1")
	dev := &Device{Stack: newTestStack(t, local.String()), localAddrs: []netip.Addr{local}}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer udpConn.Close()
	client := netip.MustParseAddrPort("127.0.0.1:9")
	dst := func(port int) netip.AddrPort {
		return netip.AddrPortFrom(netip.MustParseAddr("192.0.2.1"), uint16(port))
	}
	var associations []*udpEgressSessions
	associate := func() *udpEgressSessions {
		sessions := &udpEgressSessions{m: map[netip.AddrPort]*egressUDPSession{}}
		associations = append(associations, sessions)
		return sessions
	}
	session := func(sessions *udpEgressSessions, port int) *egressUDPSession {
		sessions.mu.Lock()
		defer sessions.mu.Unlock()
		return sessions.m[dst(port)]
	}
	liveSessions := func() int {
		n := 0
		for _, sessions := range associations {
			sessions.mu.Lock()
			n += len(sessions.m)
			sessions.mu.Unlock()
		}
		return n
	}
	slotsTaken := func() int {
		egressUDPSessions.mu.Lock()
		defer egressUDPSessions.mu.Unlock()
		return len(egressUDPSessions.live)
	}
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s (%d live sessions)", what, liveSessions())
			}
		}
	}

	before := slotsTaken()
	others := associate()
	busy := others.getOrDial(dev, "t", udpConn, client, dst(1))
	if busy == nil || others.getOrDial(dev, "t", udpConn, client, dst(2)) == nil {
		t.Fatal("no session with the budget unspent")
	}
	// Three times the budget, spread over associations that stay small.
	for range 3 * egressMaxUDPSessions / 16 {
		// The oldest session of all, but never the longest idle one.
		GetEgressServer().deliverUDPDatagram(dev, "t", udpConn, client, others, dst(1), []byte("x"))
		flood := associate()
		for port := 1; port <= 16; port++ {
			if flood.getOrDial(dev, "t", udpConn, client, dst(port)) == nil {
				t.Fatalf("destination %d refused: the budget must evict, not drop", port)
			}
		}
	}
	waitFor("evicted sessions to retire", func() bool { return liveSessions() <= egressMaxUDPSessions })
	if session(others, 2) != nil {
		t.Error("the longest-idle session outlived a spent budget")
	}
	if session(others, 1) != busy {
		t.Error("a session that kept relaying was evicted ahead of idle ones")
	}

	for _, sessions := range associations {
		sessions.closeAll()
	}
	waitFor("closeAll to hand every session back", func() bool { return liveSessions() == 0 && slotsTaken() <= before })
}
