package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestAutoRenewClients_MultiInbound covers the renew loop across more than one
// inbound: every expired client with reset>0 must get a fresh future expiry,
// zeroed usage and re-enabled state, while a non-expiring client is untouched.
// It also guards the map-lookup refactor of the old quadratic inner loop.
func TestAutoRenewClients_MultiInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	future := time.Now().Add(365 * 24 * time.Hour).UnixMilli()

	// Two inbounds, two expiring clients each, plus one client that never expires.
	ib1Clients := []model.Client{
		{Email: "a@x", ID: "11111111-1111-1111-1111-111111111111", Enable: false, Reset: 30, ExpiryTime: past},
		{Email: "b@x", ID: "22222222-2222-2222-2222-222222222222", Enable: false, Reset: 30, ExpiryTime: past},
	}
	ib2Clients := []model.Client{
		{Email: "c@x", ID: "33333333-3333-3333-3333-333333333333", Enable: false, Reset: 7, ExpiryTime: past},
		{Email: "keep@x", ID: "44444444-4444-4444-4444-444444444444", Enable: true, Reset: 0, ExpiryTime: future},
	}

	ib1 := mkInbound(t, 30001, model.VLESS, clientsSettings(t, ib1Clients))
	ib2 := mkInbound(t, 30002, model.VLESS, clientsSettings(t, ib2Clients))
	if err := svc.clientService.SyncInbound(nil, ib1.Id, ib1Clients); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := svc.clientService.SyncInbound(nil, ib2.Id, ib2Clients); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	// Seed traffic rows: expired+depleted for the three renewable clients, and a
	// healthy row for keep@x.
	rows := []xray.ClientTraffic{
		{InboundId: ib1.Id, Email: "a@x", Enable: false, Up: 100, Down: 200, Reset: 30, ExpiryTime: past},
		{InboundId: ib1.Id, Email: "b@x", Enable: false, Up: 300, Down: 400, Reset: 30, ExpiryTime: past},
		{InboundId: ib2.Id, Email: "c@x", Enable: false, Up: 500, Down: 600, Reset: 7, ExpiryTime: past},
		{InboundId: ib2.Id, Email: "keep@x", Enable: true, Up: 1, Down: 2, Reset: 0, ExpiryTime: future},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 3 {
		t.Fatalf("renewed count = %d, want 3", count)
	}

	now := time.Now().UnixMilli()
	for _, email := range []string{"a@x", "b@x", "c@x"} {
		var row xray.ClientTraffic
		if err := db.Where("email = ?", email).First(&row).Error; err != nil {
			t.Fatalf("read %s: %v", email, err)
		}
		if row.Up != 0 || row.Down != 0 {
			t.Errorf("%s: usage not reset: up=%d down=%d", email, row.Up, row.Down)
		}
		if !row.Enable {
			t.Errorf("%s: not re-enabled", email)
		}
		if row.ExpiryTime <= now {
			t.Errorf("%s: expiry not advanced: got %d, now %d", email, row.ExpiryTime, now)
		}
	}

	// The non-expiring client must be left exactly as seeded.
	var keep xray.ClientTraffic
	if err := db.Where("email = ?", "keep@x").First(&keep).Error; err != nil {
		t.Fatalf("read keep@x: %v", err)
	}
	if keep.Up != 1 || keep.Down != 2 || keep.ExpiryTime != future {
		t.Errorf("keep@x was modified: %+v", keep)
	}

	// The renewed state must also be reflected in the inbound settings JSON.
	reloaded, err := svc.GetInbound(ib1.Id)
	if err != nil {
		t.Fatalf("GetInbound ib1: %v", err)
	}
	cs, err := svc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients ib1: %v", err)
	}
	for _, c := range cs {
		if !c.Enable {
			t.Errorf("settings client %s still disabled after renew", c.Email)
		}
		if c.ExpiryTime <= now {
			t.Errorf("settings client %s expiry not advanced: %d", c.Email, c.ExpiryTime)
		}
	}
}
