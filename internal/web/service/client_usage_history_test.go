package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedUsageHistoryClient seeds a local VLESS inbound hosting one client and a
// live traffic row with the given counters.
func seedUsageHistoryClient(t *testing.T, inboundSvc *InboundService, email, uid string, up, down int64) *model.Inbound {
	t.Helper()
	setupBulkDB(t)
	svc := &ClientService{}

	client := model.Client{Email: email, ID: uid, SubID: email, Enable: true}
	ib := mkInbound(t, 54501, model.VLESS, clientsSettings(t, []model.Client{client}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	db := database.GetDB()
	traffic := xray.ClientTraffic{
		InboundId:  ib.Id,
		Email:      email,
		Enable:     true,
		Up:         up,
		Down:       down,
		ExpiryTime: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		Total:      1 << 40,
	}
	if err := db.Create(&traffic).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	return ib
}

func windowsOf(t *testing.T, email string) []model.ClientTrafficWindow {
	t.Helper()
	var windows []model.ClientTrafficWindow
	if err := database.GetDB().Where("email = ?", email).Order("id").Find(&windows).Error; err != nil {
		t.Fatalf("read windows: %v", err)
	}
	return windows
}

func changesOf(t *testing.T, email string) []model.ClientChangeLog {
	t.Helper()
	var changes []model.ClientChangeLog
	if err := database.GetDB().Where("email = ?", email).Order("id").Find(&changes).Error; err != nil {
		t.Fatalf("read change log: %v", err)
	}
	return changes
}

// Lifetime traffic is the archived windows plus the live counters, so a reset path
// that zeroes without archiving silently loses that cycle from the total.
func TestEveryResetPathArchivesTheWindowItCloses(t *testing.T) {
	const email = "reset-win@example.test"
	for _, tc := range []struct {
		name  string
		reset func(*InboundService, *ClientService, *model.Inbound) error
	}{
		{"one client of an inbound", func(inbounds *InboundService, _ *ClientService, ib *model.Inbound) error {
			_, err := inbounds.ResetClientTraffic(ib.Id, email)
			return err
		}},
		{"one client by email", func(inbounds *InboundService, clients *ClientService, _ *model.Inbound) error {
			_, err := clients.ResetTrafficByEmail(inbounds, email)
			return err
		}},
		{"one client's counters only", func(inbounds *InboundService, _ *ClientService, _ *model.Inbound) error {
			return inbounds.ResetClientTrafficByEmail(email)
		}},
		{"selected clients", func(inbounds *InboundService, clients *ClientService, _ *model.Inbound) error {
			_, err := clients.BulkResetTraffic(inbounds, []string{email})
			return err
		}},
		{"every client of an inbound", func(inbounds *InboundService, clients *ClientService, ib *model.Inbound) error {
			_, err := clients.ResetAllClientTraffics(inbounds, ib.Id)
			return err
		}},
		{"every client", func(_ *InboundService, clients *ClientService, _ *model.Inbound) error {
			_, err := clients.ResetAllTraffics()
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbounds, clients := &InboundService{}, &ClientService{}
			ib := seedUsageHistoryClient(t, inbounds, email, "44444444-1111-4111-8111-111111111111", 100, 200)
			if err := tc.reset(inbounds, clients, ib); err != nil {
				t.Fatal(err)
			}
			windows := windowsOf(t, email)
			if len(windows) != 1 || windows[0].Reason != trafficWindowReset || windows[0].Up != 100 || windows[0].Down != 200 {
				t.Fatalf("windows = %+v, want exactly one reset window of 100/200", windows)
			}
			history, err := clients.GetUsageHistory(email)
			if err != nil {
				t.Fatal(err)
			}
			if history.CurrentUp != 0 || history.CurrentDown != 0 || history.LifetimeUp != 100 || history.LifetimeDown != 200 {
				t.Fatalf("current %d/%d lifetime %d/%d, want 0/0 and 100/200",
					history.CurrentUp, history.CurrentDown, history.LifetimeUp, history.LifetimeDown)
			}
		})
	}
}

// Past the cap a client's oldest windows fold into one row: the table stops growing and
// the lifetime total must not move. At the cap itself nothing is folded yet.
func TestWindowsPastTheCapFoldWithoutChangingLifetime(t *testing.T) {
	const email = "capped@example.test"
	inbounds, clients := &InboundService{}, &ClientService{}
	ib := seedUsageHistoryClient(t, inbounds, email, "12121212-1111-4111-8111-111111111111", 100, 200)
	db := database.GetDB()
	seeded := make([]model.ClientTrafficWindow, 0, clientTrafficWindowKeep)
	for i := range clientTrafficWindowKeep {
		seeded = append(seeded, model.ClientTrafficWindow{Email: email, At: int64(1000 + i), Up: 1, Down: 2, Reason: trafficWindowRenew})
	}
	if err := db.Create(&seeded).Error; err != nil {
		t.Fatal(err)
	}

	lifetimeUp := int64(clientTrafficWindowKeep)
	for cycle, tc := range []struct {
		usedUp, usedDown     int64
		folded               int
		foldedUp, foldedDown int64
	}{
		{100, 200, 0, 0, 0},
		{7, 9, 1, 2, 4},
		{3, 1, 1, 3, 6},
	} {
		if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).
			Updates(map[string]any{"up": tc.usedUp, "down": tc.usedDown}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := inbounds.ResetClientTraffic(ib.Id, email); err != nil {
			t.Fatal(err)
		}
		lifetimeUp += tc.usedUp
		windows := windowsOf(t, email)
		var folded []model.ClientTrafficWindow
		for _, window := range windows {
			if window.Reason == trafficWindowEarlier {
				folded = append(folded, window)
			}
		}
		if len(windows) != clientTrafficWindowKeep+1 || len(folded) != tc.folded ||
			(tc.folded == 1 && (folded[0].Up != tc.foldedUp || folded[0].Down != tc.foldedDown)) {
			t.Fatalf("cycle %d: %d windows, folded %+v; want %d windows with %d earlier row(s) of %d/%d",
				cycle, len(windows), folded, clientTrafficWindowKeep+1, tc.folded, tc.foldedUp, tc.foldedDown)
		}
		history, err := clients.GetUsageHistory(email)
		if err != nil {
			t.Fatal(err)
		}
		if history.LifetimeUp != lifetimeUp {
			t.Fatalf("cycle %d: lifetime up = %d, want %d: folding changed the total", cycle, history.LifetimeUp, lifetimeUp)
		}
	}
}

// Lifetime is what the client used in total, so the cycle still open counts too.
func TestLifetimeCountsTheOpenWindow(t *testing.T) {
	const email = "open-win@example.test"
	inbounds, clients := &InboundService{}, &ClientService{}
	ib := seedUsageHistoryClient(t, inbounds, email, "17171717-1111-4111-8111-111111111111", 100, 200)
	if _, err := inbounds.ResetClientTraffic(ib.Id, email); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"up": 7, "down": 9}).Error; err != nil {
		t.Fatal(err)
	}
	history, err := clients.GetUsageHistory(email)
	if err != nil {
		t.Fatal(err)
	}
	if history.CurrentUp != 7 || history.CurrentDown != 9 || history.LifetimeUp != 107 || history.LifetimeDown != 209 {
		t.Fatalf("current %d/%d lifetime %d/%d, want 7/9 and 107/209",
			history.CurrentUp, history.CurrentDown, history.LifetimeUp, history.LifetimeDown)
	}
}

// The change log keeps the newest rows of each client and never trims another client's.
func TestChangeLogKeepsTheNewestRowsPerClient(t *testing.T) {
	const email, other = "chatty@example.test", "quiet@example.test"
	inbounds, clients := &InboundService{}, &ClientService{}
	seedUsageHistoryClient(t, inbounds, email, "13131313-1111-4111-8111-111111111111", 0, 0)
	db := database.GetDB()
	seeded := make([]model.ClientChangeLog, 0, clientChangeLogKeep+1)
	for i := range clientChangeLogKeep {
		seeded = append(seeded, model.ClientChangeLog{Email: email, At: int64(1000 + i), Field: "enable", NewValue: itoa(int64(i))})
	}
	seeded = append(seeded, model.ClientChangeLog{Email: other, At: 1, Field: "enable"})
	if err := db.Create(&seeded).Error; err != nil {
		t.Fatal(err)
	}

	record, err := clients.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatal(err)
	}
	edited := record.ToClient()
	edited.TotalGB = 5 << 30
	if _, err := clients.Update(inbounds, record.Id, *edited, record.LimitHwid); err != nil {
		t.Fatal(err)
	}

	kept := changesOf(t, email)
	if len(kept) != clientChangeLogKeep || kept[0].NewValue != "1" || kept[len(kept)-1].Field != "dataLimit" {
		t.Fatalf("%d rows, first %+v last %+v; want %d rows from the second seeded one to the new dataLimit change",
			len(kept), kept[0], kept[len(kept)-1], clientChangeLogKeep)
	}
	if got := len(changesOf(t, other)); got != 1 {
		t.Fatalf("another client's log has %d rows, want its 1 untouched", got)
	}
}

// A bot edit and a bulk edit are logged from the stored record, under their own source.
func TestBotAndBulkEditsAreLoggedWithTheirSource(t *testing.T) {
	const email = "edited@example.test"
	inbounds, clients := &InboundService{}, &ClientService{}
	seedUsageHistoryClient(t, inbounds, email, "14141414-1111-4111-8111-111111111111", 0, 0)

	// The bulk add skips an unlimited client, so the bot sets a limit first.
	if _, err := clients.ResetClientTrafficLimitByEmail(inbounds, email, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := clients.BulkAdjust(inbounds, []string{email}, 0, 1<<30, "", nil, ""); err != nil {
		t.Fatal(err)
	}

	type row struct{ field, old, new, source string }
	var got []row
	for _, change := range changesOf(t, email) {
		got = append(got, row{change.Field, change.OldValue, change.NewValue, change.Source})
	}
	want := []row{
		{"dataLimit", "0", itoa(1 << 30), clientChangeSourceTgbot},
		{"dataLimit", itoa(1 << 30), itoa(2 << 30), clientChangeSourcePanel},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("change log = %+v, want %+v", got, want)
	}
}

// History belongs to a client: once the client is gone the reaper drops it, and only it.
func TestHistoryOfADeletedClientIsReaped(t *testing.T) {
	const gone, kept = "gone@example.test", "kept@example.test"
	inbounds, clients := &InboundService{}, &ClientService{}
	seedUsageHistoryClient(t, inbounds, gone, "15151515-1111-4111-8111-111111111111", 0, 0)
	stays := model.Client{Email: kept, ID: "16161616-1111-4111-8111-111111111111", SubID: kept, Enable: true}
	ib := mkInbound(t, 54506, model.VLESS, clientsSettings(t, []model.Client{stays}))
	if err := clients.SyncInbound(nil, ib.Id, []model.Client{stays}); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	for _, email := range []string{gone, kept} {
		if err := db.Create(&model.ClientTrafficWindow{Email: email, At: 1, Up: 5, Reason: trafficWindowReset}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ClientChangeLog{Email: email, At: 1, Field: "enable"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	record, err := clients.GetRecordByEmail(nil, gone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Delete(inbounds, record.Id, false); err != nil {
		t.Fatal(err)
	}

	reaped, err := clients.ReapOrphanClientHistory()
	if err != nil || reaped != 2 {
		t.Fatalf("reaped %d rows (err %v), want the deleted client's window and change", reaped, err)
	}
	if w, c := len(windowsOf(t, gone)), len(changesOf(t, gone)); w != 0 || c != 0 {
		t.Fatalf("deleted client still has %d windows and %d changes", w, c)
	}
	if w, c := len(windowsOf(t, kept)), len(changesOf(t, kept)); w != 1 || c != 1 {
		t.Fatalf("living client has %d windows and %d changes, want 1 and 1", w, c)
	}
}

// The orphan list is read off the writer. A client that takes one of the listed emails
// before the delete runs owns that history, renamed-in rows included.
func TestReapSparesAnEmailClaimedAfterItWasListed(t *testing.T) {
	const claimed, held, orphan = "claimed@example.test", "held@example.test", "orphan@example.test"
	setupBulkDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientRecord{Email: held, SubID: held, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{claimed, held, orphan} {
		if err := db.Create(&model.ClientTrafficWindow{Email: email, At: 1, Up: 5, Reason: trafficWindowReset}).Error; err != nil {
			t.Fatal(err)
		}
	}
	listed, err := orphanHistoryEmails(db, "client_traffic_windows")
	if err != nil || len(listed) != 2 || listed[0] != claimed || listed[1] != orphan {
		t.Fatalf("listed %v (err %v), want the two client-less emails and not the held one", listed, err)
	}
	if err := db.Create(&model.ClientRecord{Email: claimed, SubID: claimed, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	dropped, err := deleteOrphanHistory(&model.ClientTrafficWindow{}, listed)
	if err != nil || dropped != 1 {
		t.Fatalf("dropped %d rows (err %v), want only the orphan's window", dropped, err)
	}
	if kept, gone := len(windowsOf(t, claimed)), len(windowsOf(t, orphan)); kept != 1 || gone != 0 {
		t.Fatalf("claimed email has %d windows and the orphan %d, want 1 and 0", kept, gone)
	}
}

// Bulk reset archives only the rows that actually carried usage.
func TestBulkResetArchivesUsedWindowsOnly(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "bulk-used@example.test", "55555555-2222-4222-8222-222222222222", 10, 5)
	svc := &ClientService{}

	// A second zero-usage client on its own inbound.
	client2 := model.Client{Email: "bulk-idle@example.test", ID: "66666666-3333-4333-8333-333333333333", SubID: "bulk-idle@example.test", Enable: true}
	ib2 := mkInbound(t, 54502, model.VLESS, clientsSettings(t, []model.Client{client2}))
	if err := svc.SyncInbound(nil, ib2.Id, []model.Client{client2}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	idle := xray.ClientTraffic{InboundId: ib2.Id, Email: "bulk-idle@example.test", Enable: true}
	if err := database.GetDB().Create(&idle).Error; err != nil {
		t.Fatalf("seed idle client_traffics: %v", err)
	}

	if _, err := svc.BulkResetTraffic(inboundSvc, []string{"bulk-used@example.test", "bulk-idle@example.test"}); err != nil {
		t.Fatalf("BulkResetTraffic: %v", err)
	}
	if got := len(windowsOf(t, "bulk-used@example.test")); got != 1 {
		t.Fatalf("used client windows = %d, want 1", got)
	}
	if got := len(windowsOf(t, "bulk-idle@example.test")); got != 0 {
		t.Fatalf("idle client windows = %d, want 0", got)
	}
}

// An auto-renewal must archive the closed quota window under reason "renew"
// and log the expiry change under source "auto-renew".
func TestAutoRenewArchivesRenewWindowAndLogsExpiry(t *testing.T) {
	inboundSvc := &InboundService{}
	svc := &ClientService{}
	setupBulkDB(t)

	const email = "renew-win@example.test"
	past := time.Now().Add(-24 * time.Hour).UnixMilli()
	client := model.Client{Email: email, ID: "77777777-4444-4444-8444-444444444444", SubID: email, Enable: true, Reset: 30}
	ib := mkInbound(t, 54503, model.VLESS, clientsSettings(t, []model.Client{client}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	db := database.GetDB()
	traffic := xray.ClientTraffic{
		InboundId: ib.Id, Email: email, Enable: true,
		Up: 300, Down: 100, Reset: 30, ExpiryTime: past, Total: 1 << 40,
	}
	if err := db.Create(&traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	_, count, err := inboundSvc.autoRenewClients(db, newTrafficMutationBatch())
	if err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}
	if count != 1 {
		t.Fatalf("renewed count = %d, want 1", count)
	}

	var live xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&live).Error; err != nil {
		t.Fatalf("read live traffic row: %v", err)
	}
	if live.Up != 0 || live.Down != 0 {
		t.Fatalf("renewal did not zero counters: %d/%d", live.Up, live.Down)
	}
	windows := windowsOf(t, email)
	if len(windows) != 1 || windows[0].Reason != trafficWindowRenew || windows[0].Up != 300 || windows[0].Down != 100 {
		t.Fatalf("windows = %+v, want one renew window 300/100", windows)
	}
	changes := changesOf(t, email)
	if len(changes) != 1 || changes[0].Field != "expiry" || changes[0].Source != clientChangeSourceRenew {
		t.Fatalf("changes = %+v, want one auto-renew expiry row", changes)
	}
	if changes[0].OldValue != itoa(past) || changes[0].NewValue == changes[0].OldValue {
		t.Fatalf("expiry row = %+v, want old=%d new>old", changes[0], past)
	}
}

// A save from the editor logs each field it changed and nothing else; a
// no-op save logs nothing.
func TestUpdateLogsChangedFieldsOnly(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "upd-log@example.test", "99999999-6666-4666-8666-666666666666", 0, 0)
	svc := &ClientService{}

	rec, err := svc.GetRecordByEmail(nil, "upd-log@example.test")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	newExpiry := time.Now().Add(60 * 24 * time.Hour).UnixMilli()
	updated.ExpiryTime = newExpiry
	updated.TotalGB = 5 << 30
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fields := map[string]string{}
	for _, c := range changesOf(t, "upd-log@example.test") {
		fields[c.Field] = c.NewValue
	}
	if len(fields) != 2 {
		t.Fatalf("logged fields = %v, want exactly expiry and dataLimit", fields)
	}
	if fields["expiry"] != itoa(newExpiry) || fields["dataLimit"] != itoa(int64(5<<30)) {
		t.Fatalf("field values = %v", fields)
	}
	for _, c := range changesOf(t, "upd-log@example.test") {
		if c.Source != clientChangeSourcePanel {
			t.Fatalf("source = %q, want %q", c.Source, clientChangeSourcePanel)
		}
	}

	// A no-op save must not grow the log.
	before := len(changesOf(t, "upd-log@example.test"))
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("no-op Update: %v", err)
	}
	if after := len(changesOf(t, "upd-log@example.test")); after != before {
		t.Fatalf("no-op save logged %d extra rows", after-before)
	}
}

// Bulk enable/disable logs an "enable" row only for the clients that flipped.
func TestBulkSetEnableLogsOnlyFlippedClients(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "bulk-en-a@example.test", "aaaaaaaa-7777-4777-8777-777777777777", 0, 0)
	svc := &ClientService{}
	clientB := model.Client{Email: "bulk-en-b@example.test", ID: "bbbbbbbb-8888-4888-8888-888888888888", SubID: "bulk-en-b@example.test", Enable: true}
	ib2 := mkInbound(t, 54505, model.VLESS, clientsSettings(t, []model.Client{clientB}))
	if err := svc.SyncInbound(nil, ib2.Id, []model.Client{clientB}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	if _, _, err := svc.BulkSetEnable(inboundSvc, []string{"bulk-en-a@example.test", "bulk-en-b@example.test"}, false); err != nil {
		t.Fatalf("BulkSetEnable: %v", err)
	}
	if got := len(changesOf(t, "bulk-en-a@example.test")); got != 1 {
		t.Fatalf("client A enable rows = %d, want 1", got)
	}
	if got := len(changesOf(t, "bulk-en-b@example.test")); got != 1 {
		t.Fatalf("client B enable rows = %d, want 1", got)
	}
	// Re-applying the same state must log nothing new.
	if _, _, err := svc.BulkSetEnable(inboundSvc, []string{"bulk-en-a@example.test"}, false); err != nil {
		t.Fatalf("BulkSetEnable repeat: %v", err)
	}
	if got := len(changesOf(t, "bulk-en-a@example.test")); got != 1 {
		t.Fatalf("repeat enable rows = %d, want still 1", got)
	}
}

// Renaming a client carries its windows and change log onto the new email and records
// the rename once, however many inbounds the client is on.
func TestRenameMigratesClientHistory(t *testing.T) {
	const oldEmail, newEmail = "old-name@example.test", "new-name@example.test"
	inboundSvc := &InboundService{}
	ib := seedUsageHistoryClient(t, inboundSvc, oldEmail, "cccccccc-9999-4999-8999-999999999999", 7, 9)
	svc := &ClientService{}
	same := model.Client{Email: oldEmail, ID: "cccccccc-9999-4999-8999-999999999999", SubID: oldEmail, Enable: true}
	second := mkInbound(t, 54507, model.VLESS, clientsSettings(t, []model.Client{same}))
	if err := svc.SyncInbound(nil, second.Id, []model.Client{same}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	// Close one window so there is history to migrate.
	if _, err := inboundSvc.ResetClientTraffic(ib.Id, oldEmail); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}

	rec, err := svc.GetRecordByEmail(nil, oldEmail)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	renamed := rec.ToClient()
	renamed.Email = newEmail
	renamed.TotalGB = 5 << 30
	if _, err := svc.Update(inboundSvc, rec.Id, *renamed, rec.LimitHwid); err != nil {
		t.Fatalf("rename Update: %v", err)
	}

	if w, c := len(windowsOf(t, oldEmail)), len(changesOf(t, oldEmail)); w != 0 || c != 0 {
		t.Fatalf("old email still has %d windows and %d changes, want all of it under the new one", w, c)
	}
	windows := windowsOf(t, newEmail)
	if len(windows) != 1 || windows[0].Reason != trafficWindowReset || windows[0].Up != 7 {
		t.Fatalf("new email windows = %+v, want the migrated reset window", windows)
	}
	names, limits := 0, 0
	for _, c := range changesOf(t, newEmail) {
		switch {
		case c.Field == "name" && c.OldValue == oldEmail && c.NewValue == newEmail:
			names++
		case c.Field == "dataLimit" && c.NewValue == itoa(5<<30):
			limits++
		}
	}
	if names != 1 || limits != 1 {
		t.Fatalf("new email has %d name and %d dataLimit rows, want 1 and 1", names, limits)
	}
}

// A client attached to no inbound is renamed by the record write alone; its history
// has to follow there too, or the reaper drops it with the old email.
func TestRenameOfAnUnattachedClientMigratesItsHistory(t *testing.T) {
	const oldEmail, newEmail = "loose-old@example.test", "loose-new@example.test"
	setupBulkDB(t)
	db := database.GetDB()
	rec := model.ClientRecord{Email: oldEmail, SubID: oldEmail, Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientTrafficWindow{Email: oldEmail, At: 1, Up: 5, Reason: trafficWindowReset}).Error; err != nil {
		t.Fatal(err)
	}

	renamed := rec.ToClient()
	renamed.Email = newEmail
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, *renamed, rec.LimitHwid); err != nil {
		t.Fatalf("rename Update: %v", err)
	}

	if old, moved := len(windowsOf(t, oldEmail)), len(windowsOf(t, newEmail)); old != 0 || moved != 1 {
		t.Fatalf("windows: %d under the old email and %d under the new, want 0 and 1", old, moved)
	}
	changes := changesOf(t, newEmail)
	if len(changes) != 1 || changes[0].Field != "name" || changes[0].OldValue != oldEmail || changes[0].NewValue != newEmail {
		t.Fatalf("change log = %+v, want the one rename row", changes)
	}
}

// GetUsageHistory must report the node's own accumulated up/down, separated.
func TestGetUsageHistoryReportsNodeUsage(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "node-usage@example.test", "eeeeeeee-bbbb-4bbb-8bbb-bbbbbbbbbbbb", 0, 0)
	db := database.GetDB()

	node := &model.Node{
		Name:    "fra-edge",
		Address: "fra.example.com",
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nodeRow := &model.NodeClientTraffic{NodeId: node.Id, Email: "node-usage@example.test", Up: 111, Down: 222}
	if err := db.Create(nodeRow).Error; err != nil {
		t.Fatalf("create node traffic: %v", err)
	}

	history, err := (&ClientService{}).GetUsageHistory("node-usage@example.test")
	if err != nil {
		t.Fatalf("GetUsageHistory: %v", err)
	}
	if len(history.Nodes) != 1 {
		t.Fatalf("nodes = %+v, want one", history.Nodes)
	}
	if history.Nodes[0].NodeName != "fra-edge" || history.Nodes[0].Up != 111 || history.Nodes[0].Down != 222 {
		t.Fatalf("node usage = %+v, want fra-edge 111/222", history.Nodes[0])
	}
}

// itoa is a tiny strconv alias kept local so the table above stays readable.
func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
