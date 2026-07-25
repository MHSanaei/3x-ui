package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCreateAcrossManyInboundsUsesOneEmailSnapshot(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const uuid = "bbbbbbbb-1111-2222-3333-555555555555"
	ids := make([]int, 0, 6)
	for i := range 6 {
		ib := mkInbound(t, 23001+i, model.VLESS, `{"clients":[]}`)
		ids = append(ids, ib.Id)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "fan@x", ID: uuid, SubID: "sub-fan", Enable: true},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("Create across %d inbounds: %v", len(ids), err)
	}

	if n := countClientRecords(t); n != 1 {
		t.Fatalf("client records = %d, want 1", n)
	}
	rec := lookupClientRecord(t, "fan@x")
	if rec.UUID != uuid || rec.SubID != "sub-fan" {
		t.Fatalf("record = {uuid:%q sub:%q}, want {%q sub-fan}", rec.UUID, rec.SubID, uuid)
	}
	for _, id := range ids {
		if !settingsHoldUUID(t, inboundSvc, id, uuid) {
			t.Fatalf("inbound %d settings missing the client", id)
		}
	}

	linked, err := svc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(linked) != len(ids) {
		t.Fatalf("linked inbounds = %d, want %d", len(linked), len(ids))
	}
}

func TestAttachAcrossManyInboundsUsesOneEmailSnapshot(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	first := mkInbound(t, 23101, model.VLESS, `{"clients":[]}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "att@x", ID: "cccccccc-1111-2222-3333-666666666666", SubID: "sub-att", Enable: true},
		InboundIds: []int{first.Id},
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	rec := lookupClientRecord(t, "att@x")

	ids := []int{first.Id}
	for i := range 4 {
		ib := mkInbound(t, 23102+i, model.VLESS, `{"clients":[]}`)
		ids = append(ids, ib.Id)
	}

	if _, err := svc.Attach(inboundSvc, rec.Id, ids); err != nil {
		t.Fatalf("Attach across %d inbounds: %v", len(ids), err)
	}

	if n := countClientRecords(t); n != 1 {
		t.Fatalf("client records after attach = %d, want 1", n)
	}
	linked, err := svc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(linked) != len(ids) {
		t.Fatalf("linked inbounds = %d, want %d", len(linked), len(ids))
	}
}
