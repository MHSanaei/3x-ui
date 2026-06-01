package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
)

func TestSetRemoteTraffic_PreservesPanelLocalGroupAndComment(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const nodeID = 1
	const email = "node-user@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c003"
	const wantGroup = "vip"
	const wantComment = "renewed manually"

	id := nodeID
	central := &model.Inbound{
		UserId:   1,
		NodeID:   &id,
		Tag:      "n1-vless",
		Enable:   true,
		Port:     20001,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"email":"` + email + `","id":"` + uid + `","enable":true,"group":"` + wantGroup + `","comment":"` + wantComment + `"}]}`,
	}
	if err := db.Create(central).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	if err := db.Create(&model.ClientRecord{
		Email:   email,
		UUID:    uid,
		Enable:  true,
		Group:   wantGroup,
		Comment: wantComment,
	}).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{
			{
				Tag:      "n1-vless",
				Enable:   true,
				Port:     20001,
				Protocol: model.VLESS,
				Settings: `{"clients":[{"email":"` + email + `","id":"` + uid + `","enable":true}]}`,
			},
		},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup client row after sync: %v", err)
	}
	if row.Group != wantGroup {
		t.Errorf("group was wiped by node snapshot sync: got %q, want %q", row.Group, wantGroup)
	}
	if row.Comment != wantComment {
		t.Errorf("comment was wiped by node snapshot sync: got %q, want %q", row.Comment, wantComment)
	}
}

func TestSyncInbound_KeepsGroupWhenIncomingEmpty(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	ib := &model.Inbound{Tag: "vless-grp", Enable: true, Port: 20002, Protocol: model.VLESS}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := ClientService{}
	const email = "grp-user@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c004"
	const wantGroup = "vip"

	withGroup := model.Client{Email: email, ID: uid, Enable: true, Group: wantGroup}
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{withGroup}); err != nil {
		t.Fatalf("SyncInbound (set group): %v", err)
	}

	noGroup := model.Client{Email: email, ID: uid, Enable: true, Group: ""}
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{noGroup}); err != nil {
		t.Fatalf("SyncInbound (group-less rebuild): %v", err)
	}

	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup client row: %v", err)
	}
	if row.Group != wantGroup {
		t.Errorf("group must survive a group-less settings rebuild (it is managed via the Groups page, not Xray settings): got %q, want %q", row.Group, wantGroup)
	}
}
