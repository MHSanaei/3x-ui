package service

import (
	"encoding/json"
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
	want := firstBillingMidnightAfter(t, time.Now().In(zone), 31, zone)
	if !got.Equal(want) {
		t.Fatalf("renewed to %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

// Deliberately not built on nextCalendarRenewal: it walks a day at a time and
// derives month length from time.Date's own zero-day trick, so it can disagree.
func firstBillingMidnightAfter(t *testing.T, from time.Time, day int, loc *time.Location) time.Time {
	t.Helper()
	cur := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	for i := 0; i < 400; i++ {
		cur = cur.AddDate(0, 0, 1)
		want := day
		if last := time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, loc).Day(); want > last {
			want = last
		}
		if cur.Day() == want {
			return cur
		}
	}
	t.Fatalf("no billing midnight for day %d within a year of %s", day, from)
	return time.Time{}
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

	// Asserted against the query rather than by running the loop: the failure
	// this guards is a hang, and a stuck goroutine outlives the test's DB.
	var selected int64
	if err := db.Model(&xray.ClientTraffic{}).
		Where("(reset > 0 or reset_day > 0) and expiry_time > 0 and expiry_time <= ?", time.Now().UnixMilli()).
		Where("email = ?", "none@x").
		Count(&selected).Error; err != nil {
		t.Fatal(err)
	}
	if selected != 0 {
		t.Fatal("a row with no renewal mode was selected for renewal: it would reach the interval loop and spin forever")
	}

	if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 0 {
		t.Fatalf("renewed count = %d, want 0", count)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "none@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ExpiryTime != past {
		t.Fatalf("a client with no renewal mode was renewed to %d", row.ExpiryTime)
	}
}

// The billing day has to survive the clients table, not just the settings JSON:
// an ordinary edit rebuilds the client from the record and writes it back (#6106).
func TestClientEditKeepsTheBillingDay(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "keep@x", ID: "55555555-5555-5555-5555-555555555555", Enable: true, ResetDay: 20, ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli()},
	}
	ib := mkInbound(t, 30205, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	mkTraffic(t, ib.Id, "keep@x", 10, 20, 0, 0, true)

	rec, err := svc.clientService.GetRecordByEmail(nil, "keep@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.ResetDay != 20 {
		t.Fatalf("clients.reset_day = %d, want the 20 the client was created with", rec.ResetDay)
	}

	// What the edit dialog does: hydrate the record, change something else, save.
	edited := rec.ToClient()
	edited.Comment = "renamed"
	if _, err := svc.clientService.Update(svc, rec.Id, *edited, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// The inbound JSON is what xray and the edit dialog read back, and it is
	// rebuilt from the record, so it is where a dropped converter field shows.
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
	if settings.Clients[0].ResetDay != 20 {
		t.Fatalf("inbound settings resetDay = %d after an unrelated edit, want 20: calendar mode was silently turned off", settings.Clients[0].ResetDay)
	}

	rec, err = svc.clientService.GetRecordByEmail(nil, "keep@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail after edit: %v", err)
	}
	if rec.ResetDay != 20 {
		t.Fatalf("clients.reset_day = %d after an unrelated edit, want 20", rec.ResetDay)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "keep@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ResetDay != 20 {
		t.Fatalf("client_traffics.reset_day = %d after an unrelated edit, want 20", row.ResetDay)
	}
}

// The billing day is useless if it can only be chosen once. The test above
// passes even without the record write, because nothing overwrites the value
// it checks; this one fails without it.
func TestClientEditChangesTheBillingDay(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}

	clients := []model.Client{
		{
			Email: "chg@x", ID: "77777777-7777-7777-7777-777777777777", Enable: true, ResetDay: 20,
			ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli(),
		},
	}
	ib := mkInbound(t, 30206, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	mkTraffic(t, ib.Id, "chg@x", 0, 0, 0, 0, true)

	rec, err := svc.clientService.GetRecordByEmail(nil, "chg@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	edited := rec.ToClient()
	edited.ResetDay = 5
	if _, err := svc.clientService.Update(svc, rec.Id, *edited, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	rec, err = svc.clientService.GetRecordByEmail(nil, "chg@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail after edit: %v", err)
	}
	if rec.ResetDay != 5 {
		t.Fatalf("clients.reset_day = %d after the operator moved the billing day to the 5th", rec.ResetDay)
	}

	// Turning calendar mode off has to work too.
	edited = rec.ToClient()
	edited.ResetDay = 0
	edited.Reset = 30
	if _, err := svc.clientService.Update(svc, rec.Id, *edited, rec.LimitHwid); err != nil {
		t.Fatalf("Update back to interval mode: %v", err)
	}
	rec, err = svc.clientService.GetRecordByEmail(nil, "chg@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail after switching mode: %v", err)
	}
	if rec.ResetDay != 0 {
		t.Fatalf("clients.reset_day = %d after the operator switched back to interval mode", rec.ResetDay)
	}
}

// The two renewal features meet here: a calendar client is capped like an
// interval one, spending one allowance per month rather than per tick.
func TestAutoRenewClients_CalendarModeSpendsOneAllowancePerMonth(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()
	zone := pinPanelZone(t, "UTC")

	// Three calendar months behind with one allowance left: a single month step
	// cannot reach the present, so the client stays expired on its billing day.
	past := time.Now().In(zone).AddDate(0, -3, 0)
	past = time.Date(past.Year(), past.Month(), 10, 0, 0, 0, 0, zone)
	clients := []model.Client{
		{Email: "calcap@x", ID: "22222222-2222-2222-2222-222222222222", Enable: false, ResetDay: 10, ResetMax: 3, ExpiryTime: past.UnixMilli()},
	}
	ib := mkInbound(t, 30205, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "calcap@x", Enable: false, ResetDay: 10, ResetMax: 3, ResetCount: 2,
		Up: 111, Down: 222, ExpiryTime: past.UnixMilli(),
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "calcap@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	got := time.UnixMilli(row.ExpiryTime).In(zone)
	if want := past.AddDate(0, 1, 0); !got.Equal(want) {
		t.Fatalf("renewed to %s, want exactly one month on to %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if row.ResetCount != 3 {
		t.Fatalf("resetCount = %d, want 3: one allowance per month stepped", row.ResetCount)
	}
	if row.Enable {
		t.Fatal("a client still expired after a truncated catch-up was enabled")
	}
	if row.Up != 111 || row.Down != 222 {
		t.Fatalf("counters zeroed for a month the client can never use: up=%d down=%d", row.Up, row.Down)
	}
}
