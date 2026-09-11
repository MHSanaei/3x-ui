package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestExportImportPreservesDisabledEnable covers #6478: ExportAll keeps the
// real enable flag, and ImportClients (attached via BulkCreate + orphan path)
// must not force enable=true.
func TestExportImportPreservesDisabledEnable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 26001, model.VLESS, `{"clients":[]}`)
	const email = "portable@disabled"
	const subID = "sub-portable-disabled"
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := lookupClientRecord(t, email)
	updated := rec.ToClient()
	updated.Enable = false
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update disable: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)

	exported, err := svc.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(exported) != 1 {
		t.Fatalf("ExportAll len=%d, want 1", len(exported))
	}
	if exported[0].Client.Enable {
		t.Fatal("ExportAll should carry enable=false for a disabled client")
	}

	raw, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var roundTrip []ClientCreatePayload
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if roundTrip[0].Client.Enable {
		t.Fatal("JSON round-trip lost enable=false")
	}

	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	res, _, err := svc.ImportClients(inboundSvc, roundTrip)
	if err != nil {
		t.Fatalf("ImportClients: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 0 {
		t.Fatalf("ImportClients result=%+v", res)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestImportClientsPreservesOrphanDisabledEnable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}

	items := []ClientCreatePayload{{
		Client: model.Client{
			Email: "orphan@disabled", SubID: "sub-orphan-disabled", Enable: false,
			ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		},
		InboundIds: nil,
	}}
	res, _, err := svc.ImportClients(&InboundService{}, items)
	if err != nil {
		t.Fatalf("ImportClients orphan: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created=%d, want 1; skipped=%v", res.Created, res.Skipped)
	}
	if got := recordEnableOf(t, svc, "orphan@disabled"); got {
		t.Fatal("orphan import forced enable=true; want false")
	}
}

func TestBulkCreatePreservesExplicitDisable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 26002, model.VLESS, `{"clients":[]}`)
	const email = "bulk@disabled"
	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{
		Client: model.Client{
			Email: email, SubID: "sub-bulk-disabled", Enable: false,
			ID: "cccccccc-cccc-cccc-cccc-cccccccccccc",
		},
		InboundIds: []int{ib.Id},
	}})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("BulkCreate result=%+v", res)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}
