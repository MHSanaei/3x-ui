package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestEnsureClientGroupRows_CreatesMissingAndCanonicalizes(t *testing.T) {
	initMigrateDB(t)
	db := GetDB()
	if err := db.Create(&model.ClientRecord{Email: "a@x", Group: "VIP", Enable: true}).Error; err != nil {
		t.Fatalf("create client a: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "b@x", Group: "vip", Enable: true}).Error; err != nil {
		t.Fatalf("create client b: %v", err)
	}

	if err := EnsureClientGroupRows(); err != nil {
		t.Fatalf("EnsureClientGroupRows: %v", err)
	}
	var groups []model.ClientGroup
	if err := db.Order("id ASC").Find(&groups).Error; err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "VIP" {
		t.Fatalf("groups = %+v, want one canonical VIP", groups)
	}
	var clients []model.ClientRecord
	if err := db.Order("email ASC").Find(&clients).Error; err != nil {
		t.Fatalf("load clients: %v", err)
	}
	for _, client := range clients {
		if client.Group != "VIP" {
			t.Fatalf("client %s group = %q, want VIP", client.Email, client.Group)
		}
	}

	if err := EnsureClientGroupRows(); err != nil {
		t.Fatalf("second EnsureClientGroupRows: %v", err)
	}
	var count int64
	if err := db.Model(&model.ClientGroup{}).Count(&count).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if count != 1 {
		t.Fatalf("second run created a duplicate: %d", count)
	}
}

func TestEnsureClientGroupRows_UpgradeExistingDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if err := GetDB().Create(&model.ClientRecord{Email: "legacy@x", Group: "Legacy", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB: %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("reopen InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var group model.ClientGroup
	if err := GetDB().Where("name = ?", "Legacy").First(&group).Error; err != nil {
		t.Fatalf("reopen did not create the missing group row: %v", err)
	}
}

func TestPruneHostGroupClientGroups(t *testing.T) {
	initMigrateDB(t)
	db := GetDB()
	inbound := seedInboundWithStream(t, "prune", 4599, `{"network":"tcp","security":"none"}`)
	if err := db.Create(&model.Host{GroupId: "live", InboundId: inbound.Id, Remark: "L", Address: "live.example.com"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	group := &model.ClientGroup{Name: "VIP"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, hostGroup := range []string{"live", "gone"} {
		if err := db.Create(&model.HostGroupClientGroup{HostGroupId: hostGroup, ClientGroupId: group.Id}).Error; err != nil {
			t.Fatalf("create binding %s: %v", hostGroup, err)
		}
	}
	if err := PruneHostGroupClientGroups(); err != nil {
		t.Fatalf("PruneHostGroupClientGroups: %v", err)
	}
	var ids []string
	if err := db.Model(&model.HostGroupClientGroup{}).Pluck("host_group_id", &ids).Error; err != nil {
		t.Fatalf("load bindings: %v", err)
	}
	if len(ids) != 1 || ids[0] != "live" {
		t.Fatalf("bindings = %v, want only live", ids)
	}
}

func TestEnsureClientGroupRows_Postgres(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres test")
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	GetDB().Exec("TRUNCATE TABLE clients, client_groups RESTART IDENTITY CASCADE")
	if err := GetDB().Create(&model.ClientRecord{Email: "pg@x", Group: "PgGroup", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := EnsureClientGroupRows(); err != nil {
		t.Fatalf("EnsureClientGroupRows: %v", err)
	}
	var group model.ClientGroup
	if err := GetDB().Where("name = ?", "PgGroup").First(&group).Error; err != nil {
		t.Fatalf("group not created: %v", err)
	}
}
