package service

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedClientRow(t *testing.T, email string, inboundId int, up, down, total int64) {
	t.Helper()
	db := database.GetDB()
	if err := db.Create(&xray.ClientTraffic{InboundId: inboundId, Email: email, Enable: true, Up: up, Down: down, Total: total}).Error; err != nil {
		t.Fatalf("seed client_traffics %q: %v", email, err)
	}
}

func TestAcceptGlobalTraffic_SideTableOnly(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedClientRow(t, "alice", 1, 100, 100, 0)

	err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{
		{Email: "alice", Up: 900, Down: 800},
		{Email: "ghost", Up: 5, Down: 5}, // not hosted here — must be dropped
	})
	if err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}

	local := readTraffic(t, db, "alice")
	if local.Up != 100 || local.Down != 100 {
		t.Errorf("local counters must stay pure, got up=%d down=%d", local.Up, local.Down)
	}
	var globals []model.ClientGlobalTraffic
	if err := db.Find(&globals).Error; err != nil {
		t.Fatalf("read globals: %v", err)
	}
	if len(globals) != 1 || globals[0].Email != "alice" || globals[0].Up != 900 || globals[0].Down != 800 {
		t.Errorf("unexpected globals: %+v", globals)
	}
}

func TestAcceptGlobalTraffic_OverwriteAndMultiMaster(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedClientRow(t, "alice", 1, 0, 0, 0)

	must := func(guid string, up, down int64) {
		t.Helper()
		if err := svc.AcceptGlobalTraffic(guid, []*xray.ClientTraffic{{Email: "alice", Up: up, Down: down}}); err != nil {
			t.Fatalf("AcceptGlobalTraffic(%s): %v", guid, err)
		}
	}
	must("master-a", 900, 900)
	must("master-a", 50, 50) // a master-side reset propagates by overwrite
	must("master-b", 500, 400)

	rows := []*xray.ClientTraffic{{Email: "alice", Up: 10, Down: 10}}
	overlayGlobalTraffic(db, rows)
	if rows[0].Up != 500 || rows[0].Down != 400 {
		t.Errorf("overlay should fold per-master max, got up=%d down=%d", rows[0].Up, rows[0].Down)
	}
}

func TestDepletedCond_ProbeGuard(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	// No global rows: the cross-panel EXISTS branch is skipped (#5392), but a
	// client over its local quota is still disabled.
	if got, _ := depletedCond(db); got != depletedClientsCondLocal {
		t.Fatalf("empty globals must use the local-only predicate")
	}
	seedClientRow(t, "local-cap", 1, 600, 600, 1000)
	if _, count, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("disableInvalidClients: %v", err)
	} else if count != 1 {
		t.Fatalf("local over-quota client must be disabled, disabled %d", count)
	}

	// Once a master pushes a global row, the full predicate is used so combined
	// quota is enforced.
	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "local-cap", Up: 1, Down: 1}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}
	if got, _ := depletedCond(db); got != depletedClientsCond {
		t.Fatalf("with globals present the cross-panel predicate must be used")
	}
}

func TestStaleGlobalTraffic_Ignored(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	staleAt := time.Now().Add(-globalTrafficFreshWindow - time.Hour).UnixMilli()
	seedStaleGlobal := func(t *testing.T, guid, email string, up, down int64) {
		t.Helper()
		if err := svc.AcceptGlobalTraffic(guid, []*xray.ClientTraffic{{Email: email, Up: up, Down: down}}); err != nil {
			t.Fatalf("AcceptGlobalTraffic(%s): %v", guid, err)
		}
		if err := db.Model(&model.ClientGlobalTraffic{}).
			Where("master_guid = ? AND email = ?", guid, email).
			Update("updated_at", staleAt).Error; err != nil {
			t.Fatalf("age row: %v", err)
		}
	}

	t.Run("stale row alone neither enforces nor selects the cross-panel predicate", func(t *testing.T) {
		seedClientRow(t, "cap", 1, 100, 100, 1000)
		seedStaleGlobal(t, "dead-master", "cap", 1000, 900)

		if got, _ := depletedCond(db); got != depletedClientsCondLocal {
			t.Fatalf("only stale globals must fall back to the local-only predicate")
		}
		if _, count, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
			t.Fatalf("disableInvalidClients: %v", err)
		} else if count != 0 {
			t.Fatalf("stale global usage must not disable a client, disabled %d", count)
		}
		if got := readTraffic(t, db, "cap"); !got.Enable {
			t.Error("client within its local quota must stay enabled")
		}
	})

	t.Run("stale row does not inflate the displayed total", func(t *testing.T) {
		rows := []*xray.ClientTraffic{{Email: "cap", Up: 100, Down: 100}}
		overlayGlobalTraffic(db, rows)
		if rows[0].Up != 100 || rows[0].Down != 100 {
			t.Errorf("stale global must not overlay display counters, got up=%d down=%d", rows[0].Up, rows[0].Down)
		}
	})

	t.Run("a live master still enforces alongside the stale row", func(t *testing.T) {
		if err := svc.AcceptGlobalTraffic("live-master", []*xray.ClientTraffic{{Email: "cap", Up: 1, Down: 1}}); err != nil {
			t.Fatalf("AcceptGlobalTraffic: %v", err)
		}
		if got, _ := depletedCond(db); got != depletedClientsCond {
			t.Fatalf("a fresh global row must select the cross-panel predicate")
		}
		if _, count, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
			t.Fatalf("disableInvalidClients: %v", err)
		} else if count != 0 {
			t.Fatalf("the live master reports usage well under quota, disabled %d", count)
		}

		if err := svc.AcceptGlobalTraffic("live-master", []*xray.ClientTraffic{{Email: "cap", Up: 600, Down: 500}}); err != nil {
			t.Fatalf("AcceptGlobalTraffic: %v", err)
		}
		if _, count, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
			t.Fatalf("disableInvalidClients: %v", err)
		} else if count != 1 {
			t.Fatalf("fresh cross-panel depletion must disable the client, disabled %d", count)
		}
	})
}

func TestGlobalUsage_DisablesClient(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	// 200 of 1000 used locally — local check alone would never trip.
	seedClientRow(t, "cap", 1, 100, 100, 1000)

	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "cap", Up: 800, Down: 700}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}

	if _, count, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("disableInvalidClients: %v", err)
	} else if count != 1 {
		t.Fatalf("expected 1 client disabled, got %d", count)
	}
	if got := readTraffic(t, db, "cap"); got.Enable {
		t.Error("client should be disabled by global usage exceeding its quota")
	}
}

func TestGlobalRows_ClearedOnReset(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedClientRow(t, "alice", 1, 50, 50, 1000)
	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "alice", Up: 999, Down: 999}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}
	if err := svc.ResetClientTrafficByEmail("alice"); err != nil {
		t.Fatalf("ResetClientTrafficByEmail: %v", err)
	}
	var cnt int64
	db.Model(&model.ClientGlobalTraffic{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("global rows should be cleared on reset, found %d", cnt)
	}
}

// The full inbound list doubles as the traffic snapshot masters poll, so it
// must report pure local counters; the slim list only feeds this panel's UI,
// so it carries the cross-panel overlay.
func TestSnapshotListNotOverlaid_SlimOverlaid(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	settings := `{"clients": [{"email": "alice", "enable": true}]}`
	ib := &model.Inbound{UserId: 1, Tag: "in-a", Enable: true, Port: 42001, Protocol: model.VLESS, Settings: settings}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	seedClientRow(t, "alice", ib.Id, 100, 100, 0)
	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "alice", Up: 900, Down: 900}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}

	full, err := svc.GetInbounds(1)
	if err != nil {
		t.Fatalf("GetInbounds: %v", err)
	}
	if len(full) != 1 || len(full[0].ClientStats) != 1 {
		t.Fatalf("unexpected full list shape: %d inbounds", len(full))
	}
	if full[0].ClientStats[0].Up != 100 {
		t.Errorf("full list (master snapshot) must stay un-overlaid, got up=%d", full[0].ClientStats[0].Up)
	}

	slim, err := svc.GetInboundsSlim(1)
	if err != nil {
		t.Fatalf("GetInboundsSlim: %v", err)
	}
	if len(slim) != 1 || len(slim[0].ClientStats) != 1 {
		t.Fatalf("unexpected slim list shape: %d inbounds", len(slim))
	}
	if slim[0].ClientStats[0].Up != 900 {
		t.Errorf("slim list should carry the global overlay, got up=%d", slim[0].ClientStats[0].Up)
	}
}

// The master GUID keys one stored row per client, so an oversized one is dropped.
func TestAcceptGlobalTrafficDropsAnOversizedMasterGuid(t *testing.T) {
	db := initTrafficTestDB(t)
	seedClientRow(t, "alice", 1, 0, 0, 0)
	if err := (&InboundService{}).AcceptGlobalTraffic(strings.Repeat("m", maxPanelGuidLen+1), []*xray.ClientTraffic{{Email: "alice", Up: 1, Down: 1}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}
	var rows int64
	db.Model(&model.ClientGlobalTraffic{}).Count(&rows)
	if rows != 0 {
		t.Fatalf("stored %d row(s) under an oversized master GUID", rows)
	}
}

// A node can report tens of thousands of its own clients over quota; one IN list with a
// bound variable per email overran SQLite's limit and stopped every expiry and quota check.
func TestDisableInvalidClientsPastTheSQLVariableLimit(t *testing.T) {
	db := initTrafficTestDB(t)
	const n = 33000
	if err := db.Create(&model.Inbound{UserId: 1, Tag: "big", Port: 41100, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	for _, stmt := range []string{
		`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		 INSERT INTO client_traffics (inbound_id, email, enable, total, up, down) SELECT 1, 'c' || i, true, 1, 1, 1 FROM seq`,
		`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		 INSERT INTO clients (email, enable, sub_id) SELECT 'c' || i, true, 's' || i FROM seq`,
	} {
		if err := db.Exec(stmt, n).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO client_inbounds (client_id, inbound_id) SELECT id, 1 FROM clients`).Error; err != nil {
		t.Fatalf("link clients: %v", err)
	}
	_, count, _, err := (&InboundService{}).disableInvalidClients(db, newTrafficMutationBatch())
	if err != nil || count != n {
		t.Fatalf("disableInvalidClients = %d, %v; want all %d disabled", count, err, n)
	}
}

// Preloading client stats binds one variable per inbound, so past SQLite's limit the
// master could not even build Xray's config; nodes report inbounds into the same table.
func TestGetAllInboundsPastTheSQLVariableLimit(t *testing.T) {
	db := initTrafficTestDB(t)
	const n = 33000
	if err := db.Exec(`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		INSERT INTO inbounds (user_id, tag, port, protocol, enable, settings, stream_settings, sniffing, listen, remark)
		SELECT 1, 'in-' || i, i, 'vless', true, '{}', '', '', '', '' FROM seq`, n).Error; err != nil {
		t.Fatalf("seed inbounds: %v", err)
	}
	seedClientRow(t, "alice", n, 0, 0, 0)
	inbounds, err := (&InboundService{}).GetAllInbounds()
	if err != nil || len(inbounds) != n {
		t.Fatalf("GetAllInbounds = %d inbounds, %v; want %d", len(inbounds), err, n)
	}
	if last := inbounds[len(inbounds)-1]; len(last.ClientStats) != 1 || last.ClientStats[0].Email != "alice" {
		t.Fatalf("last inbound's client stats = %v, want alice's row", last.ClientStats)
	}
}

// Saving every renewed inbound in one statement binds every column of every row, which
// overran SQLite's variable limit once enough inbounds held a client due for renewal.
func TestAutoRenewClientsPastTheSQLVariableLimit(t *testing.T) {
	db := initTrafficTestDB(t)
	const n = 1400
	for _, stmt := range []string{
		`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		 INSERT INTO inbounds (user_id, tag, port, protocol, enable, settings, stream_settings, sniffing, listen, remark)
		 SELECT 1, 'in-' || i, i, 'vless', true, '{"clients":[{"email":"c' || i || '","enable":true,"expiryTime":1000,"reset":30}]}', '', '', '', '' FROM seq`,
		`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		 INSERT INTO clients (email, enable, sub_id, expiry_time, reset) SELECT 'c' || i, true, 's' || i, 1000, 30 FROM seq`,
		`WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM seq WHERE i < ?)
		 INSERT INTO client_traffics (inbound_id, email, enable, expiry_time, reset) SELECT i, 'c' || i, true, 1000, 30 FROM seq`,
	} {
		if err := db.Exec(stmt, n).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO client_inbounds (client_id, inbound_id) SELECT c.id, i.id FROM clients c JOIN inbounds i ON i.tag = 'in-' || substr(c.email, 2)`).Error; err != nil {
		t.Fatalf("link clients: %v", err)
	}
	_, count, err := (&InboundService{}).autoRenewClients(db, newTrafficMutationBatch())
	if err != nil || count != n {
		t.Fatalf("autoRenewClients = %d, %v; want all %d renewed", count, err, n)
	}
}

// One inbound whose stored settings do not parse (a node can send such a blob) must not
// stop the master disabling depleted clients everywhere else.
func TestDisableInvalidClientsSurvivesAnInboundWithBrokenSettings(t *testing.T) {
	db := initTrafficTestDB(t)
	for _, ib := range []*model.Inbound{
		{UserId: 1, Tag: "broken", Port: 41201, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[`},
		{UserId: 1, Tag: "local", Port: 41202, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[{"email":"bob","enable":true}]}`},
	} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound: %v", err)
		}
	}
	for i, email := range []string{"alice", "bob"} {
		seedClientRow(t, email, i+1, 10, 10, 5)
		rec := &model.ClientRecord{Email: email, Enable: true, SubID: "s-" + email}
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("seed client: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: i + 1}).Error; err != nil {
			t.Fatalf("link client: %v", err)
		}
	}
	_, count, _, err := (&InboundService{}).disableInvalidClients(db, newTrafficMutationBatch())
	if err != nil || count != 2 {
		t.Fatalf("disableInvalidClients = %d, %v; want both depleted clients disabled", count, err)
	}
	var bob model.Inbound
	db.Where("tag = ?", "local").First(&bob)
	if !strings.Contains(bob.Settings, `"enable":false`) {
		t.Fatalf("local inbound settings = %s, want bob disabled", bob.Settings)
	}
}
