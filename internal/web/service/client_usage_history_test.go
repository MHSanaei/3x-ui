package service

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedUsageHistoryClient seeds a local VLESS inbound hosting one client and a
// live traffic row with the given counters, ready for a reset/renew/delete.
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

// A single-client reset must archive the counters into a closed window, zero
// the live row, and leave lifetime = archived + live in GetUsageHistory.
func TestResetArchivesClosedWindow(t *testing.T) {
	inboundSvc := &InboundService{}
	ib := seedUsageHistoryClient(t, inboundSvc, "reset-win@example.test", "44444444-1111-4111-8111-111111111111", 100, 200)

	if _, err := inboundSvc.ResetClientTraffic(ib.Id, "reset-win@example.test"); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}

	var live xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "reset-win@example.test").First(&live).Error; err != nil {
		t.Fatalf("read live traffic row: %v", err)
	}
	if live.Up != 0 || live.Down != 0 {
		t.Fatalf("live counters = %d/%d, want zeroed", live.Up, live.Down)
	}
	windows := windowsOf(t, "reset-win@example.test")
	if len(windows) != 1 || windows[0].Reason != trafficWindowReset || windows[0].Up != 100 || windows[0].Down != 200 {
		t.Fatalf("windows = %+v, want one reset window 100/200", windows)
	}

	history, err := (&ClientService{}).GetUsageHistory("reset-win@example.test")
	if err != nil {
		t.Fatalf("GetUsageHistory: %v", err)
	}
	if history.LifetimeUp != 100 || history.LifetimeDown != 200 {
		t.Fatalf("lifetime = %d/%d, want 100/200", history.LifetimeUp, history.LifetimeDown)
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

// Deleting a client's stats archives its counters and logs a "deleted" event.
func TestDelClientStatArchivesDeletedWindow(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "del-win@example.test", "88888888-5555-4555-8555-555555555555", 50, 25)

	db := database.GetDB()
	if err := db.Transaction(func(tx *gorm.DB) error {
		return inboundSvc.DelClientStat(tx, "del-win@example.test", trafficWindowDelete)
	}); err != nil {
		t.Fatalf("DelClientStat: %v", err)
	}

	var liveCount int64
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "del-win@example.test").Count(&liveCount).Error; err != nil {
		t.Fatalf("count live rows: %v", err)
	}
	if liveCount != 0 {
		t.Fatalf("live rows = %d, want 0", liveCount)
	}
	windows := windowsOf(t, "del-win@example.test")
	if len(windows) != 1 || windows[0].Reason != trafficWindowDelete || windows[0].Up != 50 || windows[0].Down != 25 {
		t.Fatalf("windows = %+v, want one deleted window 50/25", windows)
	}
	changes := changesOf(t, "del-win@example.test")
	if len(changes) != 1 || changes[0].Field != "deleted" {
		t.Fatalf("changes = %+v, want one deleted event", changes)
	}

	// The lifetime read still accounts for the archived window with no live row.
	history, err := (&ClientService{}).GetUsageHistory("del-win@example.test")
	if err != nil {
		t.Fatalf("GetUsageHistory: %v", err)
	}
	if history.LifetimeUp != 50 || history.LifetimeDown != 25 {
		t.Fatalf("lifetime = %d/%d, want 50/25", history.LifetimeUp, history.LifetimeDown)
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

// Renaming a client carries its windows and change log onto the new email,
// and records the rename itself once.
func TestRenameMigratesClientHistory(t *testing.T) {
	inboundSvc := &InboundService{}
	ib := seedUsageHistoryClient(t, inboundSvc, "old-name@example.test", "cccccccc-9999-4999-8999-999999999999", 7, 9)
	svc := &ClientService{}

	// Close one window so there is history to migrate.
	if _, err := inboundSvc.ResetClientTraffic(ib.Id, "old-name@example.test"); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}

	// Rename through the inbound apply path: the settings now carry the new
	// email, the old one is gone.
	rec, err := svc.GetRecordByEmail(nil, "old-name@example.test")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	renamed := rec.ToClient()
	renamed.Email = "new-name@example.test"
	if _, err := svc.Update(inboundSvc, rec.Id, *renamed, rec.LimitHwid); err != nil {
		t.Fatalf("rename Update: %v", err)
	}

	if got := len(windowsOf(t, "old-name@example.test")); got != 0 {
		t.Fatalf("old email still has %d windows, want migrated", got)
	}
	windows := windowsOf(t, "new-name@example.test")
	if len(windows) != 1 || windows[0].Reason != trafficWindowReset || windows[0].Up != 7 {
		t.Fatalf("new email windows = %+v, want the migrated reset window", windows)
	}
	names := 0
	for _, c := range changesOf(t, "new-name@example.test") {
		if c.Field == "name" && c.OldValue == "old-name@example.test" && c.NewValue == "new-name@example.test" {
			names++
		}
	}
	if names != 1 {
		t.Fatalf("name change rows = %d, want exactly 1", names)
	}
}

// A client with no windows, nodes or changes reads back as all zeros and
// empty (non-nil) slices.
func TestGetUsageHistoryEmpty(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "fresh@example.test", "dddddddd-aaaa-4aaa-8aaa-aaaaaaaaaaaa", 0, 0)

	history, err := (&ClientService{}).GetUsageHistory("fresh@example.test")
	if err != nil {
		t.Fatalf("GetUsageHistory: %v", err)
	}
	if history.LifetimeUp != 0 || history.LifetimeDown != 0 || history.CurrentUp != 0 || history.CurrentDown != 0 {
		t.Fatalf("totals = %+v, want all zero", history)
	}
	if len(history.Windows) != 0 || len(history.Nodes) != 0 || len(history.Changes) != 0 {
		t.Fatalf("collections not empty: %+v", history)
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

func TestGetUsageHistoryJSONShape(t *testing.T) {
	inboundSvc := &InboundService{}
	seedUsageHistoryClient(t, inboundSvc, "shape@example.test", "ffffffff-cccc-4ccc-8ccc-cccccccccccc", 0, 0)

	history, err := (&ClientService{}).GetUsageHistory("shape@example.test")
	if err != nil {
		t.Fatalf("GetUsageHistory: %v", err)
	}
	b, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"email", "lifetimeUp", "lifetimeDown", "currentUp", "currentDown", "windows", "nodes", "changes"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("JSON missing %q: %s", key, b)
		}
	}
}

// itoa is a tiny strconv alias kept local so the table above stays readable.
func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
