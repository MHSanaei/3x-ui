package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Locks the #5665 guard: composite-PK client_inbounds has no id column, so the
// sequence-reset SQL must never be issued for it.
func TestTableWithIdColumn_SkipsCompositeKeyModels(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if table, ok := tableWithIdColumn(db, &model.ClientInbound{}); ok {
		t.Errorf("ClientInbound (table %q) has no id column but was not skipped", table)
	}
	table, ok := tableWithIdColumn(db, &model.Inbound{})
	if !ok {
		t.Fatal("Inbound has an id column but was reported as skippable")
	}
	if table != "inbounds" {
		t.Errorf("Inbound table = %q, want inbounds", table)
	}
}

// Exercises the #5665 AutoMigrate skip on SQLite (the check is dialect-agnostic):
// settled after InitDB, not settled with a missing column or table.
func TestPostgresModelSettled_TracksSchemaPresence(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	for _, mdl := range []any{&model.ClientRecord{}, &model.ClientGroup{}, &model.ClientInbound{}} {
		if !postgresModelSettled(mdl) {
			t.Errorf("%T not settled right after InitDB", mdl)
		}
	}

	if err := db.Migrator().DropColumn(&model.ClientGroup{}, "reset_up"); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if postgresModelSettled(&model.ClientGroup{}) {
		t.Error("ClientGroup settled despite missing reset_up column")
	}

	if err := db.Migrator().DropTable(&model.ClientGroup{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if postgresModelSettled(&model.ClientGroup{}) {
		t.Error("ClientGroup settled despite missing table")
	}
}
