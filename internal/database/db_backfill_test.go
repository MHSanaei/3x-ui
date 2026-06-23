package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The upgrade backfill must seed Billed on BOTH client_traffics and the per-node
// delta baseline node_client_traffics. Seeding only client_traffics leaves the
// node baseline at billed=0 while up/down keep their pre-upgrade totals, so the
// first node-traffic sync folds the node's entire history as a Billed delta and
// double-bills every node client (#review). It must stay idempotent: rows that
// already carry Billed, or that moved no Real, are left untouched.
func TestBackfillBilledTraffic_SeedsClientAndNodeBaselines(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := db.Create(&xray.ClientTraffic{Email: "legacy@x", Up: 700, Down: 300}).Error; err != nil {
		t.Fatalf("seed client traffic: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: "legacy@x", Up: 1000, Down: 500}).Error; err != nil {
		t.Fatalf("seed node baseline: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 2, Email: "active@x", Up: 1000, BilledUp: 2000}).Error; err != nil {
		t.Fatalf("seed already-billed node baseline: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 3, Email: "reset@x"}).Error; err != nil {
		t.Fatalf("seed reset node baseline: %v", err)
	}

	if err := backfillBilledTraffic(); err != nil {
		t.Fatalf("backfillBilledTraffic: %v", err)
	}

	var ct xray.ClientTraffic
	if err := db.Where("email = ?", "legacy@x").First(&ct).Error; err != nil {
		t.Fatalf("reload client traffic: %v", err)
	}
	if ct.BilledUp != 700 || ct.BilledDown != 300 {
		t.Errorf("client_traffics billed = %d/%d, want 700/300", ct.BilledUp, ct.BilledDown)
	}

	var legacy model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", 1, "legacy@x").First(&legacy).Error; err != nil {
		t.Fatalf("reload node baseline: %v", err)
	}
	if legacy.BilledUp != 1000 || legacy.BilledDown != 500 {
		t.Errorf("node baseline billed = %d/%d, want 1000/500 (seeded from Real so the first sync delta is ~0, not the node's whole history)", legacy.BilledUp, legacy.BilledDown)
	}

	var active model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", 2, "active@x").First(&active).Error; err != nil {
		t.Fatalf("reload active baseline: %v", err)
	}
	if active.BilledUp != 2000 {
		t.Errorf("active node baseline billed_up = %d, want 2000 (already-billed row must be left untouched)", active.BilledUp)
	}

	var reset model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", 3, "reset@x").First(&reset).Error; err != nil {
		t.Fatalf("reload reset baseline: %v", err)
	}
	if reset.BilledUp != 0 || reset.BilledDown != 0 {
		t.Errorf("reset node baseline billed = %d/%d, want 0/0 (no Real moved => stays zero)", reset.BilledUp, reset.BilledDown)
	}
}
