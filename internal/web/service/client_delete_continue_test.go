package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDeleteContinuesPastFailedInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "spread@x", ID: "cccccccc-0000-0000-0000-000000000001", SubID: "sub-spread", Enable: true}}
	ib1 := mkInbound(t, 23001, model.VLESS, clientsSettings(t, source))
	ib2 := mkInbound(t, 23002, model.VLESS, clientsSettings(t, source))
	ib3 := mkInbound(t, 23003, model.VLESS, clientsSettings(t, source))
	for _, ib := range []*model.Inbound{ib1, ib2, ib3} {
		if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
			t.Fatalf("seed linkage for %d: %v", ib.Id, err)
		}
	}
	rec := lookupClientRecord(t, "spread@x")

	missingNode := 9999
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib2.Id).
		Update("node_id", missingNode).Error; err != nil {
		t.Fatalf("point inbound 2 at a missing node: %v", err)
	}

	_, err := svc.Delete(inboundSvc, rec.Id, false)
	if err == nil {
		t.Fatalf("Delete with a failing inbound succeeded, want error")
	}
	if !strings.Contains(err.Error(), "inbound "+strconv.Itoa(ib2.Id)) {
		t.Fatalf("Delete error = %q, want it to name inbound %d", err, ib2.Id)
	}

	for _, ib := range []*model.Inbound{ib1, ib3} {
		if settingsHoldUUID(t, inboundSvc, ib.Id, "spread@x") {
			t.Fatalf("inbound %d still holds the client after Delete", ib.Id)
		}
	}
	if _, err := svc.GetByID(rec.Id); err != nil {
		t.Fatalf("record removed despite a failed inbound: %v", err)
	}

	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib2.Id).
		Update("node_id", nil).Error; err != nil {
		t.Fatalf("repair inbound 2: %v", err)
	}
	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("retry Delete: %v", err)
	}
	if _, err := svc.GetByID(rec.Id); err == nil {
		t.Fatalf("record still present after successful retry")
	}
	if settingsHoldUUID(t, inboundSvc, ib2.Id, "spread@x") {
		t.Fatalf("inbound 2 still holds the client after retry")
	}
}
