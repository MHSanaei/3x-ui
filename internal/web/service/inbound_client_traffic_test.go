package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
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
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

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
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

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

// TestAddClientTraffic_ExpiryWriteOnlyForConvertedClients locks in that the
// delayed-start persistence pass touches only clients adjustTraffics actually
// converted this poll: the delayed client's negative expiry becomes an absolute
// deadline while an already-absolute expiry passes through byte-identical.
// Before the fix every polled row got its own no-op expiry UPDATE.
func TestAddClientTraffic_ExpiryWriteOnlyForConvertedClients(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	db := database.GetDB()

	const delayedEmail = "delayed-mixed-user"
	const normalEmail = "normal-mixed-user"
	const delayedUID = "ce8d33df-3a64-4f10-8f9b-91c3a8e0d002"
	const normalUID = "ce8d33df-3a64-4f10-8f9b-91c3a8e0d003"
	const sevenDays = int64(7 * 86400000)
	normalExpiry := time.Now().AddDate(0, 1, 0).UnixMilli()

	clients := []model.Client{
		{Email: delayedEmail, ID: delayedUID, Enable: true, ExpiryTime: -sevenDays},
		{Email: normalEmail, ID: normalUID, Enable: true, ExpiryTime: normalExpiry},
	}
	inbound := &model.Inbound{
		Tag: "vless-mixed", Enable: true, Port: 45002, Protocol: model.VLESS,
		Settings: clientsSettings(t, clients),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := InboundService{}
	if err := svc.clientService.SyncInbound(db, inbound.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: delayedEmail, Enable: true, ExpiryTime: -sevenDays}).Error; err != nil {
		t.Fatalf("create delayed traffic row: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: normalEmail, Enable: true, ExpiryTime: normalExpiry}).Error; err != nil {
		t.Fatalf("create normal traffic row: %v", err)
	}

	before := time.Now().UnixMilli()
	err := svc.addClientTraffic(db, []*xray.ClientTraffic{
		{Email: delayedEmail, Up: 10, Down: 20},
		{Email: normalEmail, Up: 30, Down: 40},
	})
	if err != nil {
		t.Fatalf("addClientTraffic: %v", err)
	}

	var delayed xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", delayedEmail).First(&delayed).Error; err != nil {
		t.Fatalf("reload delayed row: %v", err)
	}
	if delayed.ExpiryTime < before+sevenDays-5000 || delayed.ExpiryTime > before+sevenDays+5000 {
		t.Errorf("delayed expiry = %d, want ~now+7d (%d)", delayed.ExpiryTime, before+sevenDays)
	}

	var normal xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", normalEmail).First(&normal).Error; err != nil {
		t.Fatalf("reload normal row: %v", err)
	}
	if normal.ExpiryTime != normalExpiry {
		t.Errorf("normal expiry changed: %d, want %d", normal.ExpiryTime, normalExpiry)
	}
	if normal.Up != 30 || normal.Down != 40 {
		t.Errorf("normal traffic not applied: up=%d down=%d, want 30/40", normal.Up, normal.Down)
	}
}

func TestAddTrafficClientUpdateFailureRollsBackWholeBatch(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	db := database.GetDB()

	for _, email := range []string{"healthy@x", "rejected@x"} {
		if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true}).Error; err != nil {
			t.Fatalf("create client traffic %s: %v", email, err)
		}
	}
	if err := db.Exec(`
		CREATE TRIGGER reject_client_traffic_update
		BEFORE UPDATE OF up, down ON client_traffics
		WHEN OLD.email = 'rejected@x'
		BEGIN
			SELECT RAISE(ABORT, 'blocked client traffic update');
		END`).Error; err != nil {
		t.Fatalf("create update trigger: %v", err)
	}

	batch := []*xray.ClientTraffic{
		{Email: "healthy@x", Up: 100, Down: 200},
		{Email: "rejected@x", Up: 300, Down: 400},
	}
	svc := &InboundService{}
	if _, _, err := svc.AddTraffic(nil, batch); err == nil {
		t.Fatal("AddTraffic succeeded despite a client UPDATE failure")
	}
	assertTraffic := func(email string, up, down int64) {
		t.Helper()
		var got xray.ClientTraffic
		if err := db.Where("email = ?", email).First(&got).Error; err != nil {
			t.Fatalf("load traffic for %s: %v", email, err)
		}
		if got.Up != up || got.Down != down {
			t.Fatalf("traffic for %s = (%d,%d), want (%d,%d)", email, got.Up, got.Down, up, down)
		}
	}
	assertTraffic("healthy@x", 0, 0)
	assertTraffic("rejected@x", 0, 0)

	if err := db.Exec("DROP TRIGGER reject_client_traffic_update").Error; err != nil {
		t.Fatalf("drop update trigger: %v", err)
	}
	if _, _, err := svc.AddTraffic(nil, batch); err != nil {
		t.Fatalf("retry AddTraffic: %v", err)
	}
	assertTraffic("healthy@x", 100, 200)
	assertTraffic("rejected@x", 300, 400)
}

func TestAddClientTrafficResolvesRenamedTuicClientByStableIdentity(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	db := database.GetDB()

	const (
		inboundID  = 18001
		clientUUID = "a0000000-0000-0000-0000-000000000021"
		oldEmail   = "before-rename@x"
		newEmail   = "after-rename@x"
	)
	inbound := &model.Inbound{Id: inboundID, Tag: "tuic-rename", Enable: true, Port: 0, Protocol: model.TUIC}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	client := &model.ClientRecord{Email: newEmail, UUID: clientUUID, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create renamed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inboundID}).Error; err != nil {
		t.Fatalf("create client-inbound link: %v", err)
	}
	for _, email := range []string{oldEmail, newEmail} {
		if err := db.Create(&xray.ClientTraffic{InboundId: inboundID, Email: email, Enable: true}).Error; err != nil {
			t.Fatalf("create traffic row %s: %v", email, err)
		}
	}

	var currentTraffic xray.ClientTraffic
	if err := db.Where("email = ?", newEmail).First(&currentTraffic).Error; err != nil {
		t.Fatal(err)
	}
	stableTrafficID := currentTraffic.Id

	if err := (&InboundService{}).addClientTraffic(db, []*xray.ClientTraffic{{
		Email: oldEmail, TuicTrafficID: stableTrafficID, TuicUUID: clientUUID, TuicInboundId: inboundID, Up: 123, Down: 456,
	}}); err != nil {
		t.Fatalf("add retired snapshot traffic: %v", err)
	}
	for _, test := range []struct {
		email    string
		up, down int64
	}{{oldEmail, 0, 0}, {newEmail, 123, 456}} {
		var got xray.ClientTraffic
		if err := db.Where("email = ?", test.email).First(&got).Error; err != nil {
			t.Fatalf("load traffic for %s: %v", test.email, err)
		}
		if got.Up != test.up || got.Down != test.down {
			t.Errorf("traffic for %s = (%d,%d), want (%d,%d)", test.email, got.Up, got.Down, test.up, test.down)
		}
	}
}
