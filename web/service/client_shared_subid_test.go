package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestClientCreateAllowsSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 23001, model.VLESS, `{"clients":[]}`)

	clients := []model.Client{
		{Email: "alpha@x", ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", SubID: "shared-sub", Enable: true},
		{Email: "beta@x", ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", SubID: "shared-sub", Enable: true},
	}
	for _, client := range clients {
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{ib.Id}}); err != nil {
			t.Fatalf("Create(%q): %v", client.Email, err)
		}
	}

	list, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if got := sortedEmails(list); len(got) != 2 || got[0] != "alpha@x" || got[1] != "beta@x" {
		t.Fatalf("expected both shared-sub clients on inbound, got %v", got)
	}
	for _, client := range list {
		if client.SubID != "shared-sub" {
			t.Fatalf("client %q should keep shared subId, got %q", client.Email, client.SubID)
		}
	}
}

func TestClientUpdateAllowsSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 23002, model.VLESS, `{"clients":[]}`)

	seed := []model.Client{
		{Email: "alpha@x", ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", SubID: "shared-sub", Enable: true},
		{Email: "beta@x", ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", SubID: "beta-sub", Enable: true},
	}
	for _, client := range seed {
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{ib.Id}}); err != nil {
			t.Fatalf("Create(%q): %v", client.Email, err)
		}
	}

	beta, err := svc.GetRecordByEmail(nil, "beta@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail(beta): %v", err)
	}
	updated := beta.ToClient()
	updated.SubID = "shared-sub"
	if _, err := svc.Update(inboundSvc, beta.Id, *updated); err != nil {
		t.Fatalf("Update(beta shared subId): %v", err)
	}

	reloaded, err := inboundSvc.GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	list, err := inboundSvc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if got := sortedEmails(list); len(got) != 2 || got[0] != "alpha@x" || got[1] != "beta@x" {
		t.Fatalf("expected both clients after update, got %v", got)
	}
	for _, client := range list {
		if client.SubID != "shared-sub" {
			t.Fatalf("client %q should have shared subId after update, got %q", client.Email, client.SubID)
		}
	}
}

func TestBulkCreateAllowsSharedSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 23003, model.VLESS, `{"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "alpha@x", ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", SubID: "shared-sub", Enable: true},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("seed alpha: %v", err)
	}

	result, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client:     model.Client{Email: "beta@x", ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", SubID: "shared-sub", Enable: true},
			InboundIds: []int{ib.Id},
		},
		{
			Client:     model.Client{Email: "gamma@x", ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", SubID: "shared-sub", Enable: true},
			InboundIds: []int{ib.Id},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if result.Created != 2 || len(result.Skipped) != 0 {
		t.Fatalf("expected two created and no skipped clients, got created=%d skipped=%v", result.Created, result.Skipped)
	}

	var count int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", "shared-sub").Count(&count).Error; err != nil {
		t.Fatalf("count shared-sub records: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 client records sharing subId, got %d", count)
	}
}
