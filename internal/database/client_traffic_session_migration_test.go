package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Legacy client_traffics schema without the session columns, the upgrade path AddColumn must cover.
const legacyClientTrafficNoSessionDDL = "CREATE TABLE `client_traffics` (`id` integer PRIMARY KEY AUTOINCREMENT,`inbound_id` integer,`enable` numeric,`email` text UNIQUE,`up` integer,`down` integer,`expiry_time` integer,`total` integer,`reset` integer DEFAULT 0,`last_online` integer DEFAULT 0)"

func TestMigrateClientTrafficSessionColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	for _, stmt := range []string{
		legacyClientTrafficNoSessionDDL,
		"INSERT INTO client_traffics (inbound_id, enable, email, up, down, expiry_time, total, last_online) VALUES (1, 1, 'legacy', 111, 222, 0, 0, 1735680000000)",
	} {
		if err := legacy.Exec(stmt).Error; err != nil {
			t.Fatalf("seed legacy client_traffics: %v", err)
		}
	}
	sqlDB, err := legacy.DB()
	if err != nil {
		t.Fatalf("legacy db handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB over legacy schema: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var nulls int64
	if err := GetDB().Table("client_traffics").
		Where("session_start IS NULL OR session_up IS NULL OR session_down IS NULL").
		Count(&nulls).Error; err != nil {
		t.Fatalf("count NULL session columns: %v", err)
	}
	if nulls != 0 {
		t.Fatalf("%d pre-existing rows hold NULL session columns; NULL plus a delta stays NULL, so they never track a session", nulls)
	}

	var row xray.ClientTraffic
	if err := GetDB().Where("email = ?", "legacy").First(&row).Error; err != nil {
		t.Fatalf("pre-existing row lost: %v", err)
	}
	if row.SessionStart != 0 || row.SessionUp != 0 || row.SessionDown != 0 {
		t.Fatalf("pre-existing row session = start %d, %d/%d; want an untracked 0, 0/0", row.SessionStart, row.SessionUp, row.SessionDown)
	}
	if row.Up != 111 || row.Down != 222 || row.LastOnline != 1735680000000 {
		t.Fatalf("pre-existing row changed: up %d down %d last_online %d", row.Up, row.Down, row.LastOnline)
	}
	if err := migrateClientTrafficSessionColumns(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
}
