package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestRepairOverflowedTrafficCounters_HealsSQLiteRealPromotion reproduces
// #5762: a counter pushed past int64 makes SQLite silently store the cell as
// REAL, after which scanning the row back into the Go int64 field fails and
// every reader of client_traffics breaks. The startup repair must convert the
// cell back to a scannable integer clamped to TrafficMax.
func TestRepairOverflowedTrafficCounters_HealsSQLiteRealPromotion(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	rows := []xray.ClientTraffic{
		{Email: "overflowed@x", Enable: true, Up: 5, Down: 6},
		{Email: "negative@x", Enable: true, Up: 7, Down: 8},
		{Email: "healthy@x", Enable: true, Up: 100, Down: 200},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create traffic row %d: %v", i, err)
		}
	}

	if err := db.Exec("UPDATE client_traffics SET down = 1.2247589467272907e+19 WHERE email = 'overflowed@x'").Error; err != nil {
		t.Fatalf("corrupt down: %v", err)
	}
	if err := db.Exec("UPDATE client_traffics SET up = -42 WHERE email = 'negative@x'").Error; err != nil {
		t.Fatalf("corrupt up: %v", err)
	}

	var broken []xray.ClientTraffic
	if err := db.Find(&broken).Error; err == nil {
		t.Fatal("expected the REAL-promoted row to break scanning before the repair")
	}

	if err := repairOverflowedTrafficCounters(); err != nil {
		t.Fatalf("repairOverflowedTrafficCounters: %v", err)
	}

	byEmail := map[string]xray.ClientTraffic{}
	var repaired []xray.ClientTraffic
	if err := db.Find(&repaired).Error; err != nil {
		t.Fatalf("scan after repair: %v", err)
	}
	for _, r := range repaired {
		byEmail[r.Email] = r
	}
	if got := byEmail["overflowed@x"].Down; got != TrafficMax {
		t.Errorf("overflowed down: expected clamp to %d, got %d", TrafficMax, got)
	}
	if got := byEmail["overflowed@x"].Up; got != 5 {
		t.Errorf("overflowed up: expected untouched 5, got %d", got)
	}
	if got := byEmail["negative@x"].Up; got != 0 {
		t.Errorf("negative up: expected clamp to 0, got %d", got)
	}
	if got := byEmail["healthy@x"]; got.Up != 100 || got.Down != 200 {
		t.Errorf("healthy row changed: %+v", got)
	}
}

// TestClampedAddExpr_CapsAtTrafficMax verifies the write-path clamp: a delta
// applied to a counter near the cap must saturate at TrafficMax instead of
// overflowing int64 (which SQLite would promote to REAL).
func TestClampedAddExpr_CapsAtTrafficMax(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	row := xray.ClientTraffic{Email: "near-cap@x", Enable: true, Up: TrafficMax - 10, Down: 1}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create traffic row: %v", err)
	}

	query := "UPDATE client_traffics SET up = " + ClampedAddExpr("up") + ", down = " + ClampedAddExpr("down") + " WHERE email = ?"
	if err := db.Exec(query, int64(1_000_000), int64(5), "near-cap@x").Error; err != nil {
		t.Fatalf("clamped add: %v", err)
	}

	var got xray.ClientTraffic
	if err := db.Where("email = ?", "near-cap@x").First(&got).Error; err != nil {
		t.Fatalf("scan after clamped add: %v", err)
	}
	if got.Up != TrafficMax {
		t.Errorf("up: expected saturation at %d, got %d", TrafficMax, got.Up)
	}
	if got.Down != 6 {
		t.Errorf("down: expected 6, got %d", got.Down)
	}
}
