package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedGlobalTraffic(t *testing.T, email string, up, down int64) {
	t.Helper()
	if err := database.GetDB().Create(&model.ClientGlobalTraffic{
		MasterGuid: "peer-" + email, Email: email, Up: up, Down: down,
	}).Error; err != nil {
		t.Fatalf("seed client_global_traffics: %v", err)
	}
}

func countGlobalTraffic(t *testing.T, email string) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(&model.ClientGlobalTraffic{}).
		Where("email = ?", email).Count(&n).Error; err != nil {
		t.Fatalf("count client_global_traffics: %v", err)
	}
	return n
}

// A capped catch-up keeps its counters and stays expired, so its cross-panel
// rows still describe a window this client has not spent.
func TestAutoRenewClients_TruncatedCatchUpKeepsCrossPanelTraffic(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-150 * 24 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "capped@x", ID: "66666666-6666-6666-6666-666666666666", Enable: false, Reset: 30, ResetMax: 3, ExpiryTime: past},
	}
	ib := mkInbound(t, 30126, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "capped@x", Enable: false, Reset: 30, ResetMax: 3, ResetCount: 2,
		Up: 111, Down: 222, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	seedGlobalTraffic(t, "capped@x", 111, 222)

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "capped@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 111 || row.Down != 222 {
		t.Fatalf("local counters were reset, so this case no longer exercises the cap: up=%d down=%d", row.Up, row.Down)
	}
	if got := countGlobalTraffic(t, "capped@x"); got != 1 {
		t.Fatalf("cross-panel rows=%d, want 1: the window was dropped for a client that never got it", got)
	}
}

// The counterpart: a real renewal must drop the rows, or the stale pushed
// totals re-deplete the fresh window at once.
func TestAutoRenewClients_RenewedClientLosesCrossPanelTraffic(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-40 * 24 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "rolled@x", ID: "77777777-7777-7777-7777-777777777777", Enable: false, Reset: 30, ExpiryTime: past},
	}
	ib := mkInbound(t, 30127, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: "rolled@x", Enable: false, Reset: 30,
		Up: 333, Down: 444, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	seedGlobalTraffic(t, "rolled@x", 333, 444)

	if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	}

	var row xray.ClientTraffic
	if err := db.Where("email = ?", "rolled@x").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 0 || row.Down != 0 {
		t.Fatalf("counters survived a renewal: up=%d down=%d", row.Up, row.Down)
	}
	if got := countGlobalTraffic(t, "rolled@x"); got != 0 {
		t.Fatalf("cross-panel rows=%d, want 0: stale pushed totals would re-deplete the fresh window", got)
	}
}
