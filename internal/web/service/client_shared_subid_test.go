package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientCreateAllowsSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	firstInbound := mkInbound(t, 23201, model.VLESS, `{"clients":[]}`)
	secondInbound := mkInbound(t, 23202, model.VLESS, `{"clients":[]}`)

	const sharedSubID = "shared-create"
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "first@shared", ID: "11111111-1111-1111-1111-111111111111", SubID: sharedSubID, Enable: true,
		},
		InboundIds: []int{firstInbound.Id},
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "second@shared", ID: "22222222-2222-2222-2222-222222222222", SubID: sharedSubID, Enable: true,
		},
		InboundIds: []int{secondInbound.Id},
	}); err != nil {
		t.Fatalf("Create with shared subId: %v", err)
	}

	first := lookupClientRecord(t, "first@shared")
	second := lookupClientRecord(t, "second@shared")
	if first.Id == second.Id || first.Email == second.Email {
		t.Fatalf("clients were not kept distinct: first=%+v second=%+v", first, second)
	}
	if first.SubID != sharedSubID || second.SubID != sharedSubID {
		t.Fatalf("subIds = %q and %q, want %q", first.SubID, second.SubID, sharedSubID)
	}
	for _, tc := range []struct {
		recordID int
		wantID   int
	}{
		{recordID: first.Id, wantID: firstInbound.Id},
		{recordID: second.Id, wantID: secondInbound.Id},
	} {
		ids, err := svc.GetInboundIdsForRecord(tc.recordID)
		if err != nil {
			t.Fatalf("GetInboundIdsForRecord(%d): %v", tc.recordID, err)
		}
		if len(ids) != 1 || ids[0] != tc.wantID {
			t.Fatalf("record %d inbound ids = %v, want [%d]", tc.recordID, ids, tc.wantID)
		}
	}
}

func TestClientBulkCreateAllowsSharedSubIDInOneBatch(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	inbound := mkInbound(t, 23203, model.VLESS, `{"clients":[]}`)

	const sharedSubID = "shared-bulk"
	result, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client: model.Client{
				Email: "bulk-one@shared", ID: "33333333-3333-3333-3333-333333333333", SubID: sharedSubID, Enable: true,
			},
			InboundIds: []int{inbound.Id},
		},
		{
			Client: model.Client{
				Email: "bulk-two@shared", ID: "44444444-4444-4444-4444-444444444444", SubID: sharedSubID, Enable: true,
			},
			InboundIds: []int{inbound.Id},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate with shared subId: %v", err)
	}
	if result.Created != 2 || len(result.Skipped) != 0 {
		t.Fatalf("BulkCreate result = %+v, want 2 created and no skips", result)
	}

	first := lookupClientRecord(t, "bulk-one@shared")
	second := lookupClientRecord(t, "bulk-two@shared")
	if first.Id == second.Id || first.SubID != sharedSubID || second.SubID != sharedSubID {
		t.Fatalf("bulk clients not distinct with shared subId: first=%+v second=%+v", first, second)
	}
	clients, err := svc.ListForInbound(nil, inbound.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("inbound clients = %+v, want 2", clients)
	}
}

func TestClientImportOrphanAllowsExistingSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	inbound := mkInbound(t, 23204, model.VLESS, `{"clients":[]}`)

	const sharedSubID = "shared-import"
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "existing@shared", ID: "55555555-5555-5555-5555-555555555555", SubID: sharedSubID, Enable: true,
		},
		InboundIds: []int{inbound.Id},
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	existing := lookupClientRecord(t, "existing@shared")

	result, _, err := svc.ImportClients(inboundSvc, []ClientCreatePayload{{
		Client: model.Client{
			Email: "orphan@shared", ID: "66666666-6666-6666-6666-666666666666", SubID: sharedSubID, Enable: true,
		},
	}})
	if err != nil {
		t.Fatalf("ImportClients with shared subId: %v", err)
	}
	if result.Created != 1 || len(result.Skipped) != 0 {
		t.Fatalf("ImportClients result = %+v, want 1 created and no skips", result)
	}

	unchanged := lookupClientRecord(t, "existing@shared")
	orphan := lookupClientRecord(t, "orphan@shared")
	if unchanged.Id != existing.Id || unchanged.UUID != existing.UUID {
		t.Fatalf("existing client was overwritten: before=%+v after=%+v", existing, unchanged)
	}
	if orphan.Id == unchanged.Id || orphan.SubID != sharedSubID || unchanged.SubID != sharedSubID {
		t.Fatalf("imported clients not distinct with shared subId: existing=%+v orphan=%+v", unchanged, orphan)
	}
	ids, err := svc.GetInboundIdsForRecord(orphan.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("orphan inbound ids = %v, want none", ids)
	}
}

func TestClientCreateStillRejectsIdentityAndMalformedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	inbound := mkInbound(t, 23205, model.VLESS, `{"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "identity@shared", ID: "77777777-7777-7777-7777-777777777777", SubID: "identity-original", Enable: true,
		},
		InboundIds: []int{inbound.Id},
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "identity@shared", SubID: "identity-other", Enable: true},
		InboundIds: []int{inbound.Id},
	}); err == nil || !strings.Contains(err.Error(), "email already in use") {
		t.Fatalf("duplicate-email Create error = %v, want email already in use", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "malformed@shared", SubID: "bad sub", Enable: true},
		InboundIds: []int{inbound.Id},
	}); err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("malformed-subId Create error = %v, want invalid character", err)
	}
}
