package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestAddClientTraffic_MatchesByEmail covers two scenarios that share one fix:
// client_traffics is keyed by email (one shared row per email no matter how many
// inbounds the client is attached to), so local traffic must be applied by email
// regardless of which inbound_id the row happens to carry.
//
//   - staleEmail: the row points at an inbound id that no longer exists (a deleted
//     earlier incarnation, AddClientStat's OnConflict-DoNothing never refreshes it).
//   - dualEmail: the client is attached to both a node inbound and the mother inbound,
//     but the node inbound was attached first, so the shared row carries the node
//     inbound's id (issue #4921). The old `inbound_id NOT IN (node inbounds)` filter
//     dropped this client's local traffic, leaving it stuck at zero and offline.
//
// Both must have their local traffic counted.
func TestAddClientTraffic_MatchesByEmail(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const staleEmail = "stale-user"
	const dualEmail = "dual-user"

	localInbound := &model.Inbound{UserId: 1, Tag: "local-in", Enable: true, Port: 40001, Protocol: model.VLESS}
	if err := db.Create(localInbound).Error; err != nil {
		t.Fatalf("create local inbound: %v", err)
	}
	nodeID := 1
	nodeInbound := &model.Inbound{UserId: 1, Tag: "node-in", Enable: true, Port: 40002, Protocol: model.VLESS, NodeID: &nodeID}
	if err := db.Create(nodeInbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	if err := db.Create(&xray.ClientTraffic{InboundId: 9999, Email: staleEmail, Enable: true}).Error; err != nil {
		t.Fatalf("create stale client_traffics: %v", err)
	}
	// Attached to both inbounds, but the node inbound won the OnConflict so the
	// shared row is owned by the node inbound id.
	if err := db.Create(&xray.ClientTraffic{InboundId: nodeInbound.Id, Email: dualEmail, Enable: true}).Error; err != nil {
		t.Fatalf("create dual client_traffics: %v", err)
	}

	svc := InboundService{}
	err := svc.addClientTraffic(db, []*xray.ClientTraffic{
		{Email: staleEmail, Up: 10, Down: 20},
		{Email: dualEmail, Up: 30, Down: 40},
	})
	if err != nil {
		t.Fatalf("addClientTraffic: %v", err)
	}

	var stale xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", staleEmail).First(&stale).Error; err != nil {
		t.Fatalf("reload stale row: %v", err)
	}
	if stale.Up != 10 || stale.Down != 20 {
		t.Errorf("stale-pointer row not updated: up=%d down=%d, want 10/20", stale.Up, stale.Down)
	}
	if stale.LastOnline == 0 {
		t.Errorf("stale-pointer row LastOnline not set")
	}

	var dual xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", dualEmail).First(&dual).Error; err != nil {
		t.Fatalf("reload dual row: %v", err)
	}
	if dual.Up != 30 || dual.Down != 40 {
		t.Errorf("node-owned row not updated by local traffic (issue #4921): up=%d down=%d, want 30/40", dual.Up, dual.Down)
	}
	if dual.LastOnline == 0 {
		t.Errorf("node-owned row LastOnline not set (client stayed offline)")
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
