package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func inboundKeepAlive(t *testing.T, inboundSvc *InboundService, ibId int, email string) int {
	t.Helper()
	ib, err := inboundSvc.GetInbound(ibId)
	if err != nil {
		t.Fatalf("GetInbound %d: %v", ibId, err)
	}
	clients, err := inboundSvc.GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients %d: %v", ibId, err)
	}
	for i := range clients {
		if clients[i].Email == email {
			return clients[i].KeepAliveSeconds()
		}
	}
	t.Fatalf("email %q not found on inbound %d", email, ibId)
	return 0
}

// seedKeepAliveClient attaches one WireGuard client already carrying a
// PersistentKeepalive, and returns its inbound and client-record id.
func seedKeepAliveClient(t *testing.T, email string, keepAlive int) (*model.Inbound, int) {
	t.Helper()
	svc := &ClientService{}

	seeded := model.Client{
		Email:      email,
		SubID:      "sub-" + email,
		Enable:     true,
		AllowedIPs: []string{"10.0.0.5/32"},
		KeepAlive:  model.KeepAlivePtr(keepAlive),
	}
	ib := mkInbound(t, 51820, model.WireGuard, clientsSettings(t, []model.Client{seeded}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{seeded}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	return ib, lookupClientRecord(t, email).Id
}

// Before KeepAlive became a pointer, an explicit 0 was indistinguishable
// from "omitted" and got silently restored from the stored value instead.
func TestUpdateCanClearKeepAliveOnAnExistingClient(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	ib, recId := seedKeepAliveClient(t, "ka@x", 25)
	if got := inboundKeepAlive(t, inboundSvc, ib.Id, "ka@x"); got != 25 {
		t.Fatalf("seeded keepAlive = %d, want 25", got)
	}

	updated := model.Client{
		Email:      "ka@x",
		Enable:     true,
		AllowedIPs: []string{"10.0.0.5/32"},
		KeepAlive:  model.KeepAlivePtr(0),
	}
	if _, err := svc.Update(inboundSvc, recId, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := inboundKeepAlive(t, inboundSvc, ib.Id, "ka@x"); got != 0 {
		t.Fatalf("inbound keepAlive after an explicit 0 = %d, want 0", got)
	}
	if got := lookupClientRecord(t, "ka@x").KeepAlive; got != 0 {
		t.Fatalf("stored wg_keep_alive after an explicit 0 = %d, want 0", got)
	}
}

// The other half of the contract: omitting keepAlive (e.g. a metadata-only
// edit) must leave the stored value alone, via UpdateInboundClient's nil check.
func TestUpdateWithoutKeepAlivePreservesTheStoredValue(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	ib, recId := seedKeepAliveClient(t, "ka@x", 25)

	updated := model.Client{
		Email:      "ka@x",
		Enable:     true,
		AllowedIPs: []string{"10.0.0.5/32"},
	}
	if _, err := svc.Update(inboundSvc, recId, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := inboundKeepAlive(t, inboundSvc, ib.Id, "ka@x"); got != 25 {
		t.Fatalf("inbound keepAlive after an edit that omitted it = %d, want 25", got)
	}
	if got := lookupClientRecord(t, "ka@x").KeepAlive; got != 25 {
		t.Fatalf("stored wg_keep_alive after an edit that omitted it = %d, want 25", got)
	}
}

// Same "omit preserves" contract through client_crud.go's no-attached-inbound
// fallback path -- fork-specific, upstream has no equivalent code path here.
func TestUpdateWithoutInboundPreservesKeepAliveOnMetadataOnlyEdit(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	rec := &model.ClientRecord{
		Email:      "ka-noib@x",
		UUID:       "33333333-3333-3333-3333-333333333333",
		SubID:      "ka-noib@x",
		AllowedIPs: "10.0.0.6/32",
		KeepAlive:  30,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}

	updated := rec.ToClient()
	updated.Comment = "renamed via metadata-only edit"
	updated.KeepAlive = nil
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := lookupClientRecord(t, "ka-noib@x").KeepAlive; got != 30 {
		t.Fatalf("stored wg_keep_alive after a no-inbound metadata-only edit = %d, want 30", got)
	}
}
