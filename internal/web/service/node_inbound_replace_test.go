package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A snapshot tag that matches neither the central tag nor an alias replaces the
// inbound in one tick; the client's accumulated usage must survive the swap.
func TestSetRemoteTraffic_InboundReplacedKeepsClientHistory(t *testing.T) {
	db := initTrafficTestDB(t)
	const nodeID = 2
	if err := db.Create(&model.Node{Id: nodeID, Name: "node", Address: "10.0.0.2", Port: 2053, ApiToken: "t", Guid: "node-guid"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	const email = "baba"
	settings := `{"clients":[{"email":"baba","enable":true}]}`
	createNodeInboundWithClient(t, db, nodeID, "n2-in-2053-tcp", 2053, email)
	svc := &InboundService{}
	sync := func(up, down int64) {
		t.Helper()
		snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
			Tag: "in-2053-tcp", OriginNodeGuid: "node-guid", Enable: true, Port: 2053, Protocol: model.VLESS,
			Settings: settings, ClientStats: []xray.ClientTraffic{{Email: email, Up: up, Down: down, Enable: true}},
		}}}
		if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false, false); err != nil {
			t.Fatalf("setRemoteTrafficLocked: %v", err)
		}
	}

	sync(100, 200)
	sync(600, 1200)
	assertUpDown(t, readTraffic(t, db, email), 500, 1000, "before the replace")

	// Desync the central row so the next snapshot neither tag-matches nor aliases it.
	if err := db.Model(&model.Inbound{}).Where("node_id = ?", nodeID).
		Updates(map[string]any{"tag": "n2-legacy", "origin_node_guid": "stale-guid"}).Error; err != nil {
		t.Fatalf("desync central inbound: %v", err)
	}

	sync(650, 1300)
	assertUpDown(t, readTraffic(t, db, email), 550, 1100, "replace tick")
	sync(700, 1400)
	assertUpDown(t, readTraffic(t, db, email), 600, 1200, "tick after the replace")

	var ib model.Inbound
	if err := db.Where("node_id = ?", nodeID).First(&ib).Error; err != nil {
		t.Fatalf("read node inbound: %v", err)
	}
	if ib.Tag != "in-2053-tcp" {
		t.Fatalf("fixture did not replace the inbound: surviving tag %q", ib.Tag)
	}
	if ct := readTraffic(t, db, email); ct.InboundId != ib.Id {
		t.Errorf("client_traffics.inbound_id = %d, want the surviving inbound %d", ct.InboundId, ib.Id)
	}
}

// A node inbound created on the master has no origin until its first tag match;
// that empty origin is still this node, so a renamed tag aliases instead of replacing.
func TestSetRemoteTraffic_AliasesInboundWithEmptyOrigin(t *testing.T) {
	db := initTrafficTestDB(t)
	const nodeID = 2
	if err := db.Create(&model.Node{Id: nodeID, Name: "node", Address: "10.0.0.2", Port: 2053, ApiToken: "t", Guid: "node-guid"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	createNodeInbound(t, db, nodeID, "n2-in-2053-tcp", 2053)
	var before model.Inbound
	if err := db.Where("node_id = ?", nodeID).First(&before).Error; err != nil {
		t.Fatalf("read node inbound: %v", err)
	}

	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag: "in-2053-tcp-2", OriginNodeGuid: "node-guid", Enable: true, Port: 2053, Protocol: model.VLESS,
		Settings: `{"clients":[]}`,
	}}}
	if _, err := (&InboundService{}).setRemoteTrafficLocked(nodeID, snap, false, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var rows []model.Inbound
	if err := db.Where("node_id = ?", nodeID).Find(&rows).Error; err != nil {
		t.Fatalf("list node inbounds: %v", err)
	}
	if len(rows) != 1 || rows[0].Id != before.Id {
		t.Fatalf("node inbounds = %#v, want only the original id %d", rows, before.Id)
	}
}
