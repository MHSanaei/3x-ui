package service

import (
	"path/filepath"
	"testing"
	"time"

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

// TestAdjustTraffics_DelayedStartConvertsDespiteStaleInboundId covers "Start After
// First Use": a delayed-start client carries a negative expiry (the duration) that
// must convert to an absolute deadline on its first traffic tick. When the client's
// email-keyed client_traffics row still points at a deleted inbound (stale inbound_id
// after an inbound delete+recreate), the conversion used to resolve no inbound and
// silently skip, leaving the client perpetually "not started". The fix resolves the
// owning inbound via the client_inbounds link instead.
func TestAdjustTraffics_DelayedStartConvertsDespiteStaleInboundId(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const email = "delayed-user"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0d001"
	const sevenDays = int64(7 * 86400000)

	client := model.Client{Email: email, ID: uid, Auth: uid, Enable: true, ExpiryTime: -sevenDays}
	inbound := &model.Inbound{
		Tag: "vless-delayed", Enable: true, Port: 45001, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       clientsSettings(t, []model.Client{client}),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := InboundService{}
	if err := svc.clientService.SyncInbound(db, inbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	// The email-keyed traffic row survives an inbound delete+recreate pointing at a
	// dead inbound id; client_inbounds still links the client to the live inbound.
	if err := db.Create(&xray.ClientTraffic{InboundId: 9999, Email: email, Enable: true, ExpiryTime: -sevenDays}).Error; err != nil {
		t.Fatalf("create stale traffic row: %v", err)
	}

	before := time.Now().UnixMilli()
	if err := svc.addClientTraffic(db, []*xray.ClientTraffic{{Email: email, Up: 100, Down: 200}}); err != nil {
		t.Fatalf("addClientTraffic: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("reload traffic row: %v", err)
	}
	if row.ExpiryTime <= 0 {
		t.Fatalf("delayed-start expiry not converted: still %d (stale inbound_id skipped the conversion)", row.ExpiryTime)
	}
	if row.ExpiryTime < before+sevenDays-5000 || row.ExpiryTime > before+sevenDays+5000 {
		t.Errorf("converted expiry = %d, want ~now+7d (%d)", row.ExpiryTime, before+sevenDays)
	}

	reloaded, err := svc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	cs, err := svc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(cs) != 1 || cs[0].ExpiryTime <= 0 {
		t.Errorf("inbound settings expiry not converted: %#v", cs)
	}
}
