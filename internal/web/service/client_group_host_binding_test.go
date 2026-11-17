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

func TestClientGroup_SetHostGroups(t *testing.T) {
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
	hostSvc := &HostService{}
	if _, err := hostSvc.AddHostGroup(&entity.HostGroup{
		GroupId: "hg", InboundIds: []int{inbound.Id}, Hosts: []string{"a.cdn.com"}, Remark: "A",
	}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}

	svc := &ClientService{}
	ids, err := svc.SetGroupHostGroups("VIP", []string{"hg", " hg ", ""})
	if err != nil {
		t.Fatalf("SetGroupHostGroups: %v", err)
	}
	if len(ids) != 1 || ids[0] != "hg" {
		t.Fatalf("saved host groups = %v, want [hg]", ids)
	}
	groups, err := svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if got := findGroupSummary(groups, "VIP"); len(got.HostGroupIds) != 1 || got.HostGroupIds[0] != "hg" {
		t.Fatalf("listed host groups = %v, want [hg]", got.HostGroupIds)
	}

	// Clearing the selection removes the binding but keeps the group.
	ids, err = svc.SetGroupHostGroups("VIP", nil)
	if err != nil {
		t.Fatalf("SetGroupHostGroups clear: %v", err)
	}
	if ids == nil || len(ids) != 0 {
		t.Fatalf("cleared host groups = %v, want a non-nil empty slice", ids)
	}
	groups, err = svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups after clear: %v", err)
	}
	if got := findGroupSummary(groups, "VIP"); got.HostGroupIds == nil || len(got.HostGroupIds) != 0 {
		t.Fatalf("listed host groups after clear = %v, want a non-nil empty slice", got.HostGroupIds)
	}

	// Deleting the host group also drops the binding.
	if _, err := svc.SetGroupHostGroups("VIP", []string{"hg"}); err != nil {
		t.Fatalf("SetGroupHostGroups rebind: %v", err)
	}
	if err := hostSvc.DeleteHostGroup("hg"); err != nil {
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

func TestClientGroup_RejectsUnknownHostGroup(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	group := &model.ClientGroup{Name: "VIP"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	svc := &ClientService{}
	if _, err := svc.SetGroupHostGroups("VIP", []string{"missing"}); err == nil {
		t.Fatal("unknown host group must be rejected")
	}
	var bindings int64
	if err := db.Model(&model.HostGroupClientGroup{}).Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if bindings != 0 {
		t.Fatalf("failed save left %d bindings", bindings)
	}
	// A failed save must not create the client group as a side effect.
	if _, err := svc.SetGroupHostGroups("New", []string{"missing"}); err == nil {
		t.Fatal("unknown host group must be rejected for an unknown group too")
	}
	var groups int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "New").Count(&groups).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 0 {
		t.Fatalf("failed save created %d group rows", groups)
	}
}

func TestClientGroup_CreateAndUpdateWithHostGroups(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	inbound := &model.Inbound{UserId: 1, Tag: "in", Enable: true, Port: 4503, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if _, err := (&HostService{}).AddHostGroup(&entity.HostGroup{
		GroupId: "hg", InboundIds: []int{inbound.Id}, Hosts: []string{"a.cdn.com"}, Remark: "A",
	}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}

	svc := &ClientService{}
	if err := svc.CreateGroup("VIP", []string{"hg"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	groups, err := svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if got := findGroupSummary(groups, "VIP"); len(got.HostGroupIds) != 1 || got.HostGroupIds[0] != "hg" {
		t.Fatalf("created group host rules = %v, want [hg]", got.HostGroupIds)
	}

	// Renaming and clearing host rules in one edit.
	if _, err := svc.UpdateGroup("VIP", "PRO", []string{}); err != nil {
		t.Fatalf("UpdateGroup rename+clear: %v", err)
	}
	groups, err = svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups after update: %v", err)
	}
	if got := findGroupSummary(groups, "PRO"); got.Name != "PRO" || len(got.HostGroupIds) != 0 {
		t.Fatalf("updated group = %+v, want PRO with no host rules", got)
	}

	// Editing host rules without changing the name.
	if _, err := svc.UpdateGroup("PRO", "PRO", []string{"hg"}); err != nil {
		t.Fatalf("UpdateGroup host rules only: %v", err)
	}
	groups, err = svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups after host-only update: %v", err)
	}
	if got := findGroupSummary(groups, "PRO"); len(got.HostGroupIds) != 1 || got.HostGroupIds[0] != "hg" {
		t.Fatalf("host-only update host rules = %v, want [hg]", got.HostGroupIds)
	}
}

func TestClientGroup_CreateAndUpdateRejectUnknownHostGroup(t *testing.T) {
	initTrafficTestDB(t)
	db := database.GetDB()
	svc := &ClientService{}
	if err := svc.CreateGroup("VIP", []string{"missing"}); err == nil {
		t.Fatal("create with unknown host group must be rejected")
	}
	var groups int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "VIP").Count(&groups).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 0 {
		t.Fatalf("failed create left %d group rows", groups)
	}

	if err := svc.CreateGroup("VIP", nil); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if _, err := svc.UpdateGroup("VIP", "PRO", []string{"missing"}); err == nil {
		t.Fatal("update with unknown host group must be rejected")
	}
	var renamed, bindings int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "PRO").Count(&renamed).Error; err != nil {
		t.Fatalf("count renamed groups: %v", err)
	}
	if err := db.Model(&model.HostGroupClientGroup{}).Count(&bindings).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if renamed != 0 || bindings != 0 {
		t.Fatalf("failed update left renamed=%d bindings=%d", renamed, bindings)
	}
}

func findGroupSummary(groups []GroupSummary, name string) GroupSummary {
	for _, group := range groups {
		if group.Name == name {
			return group
		}
	}
	return GroupSummary{}
}
