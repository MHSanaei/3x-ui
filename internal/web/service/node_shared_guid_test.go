package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Cloned node servers ship an identical panelGuid in their copied settings, and
// a node cloned from the master shares the master's own GUID. effectiveNodeGuid
// must keep each physical node in its own attribution bucket by falling back to
// the node-unique id for both collision kinds, while leaving a uniquely-named
// node on its real GUID and never folding transitive (Id 0) nodes.
func TestEffectiveNodeGuid_DisambiguatesAmbiguousGuids(t *testing.T) {
	nodes := []*model.Node{
		{Id: 1, Guid: "dup"},
		{Id: 2, Guid: "dup"},
		{Id: 3, Guid: "uniq"},
		{Id: 4, Guid: ""},
		{Id: 5, Guid: "master"},
		{Id: 0, Guid: "transitive"},
	}
	ambiguous := ambiguousNodeGuids(nodes, "master")

	if _, ok := ambiguous["dup"]; !ok {
		t.Fatalf("dup must be flagged ambiguous, got %v", ambiguous)
	}
	if _, ok := ambiguous["master"]; !ok {
		t.Fatalf("a node sharing the master GUID must be flagged, got %v", ambiguous)
	}
	if _, ok := ambiguous["uniq"]; ok {
		t.Fatalf("uniq must not be flagged, got %v", ambiguous)
	}
	if _, ok := ambiguous["transitive"]; ok {
		t.Fatalf("transitive (Id 0) must not count, got %v", ambiguous)
	}

	cases := map[*model.Node]string{
		nodes[0]: "node:1",
		nodes[1]: "node:2",
		nodes[2]: "uniq",
		nodes[3]: "node:4",
		nodes[4]: "node:5",
		nodes[5]: "transitive",
	}
	for n, want := range cases {
		if got := effectiveNodeGuid(n, ambiguous); got != want {
			t.Errorf("effectiveNodeGuid(Id=%d, Guid=%q) = %q, want %q", n.Id, n.Guid, got, want)
		}
	}
}

// effectiveNodeKey (the no-preloaded-list variant used by the write paths) must
// agree with the slice helper: fall back to the node-unique id when a GUID is
// shared with another node or with the master, else keep the real GUID.
func TestEffectiveNodeKey_FallsBackOnCollision(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	selfGuid, _ := (&SettingService{}).GetPanelGuid()
	if selfGuid == "" {
		t.Fatal("expected a panel guid")
	}

	mk := func(id int, name, guid string) *model.Node {
		n := &model.Node{Id: id, Name: name, Address: fmt.Sprintf("10.0.0.%d", id), Port: 2053, ApiToken: "t", Guid: guid, Status: "online"}
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return n
	}
	dupA := mk(1, "a", "shared")
	mk(2, "b", "shared")
	uniq := mk(3, "c", "solo")
	masterClone := mk(4, "d", selfGuid)

	if got := effectiveNodeKey(dupA); got != "node:1" {
		t.Errorf("node-node collision: got %q, want node:1", got)
	}
	if got := effectiveNodeKey(uniq); got != "solo" {
		t.Errorf("unique node: got %q, want solo", got)
	}
	if got := effectiveNodeKey(masterClone); got != "node:4" {
		t.Errorf("master collision: got %q, want node:4", got)
	}
}

// recountByGuid must split per-node counts even when two direct nodes share a
// GUID and their inbounds still carry that shared GUID as origin (pre-backfill).
func TestRecountByGuid_SplitsClonedNodesWithSharedGuid(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	svc := NodeService{}
	selfGuid, _ := (&SettingService{}).GetPanelGuid()

	n1 := &model.Node{Id: 1, Name: "A", Address: "10.0.0.1", Port: 2053, ApiToken: "t", Guid: "dup", Status: "online"}
	n2 := &model.Node{Id: 2, Name: "B", Address: "10.0.0.2", Port: 2053, ApiToken: "t", Guid: "dup", Status: "online"}
	n3 := &model.Node{Id: 3, Name: "C", Address: "10.0.0.3", Port: 2053, ApiToken: "t", Guid: "uniq", Status: "online"}
	for _, n := range []*model.Node{n1, n2, n3} {
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node %s: %v", n.Name, err)
		}
	}

	id1, id2, id3 := 1, 2, 3
	inbounds := []*model.Inbound{
		{Tag: "a", Port: 1001, Protocol: model.VLESS, Settings: `{"clients":[]}`, Enable: true, NodeID: &id1, OriginNodeGuid: "dup"},
		{Tag: "b", Port: 1002, Protocol: model.VLESS, Settings: `{"clients":[]}`, Enable: true, NodeID: &id2, OriginNodeGuid: "dup"},
		{Tag: "c", Port: 1003, Protocol: model.VLESS, Settings: `{"clients":[]}`, Enable: true, NodeID: &id3, OriginNodeGuid: "uniq"},
	}
	for _, ib := range inbounds {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	nodes := []*model.Node{n1, n2, n3}
	svc.recountByGuid(nodes, selfGuid)

	if n1.InboundCount != 1 || n2.InboundCount != 1 {
		t.Errorf("cloned nodes must not share inbound counts: n1=%d n2=%d, want 1,1", n1.InboundCount, n2.InboundCount)
	}
	if n3.InboundCount != 1 {
		t.Errorf("unique node InboundCount = %d, want 1", n3.InboundCount)
	}
}

// A cloned node's IP-attribution subtree must be stored under its node-unique
// key, so a second clone sharing the GUID can't overwrite it in node_client_ips.
func TestMergeClientIpsByGuid_RemapsClonedNodeSubtree(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	svc := &InboundService{}
	now := time.Now().Unix()

	n1 := &model.Node{Id: 1, Name: "A", Address: "10.0.0.1", Port: 2053, ApiToken: "t", Guid: "dup", Status: "online"}
	n2 := &model.Node{Id: 2, Name: "B", Address: "10.0.0.2", Port: 2053, ApiToken: "t", Guid: "dup", Status: "online"}
	for _, n := range []*model.Node{n1, n2} {
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}

	if err := svc.MergeClientIpsByGuid(n1, map[string]map[string][]model.ClientIpEntry{
		"dup": {"u@x": {{IP: "1.1.1.1", Timestamp: now}}},
	}); err != nil {
		t.Fatalf("merge n1: %v", err)
	}

	var rows []model.NodeClientIp
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 attribution row, got %d", len(rows))
	}
	if rows[0].NodeGuid != "node:1" {
		t.Errorf("cloned node IPs must be stored under node-unique key, got %q", rows[0].NodeGuid)
	}
}
