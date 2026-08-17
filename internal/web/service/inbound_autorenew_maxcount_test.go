package service

import (
	"encoding/json"
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

// The cap has to survive the clients table, not just the settings JSON: an
// ordinary edit rebuilds the client from the record and writes it back (#5804).
func TestClientEditKeepsTheRenewalCap(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "cap@x", ID: "44444444-4444-4444-4444-444444444444", Enable: true, Reset: 30, ResetMax: 3, ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli()},
	}
	ib := mkInbound(t, 30104, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	mkTraffic(t, ib.Id, "cap@x", 10, 20, 0, 0, true)

	rec, err := svc.clientService.GetRecordByEmail(nil, "cap@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.ResetMax != 3 {
		t.Fatalf("clients.reset_max = %d, want the 3 the client was created with", rec.ResetMax)
	}

	// What the edit dialog does: hydrate the record, change something else, save.
	edited := rec.ToClient()
	edited.Comment = "renamed"
	if _, err := svc.clientService.Update(svc, rec.Id, *edited, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var stored model.Inbound
	if err := db.Where("id = ?", ib.Id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(stored.Settings), &settings); err != nil {
		t.Fatalf("parse inbound settings: %v", err)
	}
	if len(settings.Clients) != 1 {
		t.Fatalf("inbound holds %d clients, want 1", len(settings.Clients))
	}
	if settings.Clients[0].ResetMax != 3 {
		t.Fatalf("inbound settings resetMax = %d after an unrelated edit, want 3: the cap was silently lifted", settings.Clients[0].ResetMax)
	}

	rec, err = svc.clientService.GetRecordByEmail(nil, "cap@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail after edit: %v", err)
	}
	if rec.ResetMax != 3 {
		t.Fatalf("clients.reset_max = %d after an unrelated edit, want 3", rec.ResetMax)
	}
}

// A cap that runs out mid-catch-up leaves the client expired, so the renewal
// side effects must not fire: disableInvalidClients would undo them at once.
func TestAutoRenewClients_TruncatedCatchUpLeavesTheClientDisabled(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	// Five periods behind with one allowance left: one 30-day step cannot reach
	// the present, so the client stays expired.
	past := time.Now().Add(-150 * 24 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "short@x", ID: "55555555-5555-5555-5555-555555555555", Enable: false, Reset: 30, ResetMax: 3, ExpiryTime: past},
	}
	ib := mkInbound(t, 30105, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "short@x", Enable: false, Reset: 30, ResetMax: 3, ResetCount: 2,
		Up: 111, Down: 222, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "short@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ExpiryTime >= time.Now().UnixMilli() {
		t.Fatalf("expiry %d reached the present: the cap did not truncate the catch-up", row.ExpiryTime)
	}
	if row.Enable {
		t.Fatal("a client still expired after a truncated catch-up was enabled: xray gains a user only to lose it again")
	}
	if row.Up != 111 || row.Down != 222 {
		t.Fatalf("counters zeroed for periods the client can never use: up=%d down=%d", row.Up, row.Down)
	}
}
