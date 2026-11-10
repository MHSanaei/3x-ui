package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSQLiteConnPragmaBudgetIsDividedAcrossPool(t *testing.T) {
	t.Setenv("XUI_DB_CACHE_MB", "32")
	t.Setenv("XUI_DB_MMAP_MB", "128")

	pragmas := sqliteConnPragmaStatements("NORMAL", 8)
	want := map[string]string{
		"PRAGMA cache_size=-4096":    "",
		"PRAGMA mmap_size=134217728":  "",
		"PRAGMA synchronous=NORMAL":   "",
		"PRAGMA temp_store=MEMORY":    "",
		"PRAGMA busy_timeout=10000":   "",
	}
	for _, pragma := range pragmas {
		if _, ok := want[pragma]; !ok {
			t.Fatalf("unexpected pragma %q", pragma)
		}
		delete(want, pragma)
	}
	if len(want) != 0 {
		t.Fatalf("missing pragmas %v (got %v)", want, pragmas)
	}

	// A tiny budget must not be divided into a useless per-connection cache.
	t.Setenv("XUI_DB_CACHE_MB", "1")
	floored := sqliteConnPragmaStatements("FULL", 8)
	if got := floored[2]; got != "PRAGMA cache_size=-1024" {
		t.Fatalf("cache_size = %q, want -1024", got)
	}
}

func TestSQLiteConnPragmasApplyToEveryPooledConnection(t *testing.T) {
	t.Setenv("XUI_DB_CACHE_MB", "16")
	t.Setenv("XUI_DB_MAX_OPEN_CONNS", "4")
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	ctx := context.Background()
	conns := make([]*sql.Conn, 0, 4)
	for range 4 {
		conn, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire connection: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		conns = append(conns, conn)
	}

	const wantCacheKB = 16 * 1024 / 4
	for i, conn := range conns {
		var cacheKB int
		if err := conn.QueryRowContext(ctx, "PRAGMA cache_size;").Scan(&cacheKB); err != nil {
			t.Fatalf("connection %d: read cache_size: %v", i, err)
		}
		if cacheKB != -wantCacheKB {
			t.Fatalf("connection %d cache_size = %d, want %d", i, cacheKB, -wantCacheKB)
		}
		var tempStore int
		if err := conn.QueryRowContext(ctx, "PRAGMA temp_store;").Scan(&tempStore); err != nil {
			t.Fatalf("connection %d: read temp_store: %v", i, err)
		}
		if tempStore != 2 {
			t.Fatalf("connection %d temp_store = %d, want 2 (MEMORY)", i, tempStore)
		}
	}
}
