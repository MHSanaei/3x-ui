package database

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func createBareNode(t *testing.T, name string) *model.Node {
	t.Helper()
	n := &model.Node{Name: name, Address: "127.0.0.1", Port: 2053, ApiToken: "tok"}
	if err := GetDB().Create(n).Error; err != nil {
		t.Fatalf("create node %s: %v", name, err)
	}
	return n
}

func readRawMultiplier(t *testing.T, id int) sql.NullInt64 {
	t.Helper()
	var v sql.NullInt64
	if err := GetDB().Raw("SELECT traffic_multiplier FROM nodes WHERE id = ?", id).Scan(&v).Error; err != nil {
		t.Fatalf("read traffic_multiplier for node %d: %v", id, err)
	}
	return v
}

func checkMultiplierBackfill(t *testing.T) {
	t.Helper()

	// Fresh nodes created without an explicit multiplier land on the column
	// default 100 (1x) — the upgrade changes nothing for new rows.
	fresh := createBareNode(t, "mult-fresh")
	if v := readRawMultiplier(t, fresh.Id); !v.Valid || v.Int64 != 100 {
		t.Fatalf("fresh node multiplier = %+v, want 100 (column default)", v)
	}

	// Rows written before the column existed surface as NULL on an old SQLite
	// ALTER TABLE; a corrupt non-positive value is equally invalid. Both must
	// be repaired to 100 (1x) while a valid operator-set value stays untouched.
	nullRow := createBareNode(t, "mult-null")
	negRow := createBareNode(t, "mult-neg")
	okRow := createBareNode(t, "mult-ok")
	if err := GetDB().Exec("UPDATE nodes SET traffic_multiplier = NULL WHERE id = ?", nullRow.Id).Error; err != nil {
		t.Fatalf("null legacy row: %v", err)
	}
	if err := GetDB().Exec("UPDATE nodes SET traffic_multiplier = -5 WHERE id = ?", negRow.Id).Error; err != nil {
		t.Fatalf("neg legacy row: %v", err)
	}
	if err := GetDB().Exec("UPDATE nodes SET traffic_multiplier = 250 WHERE id = ?", okRow.Id).Error; err != nil {
		t.Fatalf("ok row: %v", err)
	}

	if err := migrateNodeTrafficMultiplier(); err != nil {
		t.Fatalf("migrateNodeTrafficMultiplier: %v", err)
	}

	if v := readRawMultiplier(t, nullRow.Id); !v.Valid || v.Int64 != 100 {
		t.Errorf("NULL legacy row backfilled to %+v, want 100", v)
	}
	if v := readRawMultiplier(t, negRow.Id); !v.Valid || v.Int64 != 100 {
		t.Errorf("non-positive row repaired to %+v, want 100", v)
	}
	if v := readRawMultiplier(t, okRow.Id); !v.Valid || v.Int64 != 250 {
		t.Errorf("valid operator value changed to %+v, want 250 untouched", v)
	}

	// A second run is a no-op for already-healthy rows.
	if err := migrateNodeTrafficMultiplier(); err != nil {
		t.Fatalf("second migrateNodeTrafficMultiplier: %v", err)
	}
	if v := readRawMultiplier(t, okRow.Id); !v.Valid || v.Int64 != 250 {
		t.Errorf("second run changed valid value to %+v, want 250", v)
	}
}

func TestMigrateNodeTrafficMultiplier_SQLite(t *testing.T) {
	initMigrateDB(t)
	checkMultiplierBackfill(t)
}

func TestMigrateNodeTrafficMultiplier_Postgres(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres migration test")
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	// Clean slate so this run owns the table regardless of prior tests.
	GetDB().Exec("TRUNCATE TABLE nodes RESTART IDENTITY CASCADE")
	checkMultiplierBackfill(t)
}
