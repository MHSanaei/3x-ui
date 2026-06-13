package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func makeInboundWithSubSortIndex(tag string, port int, subSortIndex int) *model.Inbound {
	return &model.Inbound{
		UserId:         1,
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
		SubSortIndex:   subSortIndex,
	}
}

// TestUpdateInbound_PersistsSubSortIndex verifies that UpdateInbound copies
// SubSortIndex from the incoming update payload to the persisted row.
func TestUpdateInbound_PersistsSubSortIndex(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSubSortIndex("in-7001-tcp", 7001, 1)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	update := *ib
	update.SubSortIndex = 7

	svc := &InboundService{}
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got.SubSortIndex != 7 {
		t.Fatalf("returned SubSortIndex = %d, want 7", got.SubSortIndex)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.SubSortIndex != 7 {
		t.Fatalf("persisted SubSortIndex = %d, want 7", reloaded.SubSortIndex)
	}
}

// TestUpdateInbound_SubSortIndexClampedToMinimum verifies that values below
// the 1-based minimum (0 from clients that predate the field, or negatives)
// are clamped to 1 instead of being stored.
func TestUpdateInbound_SubSortIndexClampedToMinimum(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSubSortIndex("in-7002-tcp", 7002, 5)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := &InboundService{}
	for _, below := range []int{0, -3} {
		update := *ib
		update.SubSortIndex = below

		got, _, err := svc.UpdateInbound(&update)
		if err != nil {
			t.Fatalf("UpdateInbound(%d): %v", below, err)
		}
		if got.SubSortIndex != 1 {
			t.Fatalf("returned SubSortIndex = %d for input %d, want 1", got.SubSortIndex, below)
		}

		var reloaded model.Inbound
		if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.SubSortIndex != 1 {
			t.Fatalf("persisted SubSortIndex = %d for input %d, want 1", reloaded.SubSortIndex, below)
		}
	}
}

// TestAddInbound_SubSortIndexClampedToMinimum verifies the same clamping on
// the create path (an omitted form field binds to 0).
func TestAddInbound_SubSortIndexClampedToMinimum(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	ib := makeInboundWithSubSortIndex("in-7003-tcp", 7003, 0)
	got, _, err := svc.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if got.SubSortIndex != 1 {
		t.Fatalf("returned SubSortIndex = %d, want 1", got.SubSortIndex)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, got.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.SubSortIndex != 1 {
		t.Fatalf("persisted SubSortIndex = %d, want 1", reloaded.SubSortIndex)
	}
}
