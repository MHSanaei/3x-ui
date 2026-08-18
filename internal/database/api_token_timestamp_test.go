package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNormalizeApiTokenCreatedAtSeconds(t *testing.T) {
	originalDB := db
	t.Cleanup(func() { db = originalDB })

	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ApiToken{}); err != nil {
		t.Fatalf("migrate api_tokens: %v", err)
	}

	rows := []model.ApiToken{
		{Name: "seconds", Token: "a", CreatedAt: 1_782_485_394},
		{Name: "milliseconds", Token: "b", CreatedAt: 1_782_485_394_270},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed api tokens: %v", err)
	}

	if err := normalizeApiTokenCreatedAtSeconds(); err != nil {
		t.Fatalf("normalize timestamps: %v", err)
	}
	if err := normalizeApiTokenCreatedAtSeconds(); err != nil {
		t.Fatalf("normalize timestamps again: %v", err)
	}

	var got []model.ApiToken
	if err := db.Order("id asc").Find(&got).Error; err != nil {
		t.Fatalf("read api tokens: %v", err)
	}
	for _, row := range got {
		if row.CreatedAt != 1_782_485_394 {
			t.Fatalf("%s created_at = %d, want seconds", row.Name, row.CreatedAt)
		}
	}
}

func TestMigrateApiTokenScopeAndExpiryFromLegacyTable(t *testing.T) {
	originalDB := db
	t.Cleanup(func() { db = originalDB })
	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE api_tokens (
		id integer primary key autoincrement, name text, token text, enabled numeric, created_at integer
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Exec("INSERT INTO api_tokens(name, token, enabled, created_at) VALUES ('legacy','hash',1,1)").Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := migrateApiTokenScopeAndExpiry(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migrateApiTokenScopeAndExpiry(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
	var row model.ApiToken
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if row.Scope != model.ApiScopeAdmin || row.ExpiresAt != 0 {
		t.Fatalf("legacy defaults = %q/%d, want admin/0", row.Scope, row.ExpiresAt)
	}
}
