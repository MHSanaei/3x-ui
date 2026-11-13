package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
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
	for _, col := range []string{"sub_id", "hwid_hash", "blocked", "first_seen", "last_seen", "user_agent", "device_os", "os_version", "device_model"} {
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

func TestClientHwidBanMigrationPreservesDevices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// Match the quoted, single-line DDL emitted by GORM for existing panels.
	if err := db.Exec("CREATE TABLE `client_hwids` (`id` integer PRIMARY KEY AUTOINCREMENT,`sub_id` text NOT NULL,`hwid_hash` text NOT NULL,`first_seen` integer NOT NULL,`last_seen` integer NOT NULL,`user_agent` text,`device_os` text,`os_version` text,`device_model` text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO client_hwids (sub_id, hwid_hash, first_seen, last_seen, device_model)
		VALUES ('legacy-sub', 'legacy-hash', 123, 456, 'Existing phone')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ClientHwid{}); err != nil {
		t.Fatal(err)
	}
	var device model.ClientHwid
	if err := db.First(&device).Error; err != nil {
		t.Fatal(err)
	}
	if device.Blocked || device.SubID != "legacy-sub" || device.HwidHash != "legacy-hash" || device.FirstSeen != 123 || device.LastSeen != 456 || device.DeviceModel != "Existing phone" {
		t.Fatalf("migration changed existing device: %+v", device)
	}
	if err := db.Model(&device).Update("blocked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&device, device.Id).Error; err != nil || !device.Blocked {
		t.Fatalf("ban was not saved: %+v, %v", device, err)
	}
	if err := db.AutoMigrate(&model.ClientHwid{}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&device, device.Id).Error; err != nil || !device.Blocked {
		t.Fatalf("repeated migration cleared ban: %+v, %v", device, err)
	}
}
