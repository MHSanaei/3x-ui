package database

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A database written before inbound_port_reservations existed must still migrate:
// the table is derived, so the destination rebuilds it empty instead of failing.
func TestCopyAllModelsSkipsTablesAbsentFromSource(t *testing.T) {
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open(t.TempDir()+"/"+name), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		return db
	}

	src := open("old.db")
	skipped := reflect.TypeOf(&model.InboundPortReservation{})
	for _, m := range migrationModels() {
		if reflect.TypeOf(m) == skipped {
			continue
		}
		if err := src.AutoMigrate(m); err != nil {
			t.Fatalf("automigrate source %T: %v", m, err)
		}
	}
	if src.Migrator().HasTable(&model.InboundPortReservation{}) {
		t.Fatal("source fixture must not carry inbound_port_reservations")
	}
	if err := src.Create(&model.Inbound{UserId: 1, Tag: "in-1", Port: 443, Protocol: model.VLESS}).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	dst := open("new.db")
	if err := copyAllModels(src, dst); err != nil {
		t.Fatalf("copyAllModels on a source without inbound_port_reservations: %v", err)
	}

	if !dst.Migrator().HasTable(&model.InboundPortReservation{}) {
		t.Fatal("destination must still carry the rebuilt reservation table")
	}
	var reservations int64
	if err := dst.Model(&model.InboundPortReservation{}).Count(&reservations).Error; err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if reservations != 0 {
		t.Fatalf("rebuilt reservation table = %d rows, want 0", reservations)
	}
	var inbounds int64
	if err := dst.Model(&model.Inbound{}).Count(&inbounds).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if inbounds != 1 {
		t.Fatalf("copied inbounds = %d, want 1", inbounds)
	}
}
