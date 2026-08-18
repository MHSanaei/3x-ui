package database

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func assertClientHwidSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasColumn(&model.ClientRecord{}, "limit_hwid") {
		t.Fatalf("clients.limit_hwid missing")
	}
	if !db.Migrator().HasTable(&model.ClientHwid{}) {
		t.Fatalf("client_hwids table missing")
	}
	for _, col := range []string{"sub_id", "hwid_hash", "first_seen", "last_seen", "user_agent", "device_os", "os_version", "device_model"} {
		if !db.Migrator().HasColumn(&model.ClientHwid{}, col) {
			t.Fatalf("client_hwids.%s missing", col)
		}
	}
	if !db.Migrator().HasIndex(&model.ClientHwid{}, "idx_client_hwids_sub_hash") {
		t.Fatalf("client_hwids unique hash index missing")
	}
}

func TestClientHwidSchemaSQLite(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	assertClientHwidSchema(t, GetDB())
}

// TestIsIgnorableTableExistsErr covers the real production incident (#5802
// follow-up): AutoMigrate re-issuing a bare CREATE TABLE for *model.ClientHwid
// against a database that already has it, correctly shaped, from an earlier
// run -- previously fatal (crashed the panel on every subsequent start until
// the table was manually dropped), now tolerated the same way a duplicate
// column already is.
func TestIsIgnorableTableExistsErr(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	gdb := GetDB()

	if !isIgnorableTableExistsErr(gdb, errors.New("table `client_hwids` already exists"), &model.ClientHwid{}) {
		t.Fatal("want true: client_hwids really exists (InitDB just created it) and the error names a table, not a column")
	}
	if isIgnorableTableExistsErr(gdb, errors.New("table `no_such_table` already exists"), &model.ClientHwid{}) {
		t.Fatal("want false: HasTable must be checked against the model actually passed in, not assumed from the error text alone")
	}
	if isIgnorableTableExistsErr(gdb, nil, &model.ClientHwid{}) {
		t.Fatal("want false: nil error")
	}
	if isIgnorableTableExistsErr(gdb, errors.New(`duplicate column name: sub_id`), &model.ClientHwid{}) {
		t.Fatal("want false: column-shaped errors are isIgnorableDuplicateColumnErr's job, not this one's")
	}
	if isIgnorableTableExistsErr(gdb, errors.New(`column "sub_id" of relation "client_hwids" already exists`), &model.ClientHwid{}) {
		t.Fatal("want false: Postgres column-already-exists text also isn't this function's job")
	}
}

func TestClientHwidSchemaPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to a reachable Postgres to run this test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("postgres db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.ClientRecord{}, &model.ClientHwid{}); err != nil {
		t.Fatalf("automigrate postgres: %v", err)
	}
	assertClientHwidSchema(t, db)
}
