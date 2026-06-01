package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

// TestAddClientTraffic_MatchesDespiteStaleInboundId reproduces the production bug where
// client_traffics rows survive an inbound delete+recreate with a stale inbound_id (the
// shared-by-email row keeps the deleted inbound's id, and AddClientStat's OnConflict-
// DoNothing never refreshes it). The old `inbound_id IN (local inbounds)` filter dropped
// those rows, so local traffic and online status stopped updating. The fix matches by
// email and only excludes rows owned by a node inbound, so a stale local row is still
// updated while a genuine node-owned row is left untouched.
func TestAddClientTraffic_MatchesDespiteStaleInboundId(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const localEmail = "local-user"
	const nodeEmail = "node-user"

	// A local inbound exists, but the local client's traffic row points at an inbound id
	// that no longer exists (a deleted earlier incarnation) — the stale-pointer scenario.
	localInbound := &model.Inbound{UserId: 1, Tag: "local-in", Enable: true, Port: 40001, Protocol: model.VLESS}
	if err := db.Create(localInbound).Error; err != nil {
		t.Fatalf("create local inbound: %v", err)
	}
	nodeID := 1
	nodeInbound := &model.Inbound{UserId: 1, Tag: "node-in", Enable: true, Port: 40002, Protocol: model.VLESS, NodeID: &nodeID}
	if err := db.Create(nodeInbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	if err := db.Create(&xray.ClientTraffic{InboundId: 9999, Email: localEmail, Enable: true}).Error; err != nil {
		t.Fatalf("create stale local client_traffics: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: nodeInbound.Id, Email: nodeEmail, Enable: true}).Error; err != nil {
		t.Fatalf("create node client_traffics: %v", err)
	}

	svc := InboundService{}
	err := svc.addClientTraffic(db, []*xray.ClientTraffic{
		{Email: localEmail, Up: 10, Down: 20},
		{Email: nodeEmail, Up: 30, Down: 40},
	})
	if err != nil {
		t.Fatalf("addClientTraffic: %v", err)
	}

	var local xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", localEmail).First(&local).Error; err != nil {
		t.Fatalf("reload local row: %v", err)
	}
	if local.Up != 10 || local.Down != 20 {
		t.Errorf("stale-pointer local row not updated: up=%d down=%d, want 10/20", local.Up, local.Down)
	}
	if local.LastOnline == 0 {
		t.Errorf("stale-pointer local row LastOnline not set")
	}

	var node xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", nodeEmail).First(&node).Error; err != nil {
		t.Fatalf("reload node row: %v", err)
	}
	if node.Up != 0 || node.Down != 0 {
		t.Errorf("node-owned row should not be touched by local traffic: up=%d down=%d, want 0/0", node.Up, node.Down)
	}
}
