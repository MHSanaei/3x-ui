package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func seedLegacySettingsForServiceTest(t *testing.T, dbPath string, rows []model.Setting) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("migrate setting table: %v", err)
	}
	for _, row := range rows {
		if err := gdb.Create(&row).Error; err != nil {
			t.Fatalf("insert legacy row %s: %v", row.Key, err)
		}
	}
}

func TestSettingServiceReadsMigratedTypedSettingsAndShadowWritesLegacy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "setting-service.db")
	seedLegacySettingsForServiceTest(t, dbPath, []model.Setting{{Key: "webPort", Value: "8111"}})

	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &SettingService{}
	port, err := svc.GetPort()
	if err != nil {
		t.Fatalf("GetPort failed: %v", err)
	}
	if port != 8111 {
		t.Fatalf("expected migrated port 8111, got %d", port)
	}

	if err := svc.SetPort(9001); err != nil {
		t.Fatalf("SetPort failed: %v", err)
	}

	cfg, err := database.GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings failed: %v", err)
	}
	if cfg.WebPort != 9001 {
		t.Fatalf("expected typed settings WebPort=9001, got %d", cfg.WebPort)
	}

	var legacy model.Setting
	if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", "webPort").First(&legacy).Error; err != nil {
		t.Fatalf("read legacy webPort row: %v", err)
	}
	if legacy.Value != "9001" {
		t.Fatalf("expected legacy shadow write webPort=9001, got %s", legacy.Value)
	}
}

func TestSettingServiceXrayTemplateFallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "setting-template.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &SettingService{}
	template, err := svc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate failed: %v", err)
	}
	if template == "" {
		t.Fatalf("expected embedded xray template fallback, got empty string")
	}
}
