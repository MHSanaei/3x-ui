package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The migration runs on every start, so its guard has to actually match the
// index it created — otherwise every boot re-issues the CREATE.
func TestMigrateClientEmailLowerIndexIsIdempotent(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if !db.Migrator().HasIndex(&model.ClientRecord{}, "idx_clients_email_lower") {
		t.Fatal("idx_clients_email_lower missing after InitDB")
	}
	if err := migrateClientEmailLowerIndex(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !db.Migrator().HasIndex(&model.ClientRecord{}, "idx_clients_email_lower") {
		t.Fatal("idx_clients_email_lower vanished after a second run")
	}

	if IsPostgres() {
		return
	}
	// The identity lookups filter on LOWER(email); without the expression index
	// they seq-scan, which is the cost this migration exists to remove.
	var plan []struct{ Detail string }
	if err := db.Raw("EXPLAIN QUERY PLAN SELECT email FROM clients WHERE LOWER(email) IN ('a')").
		Scan(&plan).Error; err != nil {
		t.Fatalf("explain: %v", err)
	}
	used := false
	for _, row := range plan {
		if strings.Contains(row.Detail, "idx_clients_email_lower") {
			used = true
		}
	}
	if !used {
		t.Errorf("LOWER(email) lookup does not use idx_clients_email_lower: %+v", plan)
	}
}
