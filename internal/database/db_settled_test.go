package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestInitDBSQLitePragmasAndIndexes(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	if !db.Migrator().HasIndex(&model.Inbound{}, "idx_inbounds_remark") {
		t.Fatal("expected idx_inbounds_remark")
	}
	if !db.Migrator().HasIndex(&model.ClientRecord{}, "idx_clients_tg_id") {
		t.Fatal("expected idx_clients_tg_id")
	}
}

func TestCheckpointMakesSQLiteRawFileBackupCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	const key = "checkpoint_probe"
	if err := db.Create(&model.Setting{Key: key, Value: "present"}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}
	if err := Checkpoint(); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read raw db: %v", err)
	}
	copyPath := filepath.Join(dir, "backup-copy.db")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		t.Fatalf("write copy: %v", err)
	}

	copyDB, err := gorm.Open(sqlite.Open(copyPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	sqlDB, err := copyDB.DB()
	if err != nil {
		t.Fatalf("copy DB handle: %v", err)
	}
	defer sqlDB.Close()

	var got model.Setting
	if err := copyDB.Where("key = ?", key).First(&got).Error; err != nil {
		t.Fatalf("raw file copy missing checkpointed row: %v", err)
	}
}
