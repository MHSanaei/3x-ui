package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A prepaid plan must stop itself: once as many renewals have fired as the
// operator allowed, the client expires like any other (#5804).
func TestAutoRenewClients_StopsAtMaxCount(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "spent@x", ID: "11111111-1111-1111-1111-111111111111", Enable: false, Reset: 30, ResetMax: 2, ExpiryTime: past},
		{Email: "left@x", ID: "22222222-2222-2222-2222-222222222222", Enable: false, Reset: 30, ResetMax: 2, ExpiryTime: past},
	}
	ib := mkInbound(t, 30101, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	rows := []xray.ClientTraffic{
		{InboundId: ib.Id, Email: "spent@x", Enable: false, Reset: 30, ResetMax: 2, ResetCount: 2, ExpiryTime: past},
		{InboundId: ib.Id, Email: "left@x", Enable: false, Reset: 30, ResetMax: 2, ResetCount: 1, ExpiryTime: past},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1: only the client with an allowance left", count)
	}

	var spent xray.ClientTraffic
	if err := db.Where("email = ?", "spent@x").First(&spent).Error; err != nil {
		t.Fatal(err)
	}
	if spent.ExpiryTime != past {
		t.Fatalf("a client that used its allowance was renewed anyway: expiry %d", spent.ExpiryTime)
	}

	var left xray.ClientTraffic
	if err := db.Where("email = ?", "left@x").First(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left.ExpiryTime <= past {
		t.Fatal("a client with an allowance left was not renewed")
	}
	if left.ResetCount != 2 {
		t.Fatalf("reset count = %d after one renewal, want 2", left.ResetCount)
	}
}

// Catching up several missed periods spends one allowance per period: a client
// that was away for three cycles must not receive three of them for free.
func TestAutoRenewClients_CatchUpSpendsOneAllowancePerPeriod(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	// Three whole 30-day periods behind.
	past := time.Now().Add(-95 * 24 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "away@x", ID: "33333333-3333-3333-3333-333333333333", Enable: false, Reset: 30, ResetMax: 2, ExpiryTime: past},
	}
	ib := mkInbound(t, 30102, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "away@x", Enable: false, Reset: 30, ResetMax: 2, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "away@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ResetCount != 2 {
		t.Fatalf("reset count = %d, want the 2 the cap allowed", row.ResetCount)
	}
	// Two periods granted, three needed: the client stays expired rather than
	// silently receiving the third.
	want := past + 2*30*86400000
	if row.ExpiryTime != want {
		t.Fatalf("expiry = %d, want %d: exactly the periods the cap paid for", row.ExpiryTime, want)
	}
	if row.ExpiryTime > time.Now().UnixMilli() {
		t.Fatal("the capped catch-up handed out a future expiry it had not paid for")
	}
}

// No cap set is the existing behaviour: renew for as long as the client keeps
// expiring.
func TestAutoRenewClients_NoCapRenewsAsBefore(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "forever@x", ID: "44444444-4444-4444-4444-444444444444", Enable: false, Reset: 30, ExpiryTime: past},
	}
	ib := mkInbound(t, 30103, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "forever@x", Enable: false, Reset: 30, ResetCount: 99, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1: a client without a cap keeps renewing", count)
	}
}
