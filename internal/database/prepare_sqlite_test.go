package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPrepareSQLiteForMigration(t *testing.T) {
	t.Run("rejects non-panel database", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "random.db")
		gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := gdb.Exec("CREATE TABLE notes(id integer primary key, body text)").Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
		closeGorm(gdb)

		err = PrepareSQLiteForMigration(dbPath)
		if err == nil {
			t.Fatal("PrepareSQLiteForMigration accepted a non-panel database, want error")
		}
		if !strings.Contains(err.Error(), "not a 3x-ui panel database") {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), "not a 3x-ui panel database")
		}
	})

	t.Run("upgrades old panel schema", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "old.db")
		gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := gdb.AutoMigrate(&model.User{}, &model.Setting{}, &model.Inbound{}); err != nil {
			t.Fatalf("automigrate legacy subset: %v", err)
		}
		closeGorm(gdb)

		if err := PrepareSQLiteForMigration(dbPath); err != nil {
			t.Fatalf("PrepareSQLiteForMigration rejected an old panel database: %v", err)
		}

		check, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("reopen sqlite: %v", err)
		}
		defer closeGorm(check)
		sqlDB, err := check.DB()
		if err != nil {
			t.Fatalf("sql db: %v", err)
		}
		for _, table := range []string{"client_groups", "client_global_traffics", "outbound_subscriptions"} {
			if !sqliteTableExists(sqlDB, table) {
				t.Errorf("table %s was not created by the schema upgrade", table)
			}
		}
	})
}
