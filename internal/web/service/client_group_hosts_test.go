package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

func seedClientGroupHostAssignmentDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
}

func TestClientGroupHostAssignment_SetValidateAndClear(t *testing.T) {
	seedClientGroupHostAssignmentDB(t)
	db := database.GetDB()
	invalidations := 0
	releaseInvalidator := RegisterSubDataInvalidator(func() { invalidations++ })
	t.Cleanup(releaseInvalidator)
	if err := db.Create(&model.ClientGroup{Name: "staff"}).Error; err != nil {
		t.Fatalf("create client group: %v", err)
	}
	if err := db.Create(&model.Host{InboundId: 1, GroupId: "cdn-a", Remark: "A"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}

	svc := &ClientGroupHostAssignmentService{}
	if err := svc.SetGroupIDs("staff", []string{" cdn-a ", "cdn-a", ""}); err != nil {
		t.Fatalf("set valid assignment: %v", err)
	}
	if invalidations != 1 {
		t.Fatalf("successful assignment invalidations = %d, want 1", invalidations)
	}
	ids, err := svc.GetGroupIDs("staff")
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cdn-a" {
		t.Fatalf("normalized assignment = %#v, want [cdn-a]", ids)
	}

	if err := svc.SetGroupIDs("staff", []string{"missing"}); err == nil {
		t.Fatal("unknown HostGroup ID should be rejected")
	}
	if invalidations != 1 {
		t.Fatalf("rejected assignment invalidations = %d, want unchanged at 1", invalidations)
	}
	ids, err = svc.GetGroupIDs("staff")
	if err != nil {
		t.Fatalf("get assignment after rejected replacement: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cdn-a" {
		t.Fatalf("failed replacement changed existing assignment: %#v", ids)
	}

	if err := svc.SetGroupIDs("staff", []string{}); err != nil {
		t.Fatalf("clear assignment: %v", err)
	}
	if invalidations != 2 {
		t.Fatalf("assignment-clear invalidations = %d, want 2", invalidations)
	}
	ids, err = svc.GetGroupIDs("staff")
	if err != nil {
		t.Fatalf("get cleared assignment: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("cleared assignment = %#v, want empty", ids)
	}
}

func TestClientGroupHostAssignment_FollowsGroupRenameAndDelete(t *testing.T) {
	seedClientGroupHostAssignmentDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientGroup{Name: "staff"}).Error; err != nil {
		t.Fatalf("create client group: %v", err)
	}
	if err := db.Create(&model.Host{InboundId: 1, GroupId: "cdn-a", Remark: "A"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	svc := &ClientGroupHostAssignmentService{}
	if err := svc.SetGroupIDs("staff", []string{"cdn-a"}); err != nil {
		t.Fatalf("set assignment: %v", err)
	}

	clients := &ClientService{}
	if _, err := clients.RenameGroup("staff", "employees"); err != nil {
		t.Fatalf("rename group: %v", err)
	}
	ids, err := svc.GetGroupIDs("employees")
	if err != nil {
		t.Fatalf("get assignment after rename: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cdn-a" {
		t.Fatalf("assignment after rename = %#v, want [cdn-a]", ids)
	}

	if _, err := clients.DeleteGroup("employees"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	ids, err = svc.GetGroupIDs("employees")
	if err != nil {
		t.Fatalf("get assignment after delete: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("assignment remains after deleting group: %#v", ids)
	}
}

func TestClientGroupHostAssignment_RenameCollisionRollsBack(t *testing.T) {
	seedClientGroupHostAssignmentDB(t)
	db := database.GetDB()
	for _, name := range []string{"staff", "support"} {
		if err := db.Create(&model.ClientGroup{Name: name}).Error; err != nil {
			t.Fatalf("create client group %q: %v", name, err)
		}
	}
	for _, id := range []string{"cdn-a", "cdn-b"} {
		if err := db.Create(&model.Host{InboundId: 1, GroupId: id, Remark: id}).Error; err != nil {
			t.Fatalf("create host group %q: %v", id, err)
		}
	}

	assignments := &ClientGroupHostAssignmentService{}
	if err := assignments.SetGroupIDs("staff", []string{"cdn-a"}); err != nil {
		t.Fatalf("assign staff hosts: %v", err)
	}
	if err := assignments.SetGroupIDs("support", []string{"cdn-b"}); err != nil {
		t.Fatalf("assign support hosts: %v", err)
	}

	if _, err := (&ClientService{}).RenameGroup("staff", "support"); err == nil {
		t.Fatal("renaming into an existing group should fail")
	}

	for group, want := range map[string]string{"staff": "cdn-a", "support": "cdn-b"} {
		got, err := assignments.GetGroupIDs(group)
		if err != nil {
			t.Fatalf("get %q assignments after rejected rename: %v", group, err)
		}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%q assignments after rejected rename = %#v, want [%s]", group, got, want)
		}
	}

	var names []string
	if err := db.Model(&model.ClientGroup{}).Order("name ASC").Pluck("name", &names).Error; err != nil {
		t.Fatalf("list groups after rejected rename: %v", err)
	}
	if len(names) != 2 || names[0] != "staff" || names[1] != "support" {
		t.Fatalf("groups after rejected rename = %#v, want [staff support]", names)
	}
}

func TestClientGroupHostAssignmentPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires PostgreSQL")
	}
	cleanup, err := testpg.IsolatePackage("client_group_hosts")
	if err != nil {
		t.Fatalf("isolate PostgreSQL schema: %v", err)
	}
	t.Cleanup(cleanup)
	seedClientGroupHostAssignmentDB(t)

	db := database.GetDB()
	for _, name := range []string{"staff", "support"} {
		if err := db.Create(&model.ClientGroup{Name: name}).Error; err != nil {
			t.Fatalf("create client group %q: %v", name, err)
		}
	}
	for _, id := range []string{"cdn-a", "cdn-b"} {
		if err := db.Create(&model.Host{InboundId: 1, GroupId: id, Remark: id}).Error; err != nil {
			t.Fatalf("create host group %q: %v", id, err)
		}
	}

	svc := &ClientGroupHostAssignmentService{}
	if err := svc.SetGroupIDs("staff", []string{" cdn-a ", "cdn-a"}); err != nil {
		t.Fatalf("set staff assignment: %v", err)
	}
	if err := svc.SetGroupIDs("support", []string{"cdn-b"}); err != nil {
		t.Fatalf("set support assignment: %v", err)
	}
	assertIDs := func(group string, want string) {
		t.Helper()
		ids, err := svc.GetGroupIDs(group)
		if err != nil {
			t.Fatalf("get %q assignment: %v", group, err)
		}
		if len(ids) != 1 || ids[0] != want {
			t.Fatalf("%q assignment = %#v, want [%s]", group, ids, want)
		}
	}
	assertIDs("staff", "cdn-a")
	assertIDs("support", "cdn-b")

	if err := svc.SetGroupIDs("staff", []string{"missing"}); err == nil {
		t.Fatal("unknown HostGroup ID should be rejected")
	}
	assertIDs("staff", "cdn-a")

	if _, err := (&ClientService{}).RenameGroup("staff", "support"); err == nil {
		t.Fatal("renaming into an existing group should fail")
	}
	assertIDs("staff", "cdn-a")
	assertIDs("support", "cdn-b")

	if _, err := (&ClientService{}).RenameGroup("staff", "employees"); err != nil {
		t.Fatalf("rename group: %v", err)
	}
	assertIDs("employees", "cdn-a")
	if _, err := (&ClientService{}).DeleteGroup("employees"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	ids, err := svc.GetGroupIDs("employees")
	if err != nil {
		t.Fatalf("get deleted group assignment: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("deleted PostgreSQL assignment = %#v, want empty", ids)
	}

	if err := svc.SetGroupIDs("support", []string{}); err != nil {
		t.Fatalf("clear support assignment: %v", err)
	}
	ids, err = svc.GetGroupIDs("support")
	if err != nil {
		t.Fatalf("get cleared support assignment: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("cleared PostgreSQL assignment = %#v, want empty", ids)
	}

	if err := db.Create(&model.ClientRecord{Email: "legacy@e", Group: "legacy"}).Error; err != nil {
		t.Fatalf("create client with derived group: %v", err)
	}
	if err := db.Create(&model.Host{InboundId: 1, GroupId: "cdn-c", Remark: "C"}).Error; err != nil {
		t.Fatalf("create derived-group host: %v", err)
	}
	if err := svc.SetGroupIDs("legacy", []string{"cdn-c"}); err != nil {
		t.Fatalf("assign HostGroup to derived group: %v", err)
	}
	var count int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "legacy").Count(&count).Error; err != nil {
		t.Fatalf("count persisted derived group: %v", err)
	}
	if count != 1 {
		t.Fatalf("persisted derived group count = %d, want 1", count)
	}
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "legacy@e").Update("group_name", "").Error; err != nil {
		t.Fatalf("remove last client from derived group: %v", err)
	}
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "legacy").Count(&count).Error; err != nil {
		t.Fatalf("count derived group after member removal: %v", err)
	}
	if count != 1 {
		t.Fatalf("derived group should remain persisted after member removal, got %d rows", count)
	}
}

func TestClientGroupHostAssignment_PersistsDerivedGroup(t *testing.T) {
	seedClientGroupHostAssignmentDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientRecord{Email: "legacy@e", Group: "legacy"}).Error; err != nil {
		t.Fatalf("create client with derived group: %v", err)
	}
	if err := db.Create(&model.Host{InboundId: 1, GroupId: "cdn-a", Remark: "A"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}

	svc := &ClientGroupHostAssignmentService{}
	if err := svc.SetGroupIDs("legacy", []string{"cdn-a"}); err != nil {
		t.Fatalf("assign HostGroup to derived group: %v", err)
	}

	var count int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "legacy").Count(&count).Error; err != nil {
		t.Fatalf("count persisted groups: %v", err)
	}
	if count != 1 {
		t.Fatalf("persisted group count = %d, want 1 after assigning defaults", count)
	}

	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "legacy@e").Update("group_name", "").Error; err != nil {
		t.Fatalf("remove last client from group: %v", err)
	}
	var groups []model.ClientGroup
	if err := db.Where("name = ?", "legacy").Find(&groups).Error; err != nil {
		t.Fatalf("load empty group: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("group should remain visible after its last member leaves, got %d rows", len(groups))
	}
	ids, err := svc.GetGroupIDs("legacy")
	if err != nil {
		t.Fatalf("get HostGroup defaults after member removal: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cdn-a" {
		t.Fatalf("HostGroup defaults after member removal = %#v, want [cdn-a]", ids)
	}

	if err := db.Create(&model.ClientRecord{Email: "empty@e", Group: "empty"}).Error; err != nil {
		t.Fatalf("create client in derived group to clear: %v", err)
	}
	if err := svc.SetGroupIDs("empty", []string{}); err != nil {
		t.Fatalf("clear assignment on derived group: %v", err)
	}
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", "empty").Count(&count).Error; err != nil {
		t.Fatalf("count persisted groups after clearing: %v", err)
	}
	if count != 0 {
		t.Fatalf("clearing an assignment should not persist an empty derived group, got %d rows", count)
	}
}
