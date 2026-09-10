package database

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInitDBRestrictsSQLiteFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbDir := filepath.Join(t.TempDir(), "x-ui")
	dbPath := filepath.Join(dbDir, "x-ui.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if info, err := os.Stat(dbDir); err != nil {
		t.Fatalf("stat db dir: %v", err)
	} else if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("db dir perm = %o, want 700", perm)
	}
	for _, name := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		info, err := os.Stat(name)
		if errors.Is(err, os.ErrNotExist) && name != dbPath {
			continue
		}
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s perm = %o, want 600", filepath.Base(name), perm)
		}
	}
}

func TestInitDBTightensExistingSQLiteFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("seed InitDB: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("seed CloseDB: %v", err)
	}
	// Simulate a store created by an older release under the default umask.
	for _, name := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Chmod(name, 0o644); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("chmod %s: %v", name, err)
		}
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	for _, name := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		info, err := os.Stat(name)
		if errors.Is(err, os.ErrNotExist) && name != dbPath {
			continue
		}
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s perm = %o, want 600", filepath.Base(name), perm)
		}
	}
}
