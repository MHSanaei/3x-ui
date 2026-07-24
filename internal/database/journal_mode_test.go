package database

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func journalModeOf(t *testing.T) string {
	t.Helper()
	var mode string
	if err := db.Raw("PRAGMA journal_mode;").Scan(&mode).Error; err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	return mode
}

func TestSqliteJournalModeDefaultsToWal(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbDir := t.TempDir()
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if got := journalModeOf(t); got != "wal" {
		t.Fatalf("journal_mode = %q, want wal", got)
	}
}

func TestSqliteJournalModeEnvOverrideDelete(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "delete")
	dbDir := t.TempDir()
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if got := journalModeOf(t); got != "delete" {
		t.Fatalf("journal_mode = %q, want delete", got)
	}
}

func TestWalCheckpointMakesRawFileBackupComplete(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := db.Create(&model.Setting{Key: "walBackupProbe", Value: "42"}).Error; err != nil {
		t.Fatalf("write setting: %v", err)
	}
	if err := Checkpoint(); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db file: %v", err)
	}
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if err := os.WriteFile(copyPath, raw, 0o600); err != nil {
		t.Fatalf("write copy: %v", err)
	}
	if err := ValidateSQLiteDB(copyPath); err != nil {
		t.Fatalf("checkpointed raw copy must be a valid sqlite db: %v", err)
	}

	dump, err := DumpSQLiteToBytes(copyPath)
	if err != nil {
		t.Fatalf("dump copy: %v", err)
	}
	if !bytes.Contains(dump, []byte("walBackupProbe")) {
		t.Fatal("raw-file backup taken after Checkpoint must contain the latest write")
	}
}
