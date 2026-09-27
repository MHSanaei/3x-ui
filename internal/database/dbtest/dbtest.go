// Package dbtest opens throwaway panel databases for tests. Migrating a new
// SQLite file costs ~850ms under -race; copying a migrated template ~130ms.
package dbtest

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

var migrated struct {
	once sync.Once
	data []byte
	err  error
}

// InitDB opens a new, fully migrated panel database at path and closes it when
// t ends. Reopen an existing file with database.InitDB instead.
func InitDB(t testing.TB, path string) {
	t.Helper()
	if config.GetDBKind() != "postgres" {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("dbtest.InitDB would overwrite existing %s; reopen it with database.InitDB", path)
		}
		data, err := migratedTemplate()
		if err != nil {
			t.Fatalf("build template database: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create database dir: %v", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("copy template database: %v", err)
		}
	}
	if err := database.InitDB(path); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func migratedTemplate() ([]byte, error) {
	migrated.once.Do(func() {
		dir, err := os.MkdirTemp("", "xui-dbtest-")
		if err != nil {
			migrated.err = err
			return
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "template.db")
		if err := database.InitDB(path); err != nil {
			migrated.err = err
			return
		}
		// Closing the last connection checkpoints the WAL into the main file.
		if err := database.CloseDB(); err != nil {
			migrated.err = err
			return
		}
		migrated.data, migrated.err = os.ReadFile(path)
	})
	return migrated.data, migrated.err
}
