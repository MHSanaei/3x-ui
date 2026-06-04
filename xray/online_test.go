package xray

import (
	"slices"
	"testing"
)

func newOnlineTestProcess() *Process {
	return &Process{newProcess(nil)}
}

func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// TestGetOnlineClientsByNodeScopesPerNode pins the fix for issue #4809: a
// client online on one node must not be reported online on any other node.
func TestGetOnlineClientsByNodeScopesPerNode(t *testing.T) {
	p := newOnlineTestProcess()
	p.RefreshLocalOnline([]string{"user1"}, nil, 1000, 20000)
	p.SetNodeOnlineClients(3, []string{"user1", "user2"})
	p.SetNodeOnlineClients(5, []string{"user3"})

	byNode := p.GetOnlineClientsByNode()

	assertSameSet(t, "local (key 0)", byNode[localNodeKey], []string{"user1"})
	assertSameSet(t, "node 3", byNode[3], []string{"user1", "user2"})
	assertSameSet(t, "node 5", byNode[5], []string{"user3"})

	if slices.Contains(byNode[5], "user1") {
		t.Errorf("user1 leaked onto node 5: %v", byNode[5])
	}
	if slices.Contains(byNode[localNodeKey], "user3") || slices.Contains(byNode[3], "user3") {
		t.Errorf("user3 leaked off node 5: local=%v node3=%v", byNode[localNodeKey], byNode[3])
	}
}

// TestGetOnlineClientsByNodeOmitsEmptyGroups keeps the payload small: a node
// with no online clients (e.g. just cleared) must not appear as an empty key.
func TestGetOnlineClientsByNodeOmitsEmptyGroups(t *testing.T) {
	p := newOnlineTestProcess()
	p.SetNodeOnlineClients(3, []string{"user1"})
	p.SetNodeOnlineClients(7, []string{})

	byNode := p.GetOnlineClientsByNode()

	if _, ok := byNode[7]; ok {
		t.Errorf("node 7 has no online clients but is present: %v", byNode)
	}
	if _, ok := byNode[localNodeKey]; ok {
		t.Errorf("no local clients online but key 0 is present: %v", byNode)
	}
}

// TestGetOnlineClientsUnionDedupes confirms the flat union (used by the
// client-centric / total-count views) still merges every node and dedupes.
func TestGetOnlineClientsUnionDedupes(t *testing.T) {
	p := newOnlineTestProcess()
	p.RefreshLocalOnline([]string{"user1"}, nil, 1000, 20000)
	p.SetNodeOnlineClients(3, []string{"user1", "user2"})

	assertSameSet(t, "union", p.GetOnlineClients(), []string{"user1", "user2"})
}

// TestRefreshLocalOnlineGraceWindow checks the in-memory local set honours the
// grace window: idle-but-recent clients stay online, stale ones age out, and
// the set is derived only from local activity (never the shared DB column).
func TestRefreshLocalOnlineGraceWindow(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = 20000

	p.RefreshLocalOnline([]string{"user1"}, nil, 1000, grace)
	if got := p.GetOnlineClientsByNode()[localNodeKey]; !slices.Contains(got, "user1") {
		t.Fatalf("user1 should be online right after activity, got %v", got)
	}

	p.RefreshLocalOnline([]string{"user2"}, nil, 11000, grace)
	got := p.GetOnlineClientsByNode()[localNodeKey]
	if !slices.Contains(got, "user1") || !slices.Contains(got, "user2") {
		t.Fatalf("both within grace window, got %v", got)
	}

	p.RefreshLocalOnline(nil, nil, 22000, grace)
	got = p.GetOnlineClientsByNode()[localNodeKey]
	if slices.Contains(got, "user1") {
		t.Errorf("user1 (idle 21s, past grace) should have aged out, got %v", got)
	}
	if !slices.Contains(got, "user2") {
		t.Errorf("user2 (idle 11s, within grace) should still be online, got %v", got)
	}
}

// TestGetActiveInboundsByNodeTracksGraceWindow pins the fix for issue #4859: a
// multi-inbound client must only count as online on inbounds that actually
// carried traffic. The active-inbound signal honours the same grace window as
// the online-email signal, and only this panel's tags report under key 0.
func TestGetActiveInboundsByNodeTracksGraceWindow(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = 20000

	p.RefreshLocalOnline([]string{"alice"}, []string{"inbound-a"}, 1000, grace)
	got := p.GetActiveInboundsByNode()[localNodeKey]
	assertSameSet(t, "active after first poll", got, []string{"inbound-a"})

	p.RefreshLocalOnline([]string{"alice"}, []string{"inbound-b"}, 11000, grace)
	got = p.GetActiveInboundsByNode()[localNodeKey]
	assertSameSet(t, "both within grace", got, []string{"inbound-a", "inbound-b"})

	p.RefreshLocalOnline(nil, nil, 22000, grace)
	got = p.GetActiveInboundsByNode()[localNodeKey]
	assertSameSet(t, "inbound-a (idle 21s, past grace) aged out, inbound-b kept", got, []string{"inbound-b"})

	p.RefreshLocalOnline(nil, nil, 40000, grace)
	if _, ok := p.GetActiveInboundsByNode()[localNodeKey]; ok {
		t.Errorf("all inbounds idle past grace, key 0 should be absent: %v", p.GetActiveInboundsByNode())
	}
}

// TestClearNodeOnlineClientsDropsNode mirrors a failed node probe: the node's
// clients must disappear from the per-node map immediately.
func TestClearNodeOnlineClientsDropsNode(t *testing.T) {
	p := newOnlineTestProcess()
	p.SetNodeOnlineClients(3, []string{"user1"})
	p.ClearNodeOnlineClients(3)

	if _, ok := p.GetOnlineClientsByNode()[3]; ok {
		t.Errorf("node 3 should be absent after ClearNodeOnlineClients")
	}
}
