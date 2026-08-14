package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSetInboundSubSortIndexLeavesSettingsUntouched(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const settings = `{"clients":[{"email":"a@example.test","id":"11111111-1111-1111-1111-111111111111"}]}`
	ib := &model.Inbound{UserId: 1, Remark: "r", Port: 21001, Protocol: model.VLESS, Settings: settings, SubSortIndex: 1, Enable: true}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := InboundService{}
	if err := svc.SetInboundSubSortIndex(ib.Id, 7); err != nil {
		t.Fatalf("set: %v", err)
	}

	var got model.Inbound
	if err := database.GetDB().First(&got, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.SubSortIndex != 7 {
		t.Fatalf("subSortIndex = %d, want 7", got.SubSortIndex)
	}
	if got.Settings != settings {
		t.Fatalf("settings were rewritten:\n got %s\nwant %s", got.Settings, settings)
	}
}

func TestSetInboundSubSortIndexUsesNarrowNodeUpdate(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)
	ib := nodeInbound(t, nodeID, 21002, []model.Client{{Email: "a@example.test", ID: "11111111-1111-1111-1111-111111111111"}})
	ib.SubSortIndex = 1
	if err := database.GetDB().Model(ib).Update("sub_sort_index", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&InboundService{}).SetInboundSubSortIndex(ib.Id, 7); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := fake.updateSubSort.Load(); got != 1 {
		t.Fatalf("narrow node updates = %d, want 1", got)
	}
	if got := fake.updateInbound.Load(); got != 0 {
		t.Fatalf("full snapshot node updates = %d, want 0", got)
	}
}
