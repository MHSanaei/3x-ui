package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedDualProtocolClient creates a WireGuard inbound and an AmneziaWG inbound
// (real, distinct subnets: 10.0.0.0/24 and 10.8.1.0/24), attaches the same
// email to both with its own correct, protocol-appropriate address, and
// returns the two inbounds plus the shared client record id.
func seedDualProtocolClient(t *testing.T, email, wgAddr, awgAddr string) (wgIb, awgIb *model.Inbound, recordId int) {
	t.Helper()
	svc := &ClientService{}

	wgClient := model.Client{Email: email, SubID: "sub-" + email, Enable: true, AllowedIPs: []string{wgAddr}}
	wgIb = mkInbound(t, 51820, model.WireGuard, clientsSettings(t, []model.Client{wgClient}))
	if err := svc.SyncInbound(nil, wgIb.Id, []model.Client{wgClient}); err != nil {
		t.Fatalf("seed wg linkage: %v", err)
	}

	awgClient := model.Client{Email: email, SubID: "sub-" + email, Enable: true, AllowedIPs: []string{awgAddr}}
	awgIb = mkInbound(t, 443, model.AmneziaWG, clientsSettings(t, []model.Client{awgClient}))
	if err := svc.SyncInbound(nil, awgIb.Id, []model.Client{awgClient}); err != nil {
		t.Fatalf("seed awg linkage: %v", err)
	}

	recordId = lookupClientRecord(t, email).Id
	return wgIb, awgIb, recordId
}

func inboundAllowedIPs(t *testing.T, inboundSvc *InboundService, ibId int, email string) []string {
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
			return clients[i].AllowedIPs
		}
	}
	t.Fatalf("email %q not found on inbound %d", email, ibId)
	return nil
}

// TestUpdateBroadcastAllowedIPsDoesNotOverwriteOtherInboundWhenMismatched is a
// regression test for the same bug class already fixed for Attach
// (addressesFitAmneziaWGInbound), but on the far more common Update path: the
// edit-client form sends one shared AllowedIPs value, and Update's per-inbound
// loop used to broadcast it verbatim to every attached inbound, including one
// it doesn't belong to. A client attached to both wg (10.0.0.5/32) and awg
// (10.8.1.5/32) saving with the wg-labeled value as the single shared field
// must not silently overwrite the awg inbound's own, unrelated address.
func TestUpdateBroadcastAllowedIPsDoesNotOverwriteOtherInboundWhenMismatched(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	wgIb, awgIb, recId := seedDualProtocolClient(t, "dual@x", "10.0.0.5/32", "10.8.1.5/32")

	updated := model.Client{Email: "dual@x", Enable: true, AllowedIPs: []string{"10.0.0.5/32"}}
	if _, err := svc.Update(inboundSvc, recId, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := inboundAllowedIPs(t, inboundSvc, wgIb.Id, "dual@x"); len(got) != 1 || got[0] != "10.0.0.5/32" {
		t.Fatalf("wg AllowedIPs = %v, want [10.0.0.5/32]", got)
	}
	if got := inboundAllowedIPs(t, inboundSvc, awgIb.Id, "dual@x"); len(got) != 1 || got[0] != "10.8.1.5/32" {
		t.Fatalf("the real bug: awg AllowedIPs = %v, want unchanged [10.8.1.5/32] (must not inherit the wg-labeled shared value)", got)
	}
}

// TestUpdateAllowedIPsByInboundAppliesDistinctValuesPerInbound covers the new
// mechanism the two-field client-edit form uses to intentionally change both
// addresses in one save: distinct, valid, per-inbound override values must
// each land on their own inbound.
func TestUpdateAllowedIPsByInboundAppliesDistinctValuesPerInbound(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	wgIb, awgIb, recId := seedDualProtocolClient(t, "dual@x", "10.0.0.5/32", "10.8.1.5/32")

	updated := model.Client{
		Email:  "dual@x",
		Enable: true,
		AllowedIPsByInbound: map[int][]string{
			wgIb.Id:  {"10.0.0.9/32"},
			awgIb.Id: {"10.8.1.9/32"},
		},
	}
	if _, err := svc.Update(inboundSvc, recId, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := inboundAllowedIPs(t, inboundSvc, wgIb.Id, "dual@x"); len(got) != 1 || got[0] != "10.0.0.9/32" {
		t.Fatalf("wg AllowedIPs = %v, want [10.0.0.9/32]", got)
	}
	if got := inboundAllowedIPs(t, inboundSvc, awgIb.Id, "dual@x"); len(got) != 1 || got[0] != "10.8.1.9/32" {
		t.Fatalf("awg AllowedIPs = %v, want [10.8.1.9/32]", got)
	}
}

// TestCreateSharedAllowedIPsThatDontFitAmneziaWGGetsFreshAllocation is
// Create's counterpart to the Update regression above: adding a brand-new
// client to both wg and awg inbounds at once with a single manually-typed
// address must not hand the awg inbound an address from the wrong subnet --
// it must fall back to auto-allocating a real, correctly-scoped address
// instead, exactly as if AllowedIPs had been left empty for that inbound.
func TestCreateSharedAllowedIPsThatDontFitAmneziaWGGetsFreshAllocation(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	wgIb := mkInbound(t, 51820, model.WireGuard, wgServerSettings())
	awgIb := mkInbound(t, 443, model.AmneziaWG, amneziawgClientTestSettings)

	payload := &ClientCreatePayload{
		Client:     model.Client{Email: "new@x", Enable: true, AllowedIPs: []string{"10.0.0.7/32"}},
		InboundIds: []int{wgIb.Id, awgIb.Id},
	}
	if _, err := svc.Create(inboundSvc, payload); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := inboundAllowedIPs(t, inboundSvc, wgIb.Id, "new@x"); len(got) != 1 || got[0] != "10.0.0.7/32" {
		t.Fatalf("wg AllowedIPs = %v, want [10.0.0.7/32]", got)
	}
	got := inboundAllowedIPs(t, inboundSvc, awgIb.Id, "new@x")
	if len(got) != 1 {
		t.Fatalf("awg AllowedIPs = %v, want exactly one freshly allocated address", got)
	}
	if got[0] == "10.0.0.7/32" {
		t.Fatal("the real bug: awg inbound inherited the wg-shaped shared address instead of allocating its own")
	}
	if !addressesFitAmneziaWGInbound(got, awgIb) {
		t.Fatalf("freshly allocated awg address %v does not actually fit the awg inbound's own subnet", got)
	}
}

// TestCreateAllowedIPsByInboundAppliesDistinctValuesPerInbound is Create's
// counterpart to the Update explicit-override test: the add-client form,
// when attaching to both wg and awg at once with the two-field UI, must be
// able to give each inbound its own manually chosen address in one call.
func TestCreateAllowedIPsByInboundAppliesDistinctValuesPerInbound(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	wgIb := mkInbound(t, 51820, model.WireGuard, wgServerSettings())
	awgIb := mkInbound(t, 443, model.AmneziaWG, amneziawgClientTestSettings)

	payload := &ClientCreatePayload{
		Client: model.Client{
			Email:  "new@x",
			Enable: true,
			AllowedIPsByInbound: map[int][]string{
				wgIb.Id:  {"10.0.0.9/32"},
				awgIb.Id: {"10.8.1.9/32"},
			},
		},
		InboundIds: []int{wgIb.Id, awgIb.Id},
	}
	if _, err := svc.Create(inboundSvc, payload); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := inboundAllowedIPs(t, inboundSvc, wgIb.Id, "new@x"); len(got) != 1 || got[0] != "10.0.0.9/32" {
		t.Fatalf("wg AllowedIPs = %v, want [10.0.0.9/32]", got)
	}
	if got := inboundAllowedIPs(t, inboundSvc, awgIb.Id, "new@x"); len(got) != 1 || got[0] != "10.8.1.9/32" {
		t.Fatalf("awg AllowedIPs = %v, want [10.8.1.9/32]", got)
	}
}

// TestTunnelAllowedIPsByInbound covers the GET-client read side: a two-field
// display needs the real, distinct per-inbound address for each protocol,
// which ClientRecord's own single AllowedIPs column cannot represent.
func TestTunnelAllowedIPsByInbound(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	wgIb, awgIb, _ := seedDualProtocolClient(t, "dual@x", "10.0.0.5/32", "10.8.1.5/32")
	vlessIb := mkInbound(t, 8443, model.VLESS, clientsSettings(t, nil))

	got, err := svc.TunnelAllowedIPsByInbound(inboundSvc, "dual@x", []int{wgIb.Id, awgIb.Id, vlessIb.Id, 999999})
	if err != nil {
		t.Fatalf("TunnelAllowedIPsByInbound: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("result = %v, want exactly 2 entries (vless and the nonexistent id must be skipped)", got)
	}
	if got[wgIb.Id] != "10.0.0.5/32" {
		t.Fatalf("wg entry = %q, want 10.0.0.5/32", got[wgIb.Id])
	}
	if got[awgIb.Id] != "10.8.1.5/32" {
		t.Fatalf("awg entry = %q, want 10.8.1.5/32", got[awgIb.Id])
	}
	if _, ok := got[vlessIb.Id]; ok {
		t.Fatalf("a non-tunnel (VLESS) inbound must not appear in the result")
	}
	if _, ok := got[999999]; ok {
		t.Fatalf("a nonexistent inbound id must not appear in the result")
	}
}
