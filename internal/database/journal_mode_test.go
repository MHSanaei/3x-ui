package database

import (
	"path/filepath"
	"testing"
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
