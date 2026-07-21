package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestAddClientStat_RefreshesStaleRowOnInboundDeleteThenReuse covers #5958:
// deleting a client's only inbound leaves its clients/client_traffics rows in
// place (matching ClientService.Detach, which does the same on purpose so a
// later Attach can resume a client with its accumulated traffic intact). If
// that same email is instead reused for a freshly (re)created client via
// ClientService.Create, the new enable/expiry/reset/total must win over
// whatever the orphaned row still holds instead of being silently ignored by
// AddClientStat's OnConflict.
func TestAddClientStat_RefreshesStaleRowOnInboundDeleteThenReuse(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const email = "reused@example.com"
	const subID = "sub-reused"

	ibA := mkInbound(t, 22001, model.VLESS, `{"clients":[]}`)

	// Create starts every client enabled (it forces Enable=true), so the
	// depleted/disabled shape a client has by the time it's naturally deleted
	// is reached the same way production reaches it: a follow-up Update, not
	// the initial Create.
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			TotalGB: 0, ExpiryTime: 1000, Reset: 0,
		},
		InboundIds: []int{ibA.Id},
	}); err != nil {
		t.Fatalf("initial Create: %v", err)
	}
	rec0 := lookupClientRecord(t, email)
	if _, err := svc.Update(inboundSvc, rec0.Id, model.Client{
		Email: email, SubID: subID, Enable: false,
		TotalGB: 0, ExpiryTime: 1000, Reset: 0,
	}); err != nil {
		t.Fatalf("Update to disabled: %v", err)
	}

	db := database.GetDB()
	var before xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&before).Error; err != nil {
		t.Fatalf("lookup client_traffics before delete: %v", err)
	}
	if before.Enable || before.Reset != 0 || before.Total != 0 {
		t.Fatalf("unexpected initial client_traffics row: %+v", before)
	}

	// Delete the client's only inbound. This must NOT delete the client or its
	// traffic row (that's the documented, intentional Detach-parity behavior) —
	// it only orphans them.
	if _, err := inboundSvc.DelInbound(ibA.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}

	rec := lookupClientRecord(t, email)
	ids, err := svc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("client should be fully detached after its only inbound was deleted, still attached to: %v", ids)
	}
	var stillThere xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&stillThere).Error; err != nil {
		t.Fatalf("client_traffics row should survive inbound deletion (Detach parity), lookup failed: %v", err)
	}

	// Reuse the same email + subId (the only way ClientService.Create allows
	// re-adding under an already-used email) on a freshly created inbound, with
	// deliberately different, "fresh" settings.
	ibB := mkInbound(t, 22002, model.VLESS, `{"clients":[]}`)
	const wantExpiry = int64(9999999999000)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			TotalGB: 10 << 30, ExpiryTime: wantExpiry, Reset: 5,
		},
		InboundIds: []int{ibB.Id},
	}); err != nil {
		t.Fatalf("reuse Create: %v", err)
	}

	var after xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&after).Error; err != nil {
		t.Fatalf("lookup client_traffics after reuse: %v", err)
	}
	if !after.Enable {
		t.Errorf("client_traffics.enable still stale (false) after reuse, want true")
	}
	if after.Reset != 5 {
		t.Errorf("client_traffics.reset = %d, want 5 (stale value from before delete was 0)", after.Reset)
	}
	if after.Total != 10<<30 {
		t.Errorf("client_traffics.total = %d, want %d", after.Total, int64(10<<30))
	}
	if after.ExpiryTime != wantExpiry {
		t.Errorf("client_traffics.expiry_time = %d, want %d", after.ExpiryTime, wantExpiry)
	}
	if after.InboundId != ibB.Id {
		t.Errorf("client_traffics.inbound_id = %d, want refreshed to new inbound %d (was %d)", after.InboundId, ibB.Id, ibA.Id)
	}

	// up/down are deliberately NOT refreshed by AddClientStat — confirm that
	// stays true (would matter if the original client had real usage).
	if after.Up != before.Up || after.Down != before.Down {
		t.Errorf("up/down should be left untouched by the conflict refresh: before up=%d down=%d, after up=%d down=%d",
			before.Up, before.Down, after.Up, after.Down)
	}
}

// TestAddClientStat_MultiInboundReattachStaysIdempotent guards the legitimate
// case AddClientStat's OnConflict is also responsible for: a client attached
// to two inbounds at once shares one client_traffics row, and re-asserting
// its own current settings for the second inbound must be a no-op in effect,
// not a data loss. In particular it must not zero out real accumulated
// traffic just because the client gained a second attachment.
func TestAddClientStat_MultiInboundReattachStaysIdempotent(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const email = "multi@example.com"
	const subID = "sub-multi"

	ibA := mkInbound(t, 22003, model.VLESS, `{"clients":[]}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			TotalGB: 5 << 30, ExpiryTime: 42, Reset: 3,
		},
		InboundIds: []int{ibA.Id},
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	db := database.GetDB()
	// Simulate real accumulated usage before the second attachment.
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"up": int64(123), "down": int64(456)}).Error; err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	ibB := mkInbound(t, 22004, model.VLESS, `{"clients":[]}`)
	// Re-adding the same identity to a second inbound: same email/subId/settings,
	// exactly what the panel does when attaching an existing client elsewhere.
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			TotalGB: 5 << 30, ExpiryTime: 42, Reset: 3,
		},
		InboundIds: []int{ibB.Id},
	}); err != nil {
		t.Fatalf("second Create (attach to ibB): %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup after second attach: %v", err)
	}
	if row.Up != 123 || row.Down != 456 {
		t.Errorf("accumulated traffic was reset by re-attach: up=%d down=%d, want 123/456", row.Up, row.Down)
	}
	if !row.Enable || row.Reset != 3 || row.Total != 5<<30 || row.ExpiryTime != 42 {
		t.Errorf("config columns changed unexpectedly on idempotent re-assert: %+v", row)
	}
}
