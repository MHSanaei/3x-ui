package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

func setupTgBotTrafficTestDB(t *testing.T) {
	t.Helper()

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func TestGetClientTrafficTgBotUsesNormalizedRemoteNodeClients(t *testing.T) {
	setupTgBotTrafficTestDB(t)

	db := database.GetDB()
	nodeID := 7
	inbound := &model.Inbound{
		NodeID:   &nodeID,
		Tag:      "node-vless",
		Enable:   true,
		Port:     10001,
		Protocol: model.VLESS,
		Settings: `{"clients":[]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	const tgID int64 = 505739390
	const email = "remote-user@example.com"
	const uuid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c010"
	const subID = "remote-sub-id"

	clientSvc := ClientService{}
	if err := clientSvc.SyncInbound(nil, inbound.Id, []model.Client{{
		Email:  email,
		ID:     uuid,
		SubID:  subID,
		Enable: true,
		TgID:   tgID,
	}}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     email,
		Enable:    true,
		Total:     1024,
	}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}

	traffics, err := (&InboundService{}).GetClientTrafficTgBot(tgID)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot: %v", err)
	}
	if len(traffics) != 1 {
		t.Fatalf("expected one traffic row, got %d", len(traffics))
	}
	if traffics[0].Email != email || traffics[0].UUID != uuid || traffics[0].SubId != subID {
		t.Fatalf("unexpected traffic: %#v", traffics[0])
	}
}

func TestGetClientTrafficTgBotFallsBackToCompactSettingsJSON(t *testing.T) {
	setupTgBotTrafficTestDB(t)

	db := database.GetDB()
	const tgID int64 = 505739390
	const email = "legacy-user@example.com"
	const uuid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c011"
	const subID = "legacy-sub-id"

	inbound := &model.Inbound{
		Tag:      "legacy-vless",
		Enable:   true,
		Port:     10002,
		Protocol: model.VLESS,
		Settings: fmt.Sprintf(`{"clients":[{"email":%q,"id":%q,"subId":%q,"enable":true,"tgId":%d}]}`, email, uuid, subID, tgID),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     email,
		Enable:    true,
	}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}

	traffics, err := (&InboundService{}).GetClientTrafficTgBot(tgID)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot: %v", err)
	}
	if len(traffics) != 1 {
		t.Fatalf("expected one traffic row, got %d", len(traffics))
	}
	if traffics[0].Email != email || traffics[0].UUID != uuid || traffics[0].SubId != subID {
		t.Fatalf("unexpected traffic: %#v", traffics[0])
	}
}
