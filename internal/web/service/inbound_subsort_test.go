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
