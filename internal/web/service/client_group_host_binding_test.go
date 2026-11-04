package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestClientGroup_RenameKeepsMembershipAndBindings(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	group := &model.ClientGroup{Name: "VIP"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "a@x", Group: "VIP", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.HostGroupClientGroup{HostGroupId: "hg", ClientGroupId: group.Id}).Error; err != nil {
		t.Fatalf("bind host group: %v", err)
	}

	svc := &ClientService{}
	affected, err := svc.RenameGroup("VIP", "PRO")
	if err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	var client model.ClientRecord
	if err := db.Where("email = ?", "a@x").First(&client).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if client.Group != "PRO" {
		t.Fatalf("client group = %q, want PRO", client.Group)
	}
	var bindings int64
	if err := db.Model(&model.HostGroupClientGroup{}).
		Where("host_group_id = ? AND client_group_id = ?", "hg", group.Id).Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if bindings != 1 {
		t.Fatalf("rename dropped the host binding: %d", bindings)
	}
	var renamed model.ClientGroup
	if err := db.First(&renamed, group.Id).Error; err != nil || renamed.Name != "PRO" {
		t.Fatalf("group row = %+v err=%v, want PRO", renamed, err)
	}
}

func TestClientGroup_DeleteClearsMembersAndBindings(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	group := &model.ClientGroup{Name: "VIP"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "a@x", Group: "VIP", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.HostGroupClientGroup{HostGroupId: "hg", ClientGroupId: group.Id}).Error; err != nil {
		t.Fatalf("bind host group: %v", err)
	}

	svc := &ClientService{}
	affected, err := svc.DeleteGroup("VIP")
	if err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	var client model.ClientRecord
	if err := db.Where("email = ?", "a@x").First(&client).Error; err != nil {
		t.Fatalf("client must not be deleted: %v", err)
	}
	if client.Group != "" {
		t.Fatalf("client group = %q, want empty", client.Group)
	}
	var bindings, groups int64
	if err := db.Model(&model.HostGroupClientGroup{}).Where("client_group_id = ?", group.Id).Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if bindings != 0 {
		t.Fatalf("delete left %d host bindings", bindings)
	}
	if err := db.Model(&model.ClientGroup{}).Where("id = ?", group.Id).Count(&groups).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 0 {
		t.Fatalf("delete left the group row: %d", groups)
	}
}

func TestClientGroup_AddToGroupCreatesRowForDerivedLabel(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientRecord{Email: "a@x", Group: "Derived", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	svc := &ClientService{}
	if _, err := svc.AddToGroup([]string{"a@x"}, "Derived"); err != nil {
		t.Fatalf("AddToGroup: %v", err)
	}
	var groups int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "Derived").Count(&groups).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 1 {
		t.Fatalf("derived group row = %d, want 1", groups)
	}
}

func TestClientGroup_AddToGroupUsesCanonicalCase(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientGroup{Name: "VIP"}).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "a@x", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	svc := &ClientService{}
	if _, err := svc.AddToGroup([]string{"a@x"}, "vip"); err != nil {
		t.Fatalf("AddToGroup: %v", err)
	}
	var client model.ClientRecord
	if err := db.Where("email = ?", "a@x").First(&client).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if client.Group != "VIP" {
		t.Fatalf("client group = %q, want canonical VIP", client.Group)
	}
	var groups int64
	if err := db.Model(&model.ClientGroup{}).Where("LOWER(name) = 'vip'").Count(&groups).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 1 {
		t.Fatalf("case variant created a second group: %d", groups)
	}
}

func TestHostGroup_ClientGroupBindings(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	group := &model.ClientGroup{Name: "VIP"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	inbound := &model.Inbound{UserId: 1, Tag: "in", Enable: true, Port: 4501, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := &HostService{}
	req := &entity.HostGroup{
		GroupId: "hg", InboundIds: []int{inbound.Id}, Hosts: []string{"a.cdn.com"},
		Remark: "A", ClientGroupIds: []int{group.Id},
	}
	if _, err := svc.AddHostGroup(req); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	got, err := svc.GetHostGroup("hg")
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	if len(got.ClientGroupIds) != 1 || got.ClientGroupIds[0] != group.Id {
		t.Fatalf("bindings = %v, want [%d]", got.ClientGroupIds, group.Id)
	}

	// A row added later to the host group inherits the binding.
	if err := db.Create(&model.Host{GroupId: "hg", InboundId: inbound.Id, Remark: "A2", Address: "a2.cdn.com"}).Error; err != nil {
		t.Fatalf("add host row: %v", err)
	}
	got, err = svc.GetHostGroup("hg")
	if err != nil {
		t.Fatalf("GetHostGroup after add: %v", err)
	}
	if len(got.Hosts) != 2 || len(got.ClientGroupIds) != 1 {
		t.Fatalf("later row must inherit the binding: hosts=%v bindings=%v", got.Hosts, got.ClientGroupIds)
	}

	// Clearing the request bindings removes them.
	req.ClientGroupIds = nil
	if _, err := svc.UpdateHostGroup("hg", req); err != nil {
		t.Fatalf("UpdateHostGroup clear: %v", err)
	}
	if got, _ = svc.GetHostGroup("hg"); len(got.ClientGroupIds) != 0 {
		t.Fatalf("bindings after clear = %v, want none", got.ClientGroupIds)
	}

	req.ClientGroupIds = []int{group.Id}
	if _, err := svc.UpdateHostGroup("hg", req); err != nil {
		t.Fatalf("UpdateHostGroup bind: %v", err)
	}
	if err := svc.DeleteHostGroup("hg"); err != nil {
		t.Fatalf("DeleteHostGroup: %v", err)
	}
	var bindings int64
	if err := db.Model(&model.HostGroupClientGroup{}).Where("host_group_id = ?", "hg").Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if bindings != 0 {
		t.Fatalf("delete left %d bindings", bindings)
	}
}

func TestHostGroup_RejectsUnknownClientGroup(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	inbound := &model.Inbound{UserId: 1, Tag: "in", Enable: true, Port: 4502, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	svc := &HostService{}
	_, err := svc.AddHostGroup(&entity.HostGroup{
		GroupId: "hg", InboundIds: []int{inbound.Id}, Hosts: []string{"a.cdn.com"},
		Remark: "A", ClientGroupIds: []int{9999},
	})
	if err == nil {
		t.Fatal("unknown client group must be rejected")
	}
	var hosts, bindings int64
	if err := db.Model(&model.Host{}).Where("group_id = ?", "hg").Count(&hosts).Error; err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	if err := db.Model(&model.HostGroupClientGroup{}).Where("host_group_id = ?", "hg").Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if hosts != 0 || bindings != 0 {
		t.Fatalf("failed add left hosts=%d bindings=%d", hosts, bindings)
	}
}
