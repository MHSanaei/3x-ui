package xray

import (
	"testing"
	"time"
)

func TestActiveProfilesSnapshotLifecycle(t *testing.T) {
	p := newOnlineTestProcess()
	now := time.Now()
	p.SetLocalActiveProfiles([]OnlineUser{{Email: "alice", InboundTags: []string{"vless", "hysteria", "vless"}}, {Email: "bob", InboundTags: []string{"trojan"}}}, true, now)
	s := p.GetLocalActiveProfiles(now)
	assertSameSet(t, "alice", s.Clients["alice"], []string{"vless", "hysteria"})
	s.Clients["alice"][0] = "mutated"
	assertSameSet(t, "no snapshot alias", p.GetLocalActiveProfiles(now).Clients["alice"], []string{"vless", "hysteria"})
	p.SetLocalActiveProfiles(nil, true, now.Add(time.Second))
	if got := p.GetLocalActiveProfiles(now.Add(time.Second)); !got.Supported || len(got.Clients) != 0 {
		t.Fatalf("disconnect not cleared: %+v", got)
	}
	if p.GetLocalActiveProfiles(now.Add(activeProfileTTL + time.Second)).Supported {
		t.Fatal("stale data presented as supported")
	}
	p.SetLocalActiveProfiles(nil, false, now)
	if p.GetLocalActiveProfiles(now).Supported {
		t.Fatal("old core presented as exact")
	}
}

func TestActiveProfilesClusterScopesFreshnessAndRemoval(t *testing.T) {
	p := newOnlineTestProcess()
	now := time.Now()
	old := ActiveProfileSnapshot{Supported: true, ObservedAt: 1, Clients: map[string][]string{"alice": {"vless"}}}
	newer := ActiveProfileSnapshot{Supported: true, ObservedAt: 2, Clients: map[string][]string{}}
	p.SetNodeActiveProfiles(1, ActiveProfileTree{"node-a": old, "node-b": old}, now)
	p.SetNodeActiveProfiles(2, ActiveProfileTree{"node-a": newer}, now)
	got := p.GetMergedActiveProfiles(now)
	if len(got["node-a"].Clients) != 0 || len(got["node-b"].Clients) != 1 {
		t.Fatalf("cross-node or old path contamination: %+v", got)
	}
	p.ClearNodeOnlineClients(2)
	if len(p.GetMergedActiveProfiles(now)["node-a"].Clients) != 1 {
		t.Fatal("clear did not remove the branch")
	}
	p.RetainNodeOnlineClients(func(id int) bool { return false })
	if len(p.GetMergedActiveProfiles(now)) != 0 {
		t.Fatal("disabled node remains")
	}
	p.SetNodeActiveProfiles(1, ActiveProfileTree{"node-a": old}, now)
	if len(p.GetMergedActiveProfiles(now.Add(activeProfileTTL))) != 0 {
		t.Fatal("stale remote remains")
	}
}
