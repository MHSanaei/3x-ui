package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Legacy inbounds schema without exclude_from_sub — the upgrade path AddColumn must cover.
const legacyInboundNoExcludeFromSubDDL = "CREATE TABLE `inbounds` (`id` integer PRIMARY KEY AUTOINCREMENT,`user_id` integer,`up` integer,`down` integer,`total` integer,`remark` text,`enable` numeric,`expiry_time` integer,`listen` text,`port` integer,`protocol` text,`settings` text,`stream_settings` text,`tag` text UNIQUE,`sniffing` text)"

func TestMigrateInboundExcludeFromSubColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.Exec(legacyInboundNoExcludeFromSubDDL).Error; err != nil {
		t.Fatalf("create legacy inbounds: %v", err)
	}
	if err := legacy.Exec(
		`INSERT INTO inbounds (user_id, remark, enable, port, protocol, settings, stream_settings, tag, sniffing)
		 VALUES (1, 'preexisting', 1, 443, 'vless', '{"clients":[]}', '{}', 'in-443-tcp', '{}')`,
	).Error; err != nil {
		t.Fatalf("seed legacy inbound: %v", err)
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

	if !GetDB().Migrator().HasColumn(&model.Inbound{}, "exclude_from_sub") {
		t.Fatal("exclude_from_sub column missing after migrateInboundExcludeFromSubColumn")
	}

	var row model.Inbound
	if err := GetDB().Where("tag = ?", "in-443-tcp").First(&row).Error; err != nil {
		t.Fatalf("preexisting inbound lost: %v", err)
	}
	if row.ExcludeFromSub {
		t.Fatal("preexisting row must default exclude_from_sub to false, got true")
	}
	if err := migrateInboundExcludeFromSubColumn(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
}
