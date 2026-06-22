package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// #4983: a synced inbound's OriginNodeGuid must point at the panel that
// physically hosts it. A node's own local inbound (empty origin in its
// snapshot) is attributed to the node's own GUID; an inbound the node forwards
// from its own sub-node (non-empty origin) keeps that deeper GUID across the
// hop — so a chained Node1->Node2->Node3 attributes Node3's inbounds to Node3.
func TestSetRemoteTraffic_AttributesOriginNodeGuid(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 1
	if err := db.Create(&model.Node{
		Id:       nodeID,
		Name:     "node2",
		Address:  "10.0.0.2",
		Port:     2053,
		ApiToken: "t",
		Guid:     "node2-guid",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{
			{ // node2's own local inbound — reports no origin
				Tag:      "in-443-tcp",
				Enable:   true,
				Port:     443,
				Protocol: model.VLESS,
				Settings: `{"clients":[]}`,
			},
			{ // forwarded from node2's sub-node (node3) — carries node3's guid
				Tag:            "in-8443-tcp",
				Enable:         true,
				Port:           8443,
				Protocol:       model.VLESS,
				Settings:       `{"clients":[]}`,
				OriginNodeGuid: "node3-guid",
			},
		},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	origin := func(tag string) string {
		var ib model.Inbound
		if err := db.Where("tag = ?", tag).First(&ib).Error; err != nil {
			t.Fatalf("load inbound %q: %v", tag, err)
		}
		return ib.OriginNodeGuid
	}

	if og := origin("in-443-tcp"); og != "node2-guid" {
		t.Fatalf("local inbound origin = %q, want node2-guid (the node's own GUID)", og)
	}
	if og := origin("in-8443-tcp"); og != "node3-guid" {
		t.Fatalf("forwarded inbound origin = %q, want node3-guid (kept across the hop)", og)
	}
}

// A cloned node reports its OWN inbound with its own (duplicated) panelGuid as
// the origin. That must be remapped to the node-unique key, not stored verbatim
// — otherwise origin_node_guid keeps the shared GUID while online is keyed by
// the node-unique key, and the inbound page reads an empty bucket (shows
// offline). A genuinely forwarded sub-node GUID is still kept across the hop.
func TestSetRemoteTraffic_RemapsClonedNodeOwnGuidOrigin(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	// Two nodes share one panelGuid (cloned servers).
	for _, n := range []*model.Node{
		{Id: 1, Name: "a", Address: "10.0.0.1", Port: 2053, ApiToken: "t", Guid: "dup"},
		{Id: 2, Name: "b", Address: "10.0.0.2", Port: 2053, ApiToken: "t", Guid: "dup"},
	} {
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node %s: %v", n.Name, err)
		}
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{
			{ // node 1's OWN inbound, reporting its own (shared) panelGuid as origin
				Tag:            "own-443-tcp",
				Enable:         true,
				Port:           443,
				Protocol:       model.VLESS,
				Settings:       `{"clients":[]}`,
				OriginNodeGuid: "dup",
			},
			{ // forwarded from a sub-node with a distinct guid — kept across the hop
				Tag:            "fwd-8443-tcp",
				Enable:         true,
				Port:           8443,
				Protocol:       model.VLESS,
				Settings:       `{"clients":[]}`,
				OriginNodeGuid: "child-guid",
			},
		},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	origin := func(tag string) string {
		var ib model.Inbound
		if err := db.Where("tag = ?", tag).First(&ib).Error; err != nil {
			t.Fatalf("load inbound %q: %v", tag, err)
		}
		return ib.OriginNodeGuid
	}

	if og := origin("own-443-tcp"); og != "node:1" {
		t.Fatalf("cloned node's own inbound origin = %q, want node:1 (remapped from shared GUID)", og)
	}
	if og := origin("fwd-8443-tcp"); og != "child-guid" {
		t.Fatalf("forwarded inbound origin = %q, want child-guid (kept across the hop)", og)
	}
}

// A node mid-restart can return an empty inbound list with success=true. The
// sync must NOT treat that as "delete all my inbounds" — otherwise a blip wipes
// the node's central inbounds and every client on them (what happened to the
// Germany node: 0 clients but still online).
func TestSetRemoteTraffic_EmptySnapshotKeepsCentralInbounds(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 1
	if err := db.Create(&model.Node{
		Id: nodeID, Name: "n", Address: "10.0.0.1", Port: 2053, ApiToken: "t", Guid: "g",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nidPtr := nodeID
	if err := db.Create(&model.Inbound{
		UserId: 1, NodeID: &nidPtr, Tag: "remote-in", Enable: true,
		Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`,
	}).Error; err != nil {
		t.Fatalf("create central inbound: %v", err)
	}

	// Empty snapshot — the node reported no inbounds this cycle.
	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, &runtime.TrafficSnapshot{}, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var count int64
	if err := db.Model(&model.Inbound{}).Where("tag = ?", "remote-in").Count(&count).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if count != 1 {
		t.Fatalf("empty snapshot must not delete the central inbound; got count = %d", count)
	}
}

func TestSetRemoteTraffic_PreservesLocalShareAddressStrategy(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 1
	if err := db.Create(&model.Node{
		Id:       nodeID,
		Name:     "node2",
		Address:  "10.0.0.2",
		Port:     2053,
		ApiToken: "t",
		Guid:     "node2-guid",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	nodeIDPtr := nodeID
	if err := db.Create(&model.Inbound{
		UserId:            1,
		NodeID:            &nodeIDPtr,
		Tag:               "remote-in",
		Enable:            true,
		Port:              443,
		Protocol:          model.VLESS,
		Settings:          `{"clients":[]}`,
		ShareAddrStrategy: "custom",
		ShareAddr:         "edge.example.com",
	}).Error; err != nil {
		t.Fatalf("create central inbound: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag:      "remote-in",
			Enable:   true,
			Port:     8443,
			Protocol: model.VLESS,
			Settings: `{"clients":[]}`,
		}},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var ib model.Inbound
	if err := db.Where("tag = ?", "remote-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if ib.ShareAddrStrategy != "custom" || ib.ShareAddr != "edge.example.com" {
		t.Fatalf("share address fields were overwritten: strategy=%q addr=%q", ib.ShareAddrStrategy, ib.ShareAddr)
	}
	if ib.Port != 8443 {
		t.Fatalf("sync should still update regular remote fields; port = %d, want 8443", ib.Port)
	}
}

func TestSetRemoteTraffic_DefaultsShareAddressFieldsForNewCentralInbound(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 1
	if err := db.Create(&model.Node{
		Id:       nodeID,
		Name:     "node2",
		Address:  "10.0.0.2",
		Port:     2053,
		ApiToken: "t",
		Guid:     "node2-guid",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag:               "remote-in",
			Enable:            true,
			Port:              8443,
			Protocol:          model.VLESS,
			Settings:          `{"clients":[]}`,
			ShareAddrStrategy: "custom",
			ShareAddr:         "remote.example.com",
		}},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var ib model.Inbound
	if err := db.Where("tag = ?", "remote-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if ib.ShareAddrStrategy != "node" || ib.ShareAddr != "" {
		t.Fatalf("new central inbound share fields = (%q, %q), want (node, empty)", ib.ShareAddrStrategy, ib.ShareAddr)
	}
}
