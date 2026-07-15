package service

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAddTrafficCommitsDespiteDisableHelperError(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	normal := &model.Inbound{UserId: 1, Tag: "in-normal", Enable: true, Port: 43001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(normal).Error; err != nil {
		t.Fatalf("seed normal inbound: %v", err)
	}
	expired := &model.Inbound{UserId: 1, Tag: "in-expired", Enable: true, Port: 43002, Protocol: model.VLESS, ExpiryTime: 1, Settings: `{"clients":[]}`}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("seed expired inbound: %v", err)
	}

	const cbName = "b2-03:fail-disable"
	if err := db.Callback().Update().After("gorm:update").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "inbounds" &&
			strings.Contains(tx.Statement.SQL.String(), "expiry_time") {
			tx.AddError(errors.New("injected disableInvalidInbounds failure"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(cbName) })

	if _, _, err := svc.AddTraffic([]*xray.Traffic{{IsInbound: true, Tag: "in-normal", Up: 500, Down: 700}}, nil); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var reloaded model.Inbound
	if err := db.Where("tag = ?", "in-normal").First(&reloaded).Error; err != nil {
		t.Fatalf("reload normal inbound: %v", err)
	}
	if reloaded.Up != 500 || reloaded.Down != 700 {
		t.Fatalf("traffic tick was rolled back by a best-effort disable-helper error: up=%d down=%d, want 500/700", reloaded.Up, reloaded.Down)
	}
}

func TestResetAllTrafficsReenablesDepletedClients(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &ClientService{}

	if err := db.Create(&xray.ClientTraffic{InboundId: 1, Email: "spent@x", Enable: false, Up: 60, Down: 60, Total: 100}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := svc.ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}

	row := readTraffic(t, db, "spent@x")
	if row.Up != 0 || row.Down != 0 {
		t.Fatalf("usage not reset: up=%d down=%d", row.Up, row.Down)
	}
	if !row.Enable {
		t.Fatal("a depleted client must be re-enabled after Reset All Client Traffic, matching every other reset path")
	}
}

func TestResetAllTrafficsClearsNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &ClientService{}

	if err := db.Create(&xray.ClientTraffic{InboundId: 1, Email: "spent@x", Enable: true, Up: 60, Down: 60, Total: 100}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: "spent@x", Up: 60, Down: 60}).Error; err != nil {
		t.Fatalf("seed node baseline: %v", err)
	}

	if _, err := svc.ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}

	var cnt int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", "spent@x").Count(&cnt).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("Reset All Client Traffic must clear node baselines like its sibling reset paths, found %d", cnt)
	}
}
