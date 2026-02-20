package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func seedLegacySettings(t *testing.T, dbPath string, rows []model.Setting) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := gdb.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("migrate legacy setting table: %v", err)
	}
	for _, row := range rows {
		if err := gdb.Create(&row).Error; err != nil {
			t.Fatalf("insert legacy row %s: %v", row.Key, err)
		}
	}
}

func TestInitDBMigratesLegacySettingsToAppSettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	seedLegacySettings(t, dbPath, []model.Setting{
		{Key: "webPort", Value: "8899"},
		{Key: "subPath", Value: "/legacy-sub/"},
		{Key: "tgBotEnable", Value: "true"},
		{Key: "xrayTemplateConfig", Value: `{"log":{}}`},
	})

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	cfg, err := GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings failed: %v", err)
	}

	if cfg.WebPort != 8899 {
		t.Fatalf("expected WebPort=8899, got %d", cfg.WebPort)
	}
	if cfg.SubPath != "/legacy-sub/" {
		t.Fatalf("expected SubPath migrated, got %q", cfg.SubPath)
	}
	if !cfg.TgBotEnable {
		t.Fatalf("expected TgBotEnable=true from legacy row")
	}
	if cfg.XrayTemplateConfig != `{"log":{}}` {
		t.Fatalf("expected xray template config migrated, got %q", cfg.XrayTemplateConfig)
	}
	if cfg.SessionMaxAge == 0 {
		t.Fatalf("expected default SessionMaxAge to be seeded")
	}
}
