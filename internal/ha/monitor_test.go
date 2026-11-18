package ha

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakePG struct {
	mu         sync.Mutex
	inRecovery bool
	err        error
}

func (f *fakePG) IsInRecovery(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inRecovery, f.err
}

type fakePatroni struct{ leader bool }

func (f fakePatroni) IsLeader(context.Context) (bool, error) { return f.leader, nil }

type fakePeers struct {
	mu  sync.Mutex
	out []PeerStatus
}

func (f *fakePeers) set(out ...PeerStatus) {
	f.mu.Lock()
	f.out = out
	f.mu.Unlock()
}

func (f *fakePeers) Probe(context.Context) []PeerStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.out
}

type fakeStandbys struct{ names []string }

func (f fakeStandbys) StreamingStandbys(context.Context, time.Duration) ([]string, error) {
	return f.names, nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestMonitor(pg *fakePG, peers *fakePeers, standbys StandbyLister) (*Monitor, *clock) {
	clk := &clock{t: t0}
	m := NewMonitor(cluster3(), Self{PanelURL: "https://de:2053", SubURL: "https://de:2096"},
		Probes{PG: pg, Patroni: fakePatroni{leader: true}, Peers: peers, Standbys: standbys}, time.Second)
	m.now = clk.now
	return m, clk
}

func TestMonitorReplicaLearnsMasterURLs(t *testing.T) {
	peers := &fakePeers{}
	peers.set(
		PeerStatus{Code: "nl", OK: true, Role: Master, PanelURL: "https://nl:2053", SubURL: "https://nl:2096"},
		PeerStatus{Code: "pl", OK: true, Role: Replica, PanelURL: "https://pl:2053"},
	)
	m, _ := newTestMonitor(&fakePG{inRecovery: true}, peers, nil)

	snap := m.Tick(context.Background())
	if snap.Role != Replica || snap.Master != "nl" {
		t.Fatalf("want replica pointing at nl, got %+v", snap.State)
	}
	if snap.MasterPanelURL != "https://nl:2053" || snap.MasterSubURL != "https://nl:2096" {
		t.Fatalf("redirect targets must be the master's own URLs, got %+v", snap)
	}
}

func TestMonitorMasterAdvertisesOwnURLs(t *testing.T) {
	m, _ := newTestMonitor(&fakePG{}, &fakePeers{}, fakeStandbys{names: []string{"nl"}})

	snap := m.Tick(context.Background())
	if !snap.IsMaster() {
		t.Fatalf("a streaming standby is a quorum vote, want master, got %+v", snap.State)
	}
	if snap.MasterPanelURL != "https://de:2053" || snap.MasterSubURL != "https://de:2096" {
		t.Fatalf("got %+v", snap)
	}
}

func TestMonitorFencesIsolatedPrimary(t *testing.T) {
	peers := &fakePeers{}
	peers.set(PeerStatus{Code: "nl", OK: true, Role: Replica})
	m, clk := newTestMonitor(&fakePG{}, peers, nil)
	if !m.Tick(context.Background()).IsMaster() {
		t.Fatal("want master on start")
	}

	peers.set(PeerStatus{Code: "nl"}, PeerStatus{Code: "pl"})
	clk.t = clk.t.Add(11 * time.Second)
	snap := m.Tick(context.Background())
	if snap.Role != Fenced || snap.MasterPanelURL != "" {
		t.Fatalf("isolated past fence_after must fence with no redirect target, got %+v", snap)
	}
}

func TestMonitorPGErrorIsUnknown(t *testing.T) {
	m, _ := newTestMonitor(&fakePG{err: errors.New("dial tcp: refused")}, &fakePeers{}, fakeStandbys{names: []string{"nl"}})
	if snap := m.Tick(context.Background()); snap.Role != Unknown {
		t.Fatalf("got %+v", snap.State)
	}
}

func TestGatePromotesOnceAndDemotesAfterMaster(t *testing.T) {
	g := NewGate()
	g.Observe(State{Role: Replica})
	g.Observe(State{Role: Fenced, holding: true})
	if closed(g.Promoted()) || closed(g.Demoted()) {
		t.Fatal("no transition before the node is master")
	}
	g.Observe(State{Role: Master})
	g.Observe(State{Role: Master})
	if !closed(g.Promoted()) || closed(g.Demoted()) {
		t.Fatal("master must promote exactly once and not demote")
	}
	g.Observe(State{Role: Fenced})
	if !closed(g.Demoted()) {
		t.Fatal("losing master after promotion must demote")
	}
	g.Observe(State{Role: Master})
	g.Observe(State{Role: Fenced})
}

func TestGateIgnoresNonMasterBeforePromotion(t *testing.T) {
	g := NewGate()
	g.Observe(State{Role: Fenced})
	g.Observe(State{Role: Unknown})
	if closed(g.Demoted()) {
		t.Fatal("a standby that never was master has nothing to demote")
	}
}

func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
