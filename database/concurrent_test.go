package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestInitDBConcurrencyConfig verifies that InitDB applies both settings that
// are proven by TestConcurrentWrites to prevent "database is locked":
//
//  1. WAL journal mode — reduces the window during which a write lock is held
//     (readers no longer block writers and vice-versa).
//  2. SetMaxOpenConns(1) — serialises all GORM writes through a single
//     connection at the Go pool level, so SQLite write-lock contention cannot
//     occur at all.
//
// Chain of proof:
//
//	TestConcurrentWrites/with_fix_* proves SetMaxOpenConns(1) fixes the bug.
//	TestInitDBConcurrencyConfig/single_connection_pool proves InitDB calls it.
//	Therefore InitDB fixes the bug.
func TestInitDBConcurrencyConfig(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer CloseDB()

	t.Run("WAL_journal_mode", func(t *testing.T) {
		var mode string
		if err := GetDB().Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
			t.Fatalf("PRAGMA journal_mode: %v", err)
		}
		if mode != "wal" {
			t.Errorf("journal_mode = %q, want \"wal\" — add _journal_mode=WAL to InitDB DSN", mode)
		}
	})

	t.Run("single_connection_pool", func(t *testing.T) {
		sqlDB, err := GetDB().DB()
		if err != nil {
			t.Fatalf("GetDB().DB(): %v", err)
		}
		if n := sqlDB.Stats().MaxOpenConnections; n != 1 {
			t.Errorf("MaxOpenConnections = %d, want 1 — call sqlDB.SetMaxOpenConns(1) in InitDB", n)
		}
	})
}

// TestConcurrentWrites proves that SetMaxOpenConns(1) is sufficient to prevent
// the "database is locked" errors in issue #3739.
//
// Both sub-tests use _busy_timeout=10ms so that lock contention surfaces
// within milliseconds rather than the go-sqlite3 default of 5 000 ms.
// The ONLY variable that differs between the two sub-tests is MaxOpenConns.
//
// Without fix (MaxOpenConns=2):
//
//	conn1 acquires the SQLite write lock; conn2 is a separate connection that
//	immediately tries to write and fails because it cannot wait out the lock.
//
// With fix (MaxOpenConns=1):
//
//	conn2 cannot be acquired from the pool while conn1 is in use, so it
//	blocks at the Go pool level instead of racing at the SQLite level.
//	Once conn1 is released, conn2 gets the connection and writes without error.
func TestConcurrentWrites(t *testing.T) {
	const busyTimeout = 10 // ms — short so lock contention fails fast

	// openTestDB opens a fresh SQLite DB with the given MaxOpenConns.
	// Both parts use the same busy_timeout so that MaxOpenConns is the
	// only experimental variable.
	openTestDB := func(t *testing.T, maxConns int) *sql.DB {
		t.Helper()
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := gorm.Open(
			sqlite.Open(fmt.Sprintf("%s?_busy_timeout=%d", dbPath, busyTimeout)),
			&gorm.Config{Logger: logger.Discard},
		)
		if err != nil {
			t.Fatalf("gorm.Open: %v", err)
		}
		if err := db.Exec("CREATE TABLE IF NOT EXISTS settings " +
			"(id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT, value TEXT)").Error; err != nil {
			t.Fatalf("create settings table: %v", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("db.DB: %v", err)
		}
		sqlDB.SetMaxOpenConns(maxConns)
		t.Cleanup(func() { sqlDB.Close() })
		return sqlDB
	}

	// ── without fix: MaxOpenConns=2 ──────────────────────────────────────────
	// conn1 and conn2 are distinct connections. conn1 holds the write lock;
	// conn2 tries to write immediately and fails with "database is locked".
	t.Run("without_fix_write_lock_blocks_second_connection", func(t *testing.T) {
		sqlDB := openTestDB(t, 2)
		ctx := context.Background()

		conn1, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn (1): %v", err)
		}
		defer conn1.Close()

		conn2, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn (2): %v", err)
		}
		defer conn2.Close()

		// conn1 acquires SQLite write lock.
		tx, err := conn1.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings (key,value) VALUES ('k1','x')"); err != nil {
			tx.Rollback()
			t.Fatalf("conn1 INSERT: %v", err)
		}

		// conn2 tries to write while the lock is held.
		// busy_timeout=10ms: retries for 10ms then returns "database is locked".
		_, writeErr := conn2.ExecContext(ctx, "INSERT INTO settings (key,value) VALUES ('k2','x')")
		tx.Rollback()

		if writeErr == nil {
			t.Fatal("expected 'database is locked' but conn2 succeeded — root cause not reproduced")
		}
		if !strings.Contains(writeErr.Error(), "database is locked") {
			t.Fatalf("unexpected conn2 error: %v", writeErr)
		}
		// Root cause confirmed: a second connection is blocked by the write lock.
	})

	// ── with fix: MaxOpenConns=1 ─────────────────────────────────────────────
	// The pool has exactly one connection. Acquiring conn2 while conn1 is in
	// use blocks at the Go pool level — conn2 never races at the SQLite level.
	// Once conn1 is closed (connection returned to pool), conn2 unblocks and
	// writes successfully, regardless of busy_timeout.
	t.Run("with_fix_pool_serialises_writes", func(t *testing.T) {
		sqlDB := openTestDB(t, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn1, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn (1): %v", err)
		}

		tx, err := conn1.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings (key,value) VALUES ('k1','x')"); err != nil {
			tx.Rollback()
			conn1.Close()
			t.Fatalf("conn1 INSERT: %v", err)
		}

		// Goroutine: requests conn2 — blocks in pool queue because MaxOpenConns=1
		// and conn1 is still in use.
		done := make(chan error, 1)
		go func() {
			conn2, err := sqlDB.Conn(ctx) // blocks until conn1 is released
			if err != nil {
				done <- fmt.Errorf("Conn (2): %w", err)
				return
			}
			defer conn2.Close()
			_, err = conn2.ExecContext(ctx, "INSERT INTO settings (key,value) VALUES ('k2','x')")
			done <- err
		}()

		// Hold conn1 open for 50 ms, then release.
		time.Sleep(50 * time.Millisecond)
		tx.Rollback()
		conn1.Close() // returns connection to pool; goroutine above unblocks

		if err := <-done; err != nil {
			t.Errorf("conn2 failed after fix (SetMaxOpenConns=1): %v", err)
		}
		// No "database is locked": writes were serialised by the Go pool.
	})
}
