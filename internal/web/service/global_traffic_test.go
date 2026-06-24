package service

import (
	"testing"

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
	if got := depletedCond(db); got != depletedClientsCondLocal {
		t.Fatalf("empty globals must use the local-only predicate")
	}
	seedClientRow(t, "local-cap", 1, 600, 600, 1000)
	if _, count, _, err := svc.disableInvalidClients(db); err != nil {
		t.Fatalf("disableInvalidClients: %v", err)
	} else if count != 1 {
		t.Fatalf("local over-quota client must be disabled, disabled %d", count)
	}

	// Once a master pushes a global row, the full predicate is used so combined
	// quota is enforced.
	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "local-cap", Up: 1, Down: 1}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}
	if got := depletedCond(db); got != depletedClientsCond {
		t.Fatalf("with globals present the cross-panel predicate must be used")
	}
}

func TestGlobalUsage_DisablesClient(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	// 200 of 1000 used locally — local check alone would never trip.
	seedClientRow(t, "cap", 1, 100, 100, 1000)

	if err := svc.AcceptGlobalTraffic("master-a", []*xray.ClientTraffic{{Email: "cap", Up: 800, Down: 700}}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}

	if _, count, _, err := svc.disableInvalidClients(db); err != nil {
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
