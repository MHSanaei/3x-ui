package database

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBatchIncrementTrafficSQLite(t *testing.T) {
	oldDB := db
	t.Cleanup(func() { db = oldDB })

	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Inbound{}, &xray.ClientTraffic{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Create(&model.Inbound{Tag: "in-a"}).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "a@example.test", LastOnline: 5}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	if err := BatchIncrementInboundTraffic(db, []TrafficDelta{{Tag: "in-a", Up: 7, Down: 11}}); err != nil {
		t.Fatalf("increment inbound: %v", err)
	}
	if err := BatchIncrementClientTraffic(db, []ClientTrafficDelta{{Email: "a@example.test", Up: 13, Down: 17, LastOnline: 3}}); err != nil {
		t.Fatalf("increment client: %v", err)
	}

	var inbound model.Inbound
	if err := db.Where("tag = ?", "in-a").First(&inbound).Error; err != nil {
		t.Fatalf("read inbound: %v", err)
	}
	if inbound.Up != 7 || inbound.Down != 11 {
		t.Fatalf("inbound traffic = %d/%d, want 7/11", inbound.Up, inbound.Down)
	}

	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", "a@example.test").First(&traffic).Error; err != nil {
		t.Fatalf("read traffic: %v", err)
	}
	if traffic.Up != 13 || traffic.Down != 17 || traffic.LastOnline != 5 {
		t.Fatalf("client traffic = up:%d down:%d last:%d, want 13/17/5", traffic.Up, traffic.Down, traffic.LastOnline)
	}
}

func TestCastBigintDialect(t *testing.T) {
	oldDB := db
	db = nil
	t.Cleanup(func() { db = oldDB })

	t.Setenv("XUI_DB_TYPE", "sqlite")
	if got := CastBigint("?"); got != "?" {
		t.Fatalf("sqlite CastBigint = %q, want ?", got)
	}

	t.Setenv("XUI_DB_TYPE", "postgres")
	if got := CastBigint("?"); got != "CAST(? AS BIGINT)" {
		t.Fatalf("postgres CastBigint = %q, want CAST(? AS BIGINT)", got)
	}
}
