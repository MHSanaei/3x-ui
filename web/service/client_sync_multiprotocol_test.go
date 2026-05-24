package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestSyncInbound_PreservesCredentialsAcrossProtocols(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	vlessInbound := &model.Inbound{Tag: "vless-in", Enable: true, Port: 10001, Protocol: model.VLESS}
	if err := db.Create(vlessInbound).Error; err != nil {
		t.Fatalf("create vless inbound: %v", err)
	}
	hysteriaInbound := &model.Inbound{Tag: "hy-in", Enable: true, Port: 10002, Protocol: model.Hysteria2}
	if err := db.Create(hysteriaInbound).Error; err != nil {
		t.Fatalf("create hysteria inbound: %v", err)
	}

	svc := ClientService{}
	const sharedEmail = "shared@example.com"
	const wantUUID = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c001"
	const wantAuth = "h2-auth-token"

	vlessClient := model.Client{Email: sharedEmail, ID: wantUUID, Enable: true, Flow: "xtls-rprx-vision"}
	if err := svc.SyncInbound(nil, vlessInbound.Id, []model.Client{vlessClient}); err != nil {
		t.Fatalf("vless SyncInbound: %v", err)
	}

	hysteriaClient := model.Client{Email: sharedEmail, Auth: wantAuth, Enable: true}
	if err := svc.SyncInbound(nil, hysteriaInbound.Id, []model.Client{hysteriaClient}); err != nil {
		t.Fatalf("hysteria SyncInbound: %v", err)
	}

	var row model.ClientRecord
	if err := db.Where("email = ?", sharedEmail).First(&row).Error; err != nil {
		t.Fatalf("lookup client row: %v", err)
	}
	if row.UUID != wantUUID {
		t.Errorf("UUID was clobbered by Hysteria sync: got %q, want %q", row.UUID, wantUUID)
	}
	if row.Auth != wantAuth {
		t.Errorf("Auth not persisted: got %q, want %q", row.Auth, wantAuth)
	}
	if row.Flow != "xtls-rprx-vision" {
		t.Errorf("Flow was clobbered by Hysteria sync: got %q, want xtls-rprx-vision", row.Flow)
	}
}
