package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateRemarkTemplateDefaultAddsClientEmail(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	const oldDefault = "{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D"
	if err := db.Create(&model.Setting{Key: "remarkTemplate", Value: oldDefault}).Error; err != nil {
		t.Fatalf("seed remarkTemplate: %v", err)
	}

	if err := migrateRemarkTemplateDefault(); err != nil {
		t.Fatalf("migrateRemarkTemplateDefault: %v", err)
	}

	var got model.Setting
	if err := db.Where("key = ?", "remarkTemplate").First(&got).Error; err != nil {
		t.Fatalf("reload remarkTemplate: %v", err)
	}
	const want = "{{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D"
	if got.Value != want {
		t.Fatalf("remarkTemplate = %q, want %q", got.Value, want)
	}
}

func TestMigrateRemarkTemplateDefaultPreservesCustomValue(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	const custom = "{{INBOUND}}/{{EMAIL}}"
	if err := db.Create(&model.Setting{Key: "remarkTemplate", Value: custom}).Error; err != nil {
		t.Fatalf("seed remarkTemplate: %v", err)
	}

	if err := migrateRemarkTemplateDefault(); err != nil {
		t.Fatalf("migrateRemarkTemplateDefault: %v", err)
	}

	var got model.Setting
	if err := db.Where("key = ?", "remarkTemplate").First(&got).Error; err != nil {
		t.Fatalf("reload remarkTemplate: %v", err)
	}
	if got.Value != custom {
		t.Fatalf("custom remarkTemplate = %q, want %q", got.Value, custom)
	}
}
