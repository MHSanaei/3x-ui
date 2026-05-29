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
	hysteriaInbound := &model.Inbound{Tag: "hy-in", Enable: true, Port: 10002, Protocol: model.Hysteria}
	if err := db.Create(hysteriaInbound).Error; err != nil {
		t.Fatalf("create hysteria inbound: %v", err)
	}

	svc := ClientService{}
	const sharedEmail = "shared@example.com"
	const wantUUID = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c001"
	const wantAuth = "h2-auth-token"
	const wantFlow = "xtls-rprx-vision"

	vlessClient := model.Client{Email: sharedEmail, ID: wantUUID, Enable: true, Flow: wantFlow}
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

	vlessList, err := svc.ListForInbound(nil, vlessInbound.Id)
	if err != nil {
		t.Fatalf("ListForInbound(vless): %v", err)
	}
	if len(vlessList) != 1 || vlessList[0].Flow != wantFlow {
		t.Errorf("VLESS inbound should still report flow=%q via FlowOverride, got %#v", wantFlow, vlessList)
	}

	hysteriaList, err := svc.ListForInbound(nil, hysteriaInbound.Id)
	if err != nil {
		t.Fatalf("ListForInbound(hysteria): %v", err)
	}
	if len(hysteriaList) != 1 || hysteriaList[0].Flow != "" {
		t.Errorf("Hysteria inbound should report empty flow, got %#v", hysteriaList)
	}
}

func TestSyncInbound_AllowsClearingFlow(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	vless := &model.Inbound{Tag: "vless-in", Enable: true, Port: 10003, Protocol: model.VLESS}
	if err := db.Create(vless).Error; err != nil {
		t.Fatalf("create vless inbound: %v", err)
	}

	svc := ClientService{}
	const email = "alice@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c002"

	withFlow := model.Client{Email: email, ID: uid, Enable: true, Flow: "xtls-rprx-vision"}
	if err := svc.SyncInbound(nil, vless.Id, []model.Client{withFlow}); err != nil {
		t.Fatalf("vless SyncInbound (set flow): %v", err)
	}

	cleared := model.Client{Email: email, ID: uid, Enable: true, Flow: ""}
	if err := svc.SyncInbound(nil, vless.Id, []model.Client{cleared}); err != nil {
		t.Fatalf("vless SyncInbound (clear flow): %v", err)
	}

	list, err := svc.ListForInbound(nil, vless.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 client, got %d", len(list))
	}
	if list[0].Flow != "" {
		t.Errorf("flow should be clearable on the owning inbound, got %q (Copilot review on #4545)", list[0].Flow)
	}
}
