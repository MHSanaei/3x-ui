package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestInboundAutoMigrateCreatesRealityShortIDRotationColumns(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	migrator := GetDB().Migrator()
	columns := []string{
		"reality_short_ids_rotation_enabled",
		"reality_short_ids_rotation_days",
		"reality_short_ids_rotation_count",
		"reality_short_ids_grace_hours",
		"reality_short_ids_active_count",
		"reality_short_ids_rotation_cursor",
		"reality_short_ids_last_rotation_time",
		"reality_short_ids_next_rotation_time",
		"reality_short_ids_retire_at",
	}
	for _, column := range columns {
		if !migrator.HasColumn(&model.Inbound{}, column) {
			t.Errorf("inbounds table missing column %q", column)
		}
	}
}

func TestMigrateRealityShortIDRotationColumnsFromLegacyInbound(t *testing.T) {
	originalDB := db
	t.Cleanup(func() { db = originalDB })
	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("CREATE TABLE inbounds (id integer primary key autoincrement)").Error; err != nil {
		t.Fatalf("create legacy inbounds: %v", err)
	}
	if err := db.Exec("INSERT INTO inbounds DEFAULT VALUES").Error; err != nil {
		t.Fatalf("seed legacy inbound: %v", err)
	}

	if err := migrateRealityShortIDRotationColumns(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migrateRealityShortIDRotationColumns(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}

	var got struct {
		Enabled      bool  `gorm:"column:reality_short_ids_rotation_enabled"`
		Days         int   `gorm:"column:reality_short_ids_rotation_days"`
		Count        int   `gorm:"column:reality_short_ids_rotation_count"`
		GraceHours   int   `gorm:"column:reality_short_ids_grace_hours"`
		ActiveCount  int   `gorm:"column:reality_short_ids_active_count"`
		Cursor       int   `gorm:"column:reality_short_ids_rotation_cursor"`
		LastRotation int64 `gorm:"column:reality_short_ids_last_rotation_time"`
		NextRotation int64 `gorm:"column:reality_short_ids_next_rotation_time"`
		RetireAt     int64 `gorm:"column:reality_short_ids_retire_at"`
	}
	if err := db.Table("inbounds").First(&got).Error; err != nil {
		t.Fatalf("read migrated inbound: %v", err)
	}
	if got.Enabled || got.Days != 30 || got.Count != 0 || got.GraceHours != 24 ||
		got.ActiveCount != 0 || got.Cursor != 0 || got.LastRotation != 0 ||
		got.NextRotation != 0 || got.RetireAt != 0 {
		t.Fatalf("legacy defaults = %+v, want disabled/30/0/24 and zero state", got)
	}
}
