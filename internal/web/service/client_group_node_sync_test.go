package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
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
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
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

// Removing the group in the client editor and saving must clear group_name and
// drop the settings "group" key, even though SyncInbound preserves a group on a
// group-less rebuild. The editor round-trips the field, so ClientService.Update
// applies it explicitly.
func TestClientUpdate_ClearsGroup(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const email = "grp-clear@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c005"
	const wantGroup = "vip"

	ib := &model.Inbound{
		UserId:   1,
		Tag:      "vless-clear",
		Enable:   true,
		Port:     20003,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"email":"` + email + `","id":"` + uid + `","enable":true,"group":"` + wantGroup + `"}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := ClientService{}
	inboundSvc := &InboundService{}

	// Seed the client record + inbound link from the settings.
	seedClients, err := inboundSvc.GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if err := svc.SyncInbound(nil, ib.Id, seedClients); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}

	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("lookup seeded record: %v", err)
	}
	if rec.Group != wantGroup {
		t.Fatalf("setup: group not seeded, got %q", rec.Group)
	}

	// Edit the client and remove the group.
	updated := *rec.ToClient()
	updated.Group = ""
	if _, err := svc.Update(inboundSvc, rec.Id, updated); err != nil {
		t.Fatalf("Update (clear group): %v", err)
	}

	var after model.ClientRecord
	if err := db.Where("email = ?", email).First(&after).Error; err != nil {
		t.Fatalf("lookup record after update: %v", err)
	}
	if after.Group != "" {
		t.Errorf("group not cleared after editor removed it: got %q, want empty", after.Group)
	}

	var ibAfter model.Inbound
	if err := db.First(&ibAfter, ib.Id).Error; err != nil {
		t.Fatalf("lookup inbound after update: %v", err)
	}
	if strings.Contains(ibAfter.Settings, `"group"`) {
		t.Errorf("inbound settings still carry a group key after removal: %s", ibAfter.Settings)
	}
}
