package database

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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

func TestBatchIncrementTrafficSQLiteChunkSizes(t *testing.T) {
	if got := sqliteMaxVars / sqliteInboundTrafficVarsPerRow; got != 180 {
		t.Fatalf("sqlite inbound traffic chunk size = %d, want 180", got)
	}
	if got := sqliteMaxVars / sqliteClientTrafficVarsPerRow; got != 128 {
		t.Fatalf("sqlite client traffic chunk size = %d, want 128", got)
	}
	if got := postgresMaxVars / postgresInboundTrafficVarsRow; got != 20000 {
		t.Fatalf("postgres inbound traffic chunk size = %d, want 20000", got)
	}
	if got := postgresMaxVars / postgresClientTrafficVarsRow; got != 15000 {
		t.Fatalf("postgres client traffic chunk size = %d, want 15000", got)
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

func TestPostgresBatchTrafficAndIndexesOptIn(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres traffic/index test")
	}

	oldDB := db
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB(postgres): %v", err)
	}
	t.Cleanup(func() {
		_ = CloseDB()
		db = oldDB
	})

	suffix := fmt.Sprintf("pg-batch-%d", time.Now().UnixNano())
	tag := suffix + "-in"
	email := suffix + "@example.test"
	inbound := model.Inbound{UserId: 1, Tag: tag, Enable: true, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	t.Cleanup(func() { db.Where("tag = ?", tag).Delete(&model.Inbound{}) })
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: email, LastOnline: 10}).Error; err != nil {
		t.Fatalf("seed client traffic: %v", err)
	}
	t.Cleanup(func() { db.Where("email = ?", email).Delete(&xray.ClientTraffic{}) })

	if err := BatchIncrementInboundTraffic(db, []TrafficDelta{{Tag: tag, Up: 7, Down: 11}}); err != nil {
		t.Fatalf("increment inbound: %v", err)
	}
	if err := BatchIncrementClientTraffic(db, []ClientTrafficDelta{{Email: email, Up: 13, Down: 17, LastOnline: 9}}); err != nil {
		t.Fatalf("increment client: %v", err)
	}

	var gotInbound model.Inbound
	if err := db.Where("tag = ?", tag).First(&gotInbound).Error; err != nil {
		t.Fatalf("read inbound: %v", err)
	}
	if gotInbound.Up != 7 || gotInbound.Down != 11 {
		t.Fatalf("inbound traffic = %d/%d, want 7/11", gotInbound.Up, gotInbound.Down)
	}
	var gotTraffic xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&gotTraffic).Error; err != nil {
		t.Fatalf("read client traffic: %v", err)
	}
	if gotTraffic.Up != 13 || gotTraffic.Down != 17 || gotTraffic.LastOnline != 10 {
		t.Fatalf("client traffic = up:%d down:%d last:%d, want 13/17/10", gotTraffic.Up, gotTraffic.Down, gotTraffic.LastOnline)
	}

	for _, name := range []string{"idx_client_traffics_email_up_down", "idx_clients_uuid"} {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM pg_index i
				JOIN pg_class c ON c.oid = i.indexrelid
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE c.relname = ?
				  AND n.nspname = ANY (current_schemas(false))
				  AND i.indisvalid
			)`, name).Scan(&exists).Error; err != nil {
			t.Fatalf("check index %s: %v", name, err)
		}
		if !exists {
			t.Fatalf("missing valid postgres index %s", name)
		}
	}

	var hasPartialTgID bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = 'idx_clients_tg_id'
			  AND n.nspname = ANY (current_schemas(false))
			  AND i.indisvalid
			  AND i.indpred IS NOT NULL
		)`).Scan(&hasPartialTgID).Error; err != nil {
		t.Fatalf("check idx_clients_tg_id: %v", err)
	}
	if !hasPartialTgID {
		t.Fatal("missing valid partial postgres index idx_clients_tg_id")
	}
}
