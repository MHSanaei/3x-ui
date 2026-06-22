package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Cloned node servers ship an identical panelGuid in their copied settings.
// effectiveNodeGuid must keep each physical node in its own attribution bucket
// by falling back to the node-unique id when a GUID is shared, while leaving a
// uniquely-identified node on its real GUID.
func TestEffectiveNodeGuid_DisambiguatesSharedGuids(t *testing.T) {
	nodes := []*model.Node{
		{Id: 1, Guid: "dup"},
		{Id: 2, Guid: "dup"},
		{Id: 3, Guid: "uniq"},
		{Id: 4, Guid: ""},
		{Id: 0, Guid: "transitive"},
	}
	shared := sharedNodeGuids(nodes)

	if _, ok := shared["dup"]; !ok {
		t.Fatalf("dup must be flagged shared, got %v", shared)
	}
	if _, ok := shared["uniq"]; ok {
		t.Fatalf("uniq must not be shared, got %v", shared)
	}
	if _, ok := shared["transitive"]; ok {
		t.Fatalf("transitive (Id 0) must not count toward sharing, got %v", shared)
	}

	cases := map[*model.Node]string{
		nodes[0]: "node:1",
		nodes[1]: "node:2",
		nodes[2]: "uniq",
		nodes[3]: "node:4",
		nodes[4]: "transitive",
	}
	for n, want := range cases {
		if got := effectiveNodeGuid(n, shared); got != want {
			t.Errorf("effectiveNodeGuid(Id=%d, Guid=%q) = %q, want %q", n.Id, n.Guid, got, want)
		}
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
