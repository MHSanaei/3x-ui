package database

import (
	"path/filepath"
	"testing"
)

// A replaced pool that stays open keeps its database file open; Windows then
// cannot delete or replace that file.
func TestInitDBClosesThePoolItReplaces(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("first InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	replaced, err := GetDB().DB()
	if err != nil {
		t.Fatalf("first pool: %v", err)
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	if err := replaced.Ping(); err == nil || err.Error() != "sql: database is closed" {
		t.Fatalf("replaced pool Ping() = %v, want sql: database is closed", err)
	}
}
