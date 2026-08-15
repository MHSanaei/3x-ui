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

func TestUpdateInboundClientCaseOnlyRenameDoesNotDuplicateRecord(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "test", ID: "aaaaaaaa-0000-0000-0000-000000000002", SubID: "sub-case", Enable: true}}
	ib := mkInbound(t, 22002, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	origId := lookupClientRecord(t, "test").Id

	updated := source[0]
	updated.Email = "Test"
	if _, err := svc.Update(inboundSvc, origId, updated, 0); err != nil {
		t.Fatalf("Update case-only email: %v", err)
	}

	if n := countClientRecords(t); n != 1 {
		t.Fatalf("client records after case-only rename = %d, want 1", n)
	}
	rec := lookupClientRecord(t, "Test")
	if rec.Id != origId {
		t.Fatalf("record id after case-only rename = %d, want %d", rec.Id, origId)
	}
	if rec.Email != "Test" {
		t.Fatalf("email after case-only rename = %q, want %q", rec.Email, "Test")
	}
}

// The IP-limit job keys its tracking rows on the casing Xray reports, so an
// inbound whose settings JSON drifted in case leaves a row under each spelling.
func TestUpdateInboundClientCaseOnlyRenameSurvivesExistingClientIpsRow(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "Sanaei", ID: "aaaaaaaa-0000-0000-0000-000000000009", SubID: "sub-ips", Enable: true}}
	ib := mkInbound(t, 22011, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	for _, email := range []string{"Sanaei", "sanaei"} {
		row := &model.InboundClientIps{ClientEmail: email, Ips: `[{"ip":"1.2.3.4","timestamp":1700000000}]`}
		if err := database.GetDB().Create(row).Error; err != nil {
			t.Fatalf("seed client ips for %q: %v", email, err)
		}
	}

	lowered := source
	lowered[0].Email = "sanaei"
	if _, err := svc.UpdateInboundClient(inboundSvc, &model.Inbound{
		Id:       ib.Id,
		Settings: clientsSettings(t, lowered),
	}, "sanaei"); err != nil {
		t.Fatalf("UpdateInboundClient with a colliding client ips row: %v", err)
	}

	var rows []model.InboundClientIps
	if err := database.GetDB().Find(&rows).Error; err != nil {
		t.Fatalf("read client ips: %v", err)
	}
	if len(rows) != 1 || rows[0].ClientEmail != "sanaei" {
		t.Fatalf("client ips rows after rename = %+v, want a single row for %q", rows, "sanaei")
	}
}

func TestClientUpdateAllowsSharedSubIDAndRenamesEmail(t *testing.T) {
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
	updated.Email = "kept@x"
	updated.SubID = "sub-other"
	updated.TotalGB = 42
	if _, err := svc.Update(inboundSvc, origId, updated, 0); err != nil {
		t.Fatalf("Update with shared subId: %v", err)
	}

	rec := lookupClientRecord(t, "kept@x")
	if rec.Id != origId {
		t.Fatalf("record id after update = %d, want %d", rec.Id, origId)
	}
	other := lookupClientRecord(t, "other@x")
	if rec.SubID != "sub-other" || other.SubID != "sub-other" {
		t.Fatalf("subIds after update = %q and %q, want shared subId", rec.SubID, other.SubID)
	}
	if rec.Email == other.Email {
		t.Fatalf("updated clients share email %q, want distinct identities", rec.Email)
	}

	inbound, err := inboundSvc.GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	clients, err := inboundSvc.GetClients(inbound)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("inbound clients = %d, want 2", len(clients))
	}
	for _, client := range clients {
		if client.Email == "kept@x" && client.SubID == "sub-other" && client.TotalGB == 42 {
			return
		}
	}
	t.Fatalf("edited client missing from inbound settings: %+v", clients)
}

func TestClientUpdateKeepsSharedSubIDEditable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "a@node", ID: "aaaaaaaa-0000-0000-0000-000000000005", SubID: "sub-shared", Enable: true},
		{Email: "b@node", ID: "aaaaaaaa-0000-0000-0000-000000000006", SubID: "sub-shared", Enable: true},
	}
	ib := mkInbound(t, 22004, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	first := lookupClientRecord(t, "a@node")
	if first.SubID != "sub-shared" || lookupClientRecord(t, "b@node").SubID != "sub-shared" {
		t.Fatalf("seed did not produce a shared subId")
	}

	updated := source[0]
	updated.TotalGB = 42
	if _, err := svc.Update(inboundSvc, first.Id, updated, 0); err != nil {
		t.Fatalf("Update of a client whose subId is already shared: %v", err)
	}
	if got := lookupClientRecord(t, "a@node").TotalGB; got != 42 {
		t.Fatalf("totalGB after update = %d, want 42", got)
	}

	omitted := source[0]
	omitted.SubID = ""
	omitted.TotalGB = 43
	if _, err := svc.Update(inboundSvc, first.Id, omitted, 0); err != nil {
		t.Fatalf("Update with subId omitted entirely: %v", err)
	}
	other := lookupClientRecord(t, "b@node")
	if other.SubID != "sub-shared" {
		t.Fatalf("other client subId = %q, want %q", other.SubID, "sub-shared")
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
