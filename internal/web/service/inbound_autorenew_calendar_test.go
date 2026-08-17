package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// pinPanelZone fixes the panel time zone so the assertions below can talk about
// calendar days without the test machine's own zone shifting them.
func pinPanelZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("zone database unavailable: %v", err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: "timeLocation", Value: name}).Error; err != nil {
		t.Fatalf("pin panel zone: %v", err)
	}
	return loc
}

// Calendar mode renews on the same day each month. The interval mode drifts —
// 30 days from 31 January is 2 March — which is the whole reason for the mode.
func TestAutoRenewClients_CalendarModeLandsOnTheBillingDay(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()
	zone := pinPanelZone(t, "UTC")

	// Expired two calendar months ago, billed on the 15th.
	past := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
	clients := []model.Client{
		{Email: "cal@x", ID: "11111111-1111-1111-1111-111111111111", Enable: false, ResetDay: 15, ExpiryTime: past.UnixMilli()},
	}
	ib := mkInbound(t, 30201, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "cal@x", Enable: false, Up: 5, Down: 6,
		ResetDay: 15, ExpiryTime: past.UnixMilli(),
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1", count)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "cal@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	got := time.UnixMilli(row.ExpiryTime).In(zone)
	if got.Day() != 15 {
		t.Fatalf("renewed to %s, want the 15th: calendar mode must not drift", got.Format(time.RFC3339))
	}
	if !got.After(time.Now()) {
		t.Fatalf("renewed to %s, which is not in the future", got.Format(time.RFC3339))
	}
	if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
		t.Fatalf("renewed to %02d:%02d:%02d, want midnight", h, m, s)
	}
	if row.Up != 0 || row.Down != 0 {
		t.Fatalf("counters not reset: up=%d down=%d", row.Up, row.Down)
	}
	if !row.Enable {
		t.Fatal("a renewed client must be re-enabled")
	}
}

// A client billed on the 31st keeps that day, borrowing the last day only in
// months that are too short for it.
func TestAutoRenewClients_CalendarModeClampsShortMonths(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()
	zone := pinPanelZone(t, "UTC")

	past := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	clients := []model.Client{
		{Email: "eom@x", ID: "22222222-2222-2222-2222-222222222222", Enable: false, ResetDay: 31, ExpiryTime: past.UnixMilli()},
	}
	ib := mkInbound(t, 30202, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "eom@x", Enable: false, ResetDay: 31, ExpiryTime: past.UnixMilli(),
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "eom@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	got := time.UnixMilli(row.ExpiryTime).In(zone)
	last := daysInMonth(got.Year(), got.Month())
	want := 31
	if last < 31 {
		want = last
	}
	if got.Day() != want {
		t.Fatalf("renewed to day %d of %s, want %d", got.Day(), got.Month(), want)
	}
}

// Interval clients must be untouched by the new field.
func TestAutoRenewClients_IntervalModeUnchanged(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "days@x", ID: "33333333-3333-3333-3333-333333333333", Enable: false, Reset: 30, ExpiryTime: past},
	}
	ib := mkInbound(t, 30203, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "days@x", Enable: false, Reset: 30, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1", count)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "days@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if want := past + 30*86400000; row.ExpiryTime != want {
		t.Fatalf("interval renewal moved to %d, want the old fixed step %d", row.ExpiryTime, want)
	}
}

// The selection filter is what keeps a row with neither mode configured out of
// the renewal loop. That matters more than it looks: the interval step is
// reset*24h, so a zero interval reaching that loop would spin forever on the
// single traffic writer. The guard in the loop is a second line of defence and
// is deliberately unreachable while this filter holds.
func TestAutoRenewClients_RowWithNoModeIsNotSelected(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "none@x", ID: "44444444-4444-4444-4444-444444444444", Enable: false, ExpiryTime: past},
	}
	ib := mkInbound(t, 30204, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	// Seeded straight into the table with both modes off, the shape the
	// selection filter is supposed to exclude.
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "none@x", Enable: false, Reset: 0, ResetDay: 0, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
			t.Errorf("autoRenewClients: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("autoRenewClients did not return: a row with no renewal mode reached the interval loop")
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "none@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ExpiryTime != past {
		t.Fatalf("a client with no renewal mode was renewed to %d", row.ExpiryTime)
	}
}
