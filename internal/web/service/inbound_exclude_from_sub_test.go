package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdateInbound_PersistsExcludeFromSub(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSubSortIndex("in-7004-tcp", 7004, 1)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	update := *ib
	update.ExcludeFromSub = true
	got, _, err := (&InboundService{}).UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if !got.ExcludeFromSub {
		t.Fatal("returned ExcludeFromSub = false, want true")
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.ExcludeFromSub {
		t.Fatal("persisted ExcludeFromSub = false, want true")
	}
}

func TestAddInbound_PersistsExcludeFromSub(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSubSortIndex("in-7005-tcp", 7005, 1)
	ib.ExcludeFromSub = true
	got, _, err := (&InboundService{}).AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if !got.ExcludeFromSub {
		t.Fatal("returned ExcludeFromSub = false, want true")
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, got.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.ExcludeFromSub {
		t.Fatal("persisted ExcludeFromSub = false, want true")
	}
}
