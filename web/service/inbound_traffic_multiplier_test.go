package service

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

func TestAddTrafficAppliesInboundTrafficMultiplier(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	inbound := model.Inbound{
		UserId:            1,
		Tag:               "inbound-test",
		Protocol:          model.VLESS,
		Settings:          `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"alice@example.com","enable":true}],"decryption":"none"}`,
		TrafficMultiplier: 2,
		Enable:            true,
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     "alice@example.com",
		Enable:    true,
	}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	s := InboundService{}
	_, _, err := s.AddTraffic(
		[]*xray.Traffic{{
			IsInbound: true,
			Tag:       inbound.Tag,
			Up:        100,
			Down:      200,
		}},
		[]*xray.ClientTraffic{{
			Email: "alice@example.com",
			Up:    10,
			Down:  20,
		}},
	)
	if err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var storedInbound model.Inbound
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if storedInbound.Up != 200 || storedInbound.Down != 400 {
		t.Fatalf("inbound traffic = up %d/down %d, want up 200/down 400", storedInbound.Up, storedInbound.Down)
	}

	var storedClient xray.ClientTraffic
	if err := db.Where("email = ?", "alice@example.com").First(&storedClient).Error; err != nil {
		t.Fatalf("load client traffic: %v", err)
	}
	if storedClient.Up != 20 || storedClient.Down != 40 {
		t.Fatalf("client traffic = up %d/down %d, want up 20/down 40", storedClient.Up, storedClient.Down)
	}
}

func TestAddTrafficSaturatesMultipliedInboundCounters(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	inbound := model.Inbound{
		UserId:            1,
		Tag:               "inbound-overflow",
		Protocol:          model.VLESS,
		Settings:          `{"clients":[{"id":"22222222-2222-2222-2222-222222222222","email":"bob@example.com","enable":true}],"decryption":"none"}`,
		TrafficMultiplier: 2,
		Up:                math.MaxInt64 - 5,
		Down:              math.MaxInt64 - 5,
		Enable:            true,
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     "bob@example.com",
		Enable:    true,
		Up:        math.MaxInt64 - 5,
		Down:      math.MaxInt64 - 5,
	}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	s := InboundService{}
	_, _, err := s.AddTraffic(
		[]*xray.Traffic{{
			IsInbound: true,
			Tag:       inbound.Tag,
			Up:        10,
			Down:      10,
		}},
		[]*xray.ClientTraffic{{
			Email: "bob@example.com",
			Up:    10,
			Down:  10,
		}},
	)
	if err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var storedInbound model.Inbound
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if storedInbound.Up != math.MaxInt64 || storedInbound.Down != math.MaxInt64 {
		t.Fatalf("inbound traffic = up %d/down %d, want saturated MaxInt64", storedInbound.Up, storedInbound.Down)
	}

	var storedClient xray.ClientTraffic
	if err := db.Where("email = ?", "bob@example.com").First(&storedClient).Error; err != nil {
		t.Fatalf("load client traffic: %v", err)
	}
	if storedClient.Up != math.MaxInt64 || storedClient.Down != math.MaxInt64 {
		t.Fatalf("client traffic = up %d/down %d, want saturated MaxInt64", storedClient.Up, storedClient.Down)
	}
}
