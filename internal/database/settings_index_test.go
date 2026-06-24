package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// settings.key is read on nearly every request and job tick (getSetting
// WHERE key=?); AutoMigrate must create the index so those lookups don't
// full-scan the settings table past the large xrayTemplateConfig blob. gorm
// creates missing indexes on migrate, so this also covers existing DBs.
func TestAutoMigrateCreatesSettingsKeyIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if !db.Migrator().HasIndex(&model.Setting{}, "idx_settings_key") {
		t.Errorf("expected idx_settings_key to exist after AutoMigrate")
	}
}
