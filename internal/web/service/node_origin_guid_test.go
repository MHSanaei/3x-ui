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
