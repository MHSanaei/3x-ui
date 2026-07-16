package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func countClientRecords(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Count(&n).Error; err != nil {
		t.Fatalf("count client records: %v", err)
	}
	return n
}

func TestUpdateInboundClientRenameDoesNotDuplicateRecord(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "old@x", ID: "aaaaaaaa-0000-0000-0000-000000000001", SubID: "sub-old", Enable: true}}
	ib := mkInbound(t, 22001, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	origId := lookupClientRecord(t, "old@x").Id

	renamed := source
	renamed[0].Email = "new@x"
	if _, err := svc.UpdateInboundClient(inboundSvc, &model.Inbound{
		Id:       ib.Id,
		Settings: clientsSettings(t, renamed),
	}, "old@x"); err != nil {
		t.Fatalf("UpdateInboundClient: %v", err)
	}

	if n := countClientRecords(t); n != 1 {
		t.Fatalf("client records after rename = %d, want 1", n)
	}
	rec := lookupClientRecord(t, "new@x")
	if rec.Id != origId {
		t.Fatalf("record id after rename = %d, want %d", rec.Id, origId)
	}
}

func TestClientUpdateAllowsSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "keep@x", ID: "aaaaaaaa-0000-0000-0000-000000000003", SubID: "sub-keep", Enable: true},
		{Email: "other@x", ID: "aaaaaaaa-0000-0000-0000-000000000004", SubID: "sub-other", Enable: true},
	}
	ib := mkInbound(t, 22003, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	origId := lookupClientRecord(t, "keep@x").Id

	updated := source[0]
	updated.SubID = "sub-other"
	if _, err := svc.Update(inboundSvc, origId, updated); err != nil {
		t.Fatalf("Update with shared subId: %v", err)
	}

	rec := lookupClientRecord(t, "keep@x")
	if rec.Id != origId {
		t.Fatalf("record id changed after update")
	}
	if rec.SubID != "sub-other" {
		t.Fatalf("subId after update = %q, want %q", rec.SubID, "sub-other")
	}
	if got := lookupClientRecord(t, "other@x").SubID; got != "sub-other" {
		t.Fatalf("other client's subId = %q, want unchanged %q", got, "sub-other")
	}
}

func mustInboundSettings(t *testing.T, inboundSvc *InboundService, id int) string {
	t.Helper()
	ib, err := inboundSvc.GetInbound(id)
	if err != nil {
		t.Fatalf("GetInbound %d: %v", id, err)
	}
	return ib.Settings
}
