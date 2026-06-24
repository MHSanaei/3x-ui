package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/op/go-logging"
	"gorm.io/gorm"
)

// setupScaleDB initializes the DB for a scale benchmark on either Postgres
// (XUI_DB_TYPE=postgres + XUI_DB_DSN) or SQLite (XUI_SCALE_TEST=1, temp file),
// and registers cleanup. Skips the test when neither backend is configured.
func setupScaleDB(t *testing.T) {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)

	if os.Getenv("XUI_DB_TYPE") == "postgres" && strings.TrimSpace(os.Getenv("XUI_DB_DSN")) != "" {
		if err := database.InitDB(""); err != nil {
			t.Fatalf("InitDB(postgres): %v", err)
		}
		t.Cleanup(func() { _ = database.CloseDB() })
		return
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("XUI_SCALE_TEST"))) {
	case "1", "true", "yes":
		dbPath := filepath.Join(t.TempDir(), "scale.db")
		if err := database.InitDB(dbPath); err != nil {
			t.Fatalf("InitDB(sqlite): %v", err)
		}
		t.Cleanup(func() { _ = database.CloseDB() })
		return
	}

	t.Skip("set XUI_SCALE_TEST=1 (sqlite) or XUI_DB_TYPE=postgres + XUI_DB_DSN (postgres) to run the scale benchmark")
}

// resetScaleTables empties the given tables between sub-sizes. Postgres uses a
// single TRUNCATE ... CASCADE; SQLite deletes per table and clears the
// autoincrement counters so ids restart like RESTART IDENTITY.
func resetScaleTables(t *testing.T, db *gorm.DB, tables ...string) {
	t.Helper()
	if config.GetDBKind() == "postgres" {
		stmt := "TRUNCATE TABLE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return
	}
	for _, tbl := range tables {
		if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
			t.Fatalf("delete %s: %v", tbl, err)
		}
	}
	// Best-effort id reset; sqlite_sequence is absent until the first insert.
	db.Exec("DELETE FROM sqlite_sequence")
}
