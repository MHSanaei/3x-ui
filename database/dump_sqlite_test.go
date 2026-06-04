package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestCopyAllModelsIntoSQLite exercises the same AutoMigrate + copyTable
// machinery that ExportPostgresToSQLite relies on, but with a SQLite source so
// it needs no external database. The Postgres source path uses identical gorm
// reads (see MigrateData), so this validates the destination-side copy.
func TestCopyAllModelsIntoSQLite(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	src, err := gorm.Open(sqlite.Open(srcPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	defer closeGorm(src)
	for _, m := range migrationModels() {
		if err := src.AutoMigrate(m); err != nil {
			t.Fatalf("automigrate src %T: %v", m, err)
		}
	}

	// Seed a few rows across parent/child tables and a composite-PK table.
	if err := src.Create(&model.User{Username: "admin", Password: "x"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := src.Create(&model.Inbound{UserId: 1, Remark: "in", Port: 443, Protocol: "vless", Tag: "inbound-443"}).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := src.Create(&xray.ClientTraffic{InboundId: 1, Email: "a@b.c", Enable: true, Up: 10, Down: 20}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	dst, err := gorm.Open(sqlite.Open(dstPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	defer closeGorm(dst)
	if err := copyAllModels(src, dst); err != nil {
		t.Fatalf("copyAllModels: %v", err)
	}

	for _, tc := range []struct {
		model any
		want  int64
	}{
		{&model.User{}, 1},
		{&model.Inbound{}, 1},
		{&xray.ClientTraffic{}, 1},
	} {
		var got int64
		if err := dst.Model(tc.model).Count(&got).Error; err != nil {
			t.Fatalf("count %T: %v", tc.model, err)
		}
		if got != tc.want {
			t.Errorf("%T: got %d rows, want %d", tc.model, got, tc.want)
		}
	}

	// Spot-check a copied value survived the round-trip.
	var ct xray.ClientTraffic
	if err := dst.Where("email = ?", "a@b.c").First(&ct).Error; err != nil {
		t.Fatalf("read back traffic: %v", err)
	}
	if ct.Up != 10 || ct.Down != 20 || !ct.Enable {
		t.Errorf("traffic mismatch: %+v", ct)
	}
}

// TestDumpAndRestoreSQLiteRoundTrip dumps a seeded SQLite db to .dump text and
// rebuilds it, asserting the row survives.
func TestDumpAndRestoreSQLiteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dumpPath := filepath.Join(dir, "out.dump")
	dstPath := filepath.Join(dir, "rebuilt.db")

	src, err := gorm.Open(sqlite.Open(srcPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	if err := src.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := src.Create(&model.Setting{Key: "secret", Value: "o'brien \"quote\""}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if sqlDB, _ := src.DB(); sqlDB != nil {
		sqlDB.Close()
	}

	if err := DumpSQLite(srcPath, dumpPath); err != nil {
		t.Fatalf("DumpSQLite: %v", err)
	}
	if fi, err := os.Stat(dumpPath); err != nil || fi.Size() == 0 {
		t.Fatalf("dump missing/empty: %v", err)
	}
	if err := RestoreSQLite(dumpPath, dstPath); err != nil {
		t.Fatalf("RestoreSQLite: %v", err)
	}

	dst, err := gorm.Open(sqlite.Open(dstPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	defer closeGorm(dst)
	var s model.Setting
	if err := dst.Where("key = ?", "secret").First(&s).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if s.Value != "o'brien \"quote\"" {
		t.Errorf("value mismatch after round-trip: %q", s.Value)
	}
}

// closeGorm closes the underlying *sql.DB so Windows can delete the temp file.
func closeGorm(db *gorm.DB) {
	if db == nil {
		return
	}
	if s, err := db.DB(); err == nil {
		s.Close()
	}
}
