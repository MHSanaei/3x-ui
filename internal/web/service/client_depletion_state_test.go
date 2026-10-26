package service

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestCurrentDepletionTransitionsWithoutTimestampWrites(t *testing.T) {
	db := initDepletionTestDB(t)
	if err := db.Create(&model.ClientRecord{Email: "current-reason"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Millisecond)
	db.NowFunc = func() time.Time { return now }
	row := xray.ClientTraffic{Email: "current-reason", Enable: true, Up: 100, Total: 100, ExpiryTime: now.Add(time.Minute).UnixMilli()}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	if _, _, err := svc.AddTraffic(nil, nil); err != nil {
		t.Fatal(err)
	}
	first := readTraffic(t, db, row.Email).DepletedAt
	if first != now.UnixMilli() {
		t.Fatalf("detection timestamp = %d, want %d", first, now.UnixMilli())
	}
	check := func(at int64, reason string) {
		t.Helper()
		got, err := svc.GetClientTrafficByEmail(row.Email)
		if err != nil || got == nil || got.DepletedAt != at || got.DepletedReason != reason {
			t.Fatalf("current depletion = %+v, %v; want %d/%q", got, err, at, reason)
		}
	}
	check(first, "traffic")
	writes := 0
	const callback = "clock-only-depletion"
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(*gorm.DB) { writes++ }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	now = now.Add(time.Minute)
	check(first, "both")
	if writes != 0 {
		t.Fatalf("clock-only transition wrote %d UPDATEs", writes)
	}
	if err := svc.UpdateClientTrafficByEmail(row.Email, 0, 0); err != nil {
		t.Fatal(err)
	}
	check(first, "expiry")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 100, ExpiryTime: now.Add(time.Minute).UnixMilli()})
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateClientTrafficByEmail(row.Email, 1, 0); err != nil {
		t.Fatal(err)
	}
	check(0, "")
}

func TestCurrentReasonUsesEnforcementTotalsBeforeDisplayOverlay(t *testing.T) {
	db := initDepletionTestDB(t)
	if err := db.Create(&model.ClientRecord{Email: "direction-max"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	if err := db.Create(&xray.ClientTraffic{Email: "direction-max", Total: 100}).Error; err != nil {
		t.Fatal(err)
	}
	for _, g := range []model.ClientGlobalTraffic{
		{MasterGuid: "a", Email: "direction-max", Up: 90, UpdatedAt: now},
		{MasterGuid: "b", Email: "direction-max", Down: 90, UpdatedAt: now},
	} {
		if err := db.Create(&g).Error; err != nil {
			t.Fatal(err)
		}
	}
	got, err := (&InboundService{}).GetClientTrafficByEmail("direction-max")
	if err != nil || got == nil || got.Up != 90 || got.Down != 90 || got.DepletedReason != "" {
		t.Fatalf("independent master totals = %+v, %v", got, err)
	}
}

func TestGlobalFreshnessGapDoesNotReusePreviousTimestamp(t *testing.T) {
	db := initDepletionTestDB(t)
	if err := db.Create(&model.ClientRecord{Email: "freshness-gap"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	at := now.Add(-25 * time.Hour).UnixMilli()
	if err := db.Create(&xray.ClientTraffic{Email: "freshness-gap", Total: 100, DepletedAt: at, DepletedReason: "traffic"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: "freshness-gap", Up: 100, UpdatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	got, err := svc.GetClientTrafficByEmail("freshness-gap")
	if err != nil || got == nil || got.DepletedAt != 0 || got.DepletedReason != "" {
		t.Fatalf("expired report = %+v, %v", got, err)
	}
	if err := svc.AcceptGlobalTraffic("master", []*xray.ClientTraffic{{Email: "freshness-gap", Up: 100}}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.GetClientTrafficByEmail("freshness-gap")
	if err != nil || got == nil || got.DepletedAt != 0 || got.DepletedReason != "traffic" {
		t.Fatalf("new period with unknown origin = %+v, %v", got, err)
	}
}

func TestGlobalFreshnessGapBeforeExpiryStartsUnknownPeriod(t *testing.T) {
	db := initDepletionTestDB(t)
	now := time.Now()
	at := now.Add(-26 * time.Hour).UnixMilli()
	row := xray.ClientTraffic{Email: "gap-before-expiry", Total: 100, ExpiryTime: now.Add(-time.Hour).UnixMilli(), DepletedAt: at}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: row.Email, Up: 100, UpdatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "expiry")
	if err := (&InboundService{}).AcceptGlobalTraffic("master", []*xray.ClientTraffic{{Email: row.Email, Up: 100}}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "both")
}

func TestLocalTrafficAfterGlobalFreshnessGapClearsOldPeriod(t *testing.T) {
	db := initDepletionTestDB(t)
	at := time.Now().Add(-26 * time.Hour).UnixMilli()
	row := xray.ClientTraffic{Email: "local-after-gap", Up: 90, Total: 100, DepletedAt: at}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: row.Email, Up: 100, UpdatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := (&InboundService{}).AddTraffic(nil, []*xray.ClientTraffic{{Email: row.Email, Up: 10}}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "traffic")
	if got := readTraffic(t, db, row.Email); got.DepletedAt != 0 {
		t.Fatalf("obsolete period was persisted: %+v", got)
	}
}

func TestLocalPartialRecoveryIgnoresExpiredGlobalEvidence(t *testing.T) {
	for _, correction := range []bool{false, true} {
		t.Run(fmt.Sprint(correction), func(t *testing.T) {
			db := initDepletionTestDB(t)
			now := time.Now()
			at := now.Add(-26 * time.Hour).UnixMilli()
			row := xray.ClientTraffic{Email: "local-continuous", Up: 100, Total: 100, ExpiryTime: now.Add(-time.Hour).UnixMilli(), DepletedAt: at}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "old-master", Email: row.Email, Up: 100, UpdatedAt: at}).Error; err != nil {
				t.Fatal(err)
			}
			assertDepletion(t, db, row.Email, at, "both")
			svc := &InboundService{}
			if correction {
				if err := svc.UpdateClientTrafficByEmail(row.Email, 1, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := db.Transaction(func(tx *gorm.DB) error {
					return svc.UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 200, ExpiryTime: row.ExpiryTime})
				}); err != nil {
					t.Fatal(err)
				}
			}
			assertDepletion(t, db, row.Email, at, "expiry")
		})
	}
}

func TestLegacyNodeAdoptionPreservesUpstreamAccounting(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "adoption-counts", 41001)
	at := time.Now().Add(-time.Minute).UnixMilli()
	cs := xray.ClientTraffic{Email: "existing-inbound", Up: 100, Total: 100, DepletedAt: at}
	syncNode(t, &InboundService{}, 1, "adoption-counts", cs)
	if got := readTraffic(t, db, cs.Email); got.Up != 0 {
		t.Fatalf("adoption accounting changed: %+v", got)
	}
	assertDepletion(t, db, cs.Email, 0, "")
}

func TestLegacyPersistedReasonIsIgnoredWithoutBeingDropped(t *testing.T) {
	db := initDepletionTestDB(t)
	if err := db.Exec("ALTER TABLE client_traffics ADD COLUMN depleted_reason TEXT DEFAULT 'expiry'").Error; err != nil {
		t.Fatal(err)
	}
	row := xray.ClientTraffic{Email: "legacy-reason", Up: 100, Total: 100, DepletedAt: 1700000000000}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, row.DepletedAt, "traffic")
	row.DepletedReason = "both"
	if err := db.Save(&row).Error; err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.Raw("SELECT depleted_reason FROM client_traffics WHERE email = ?", row.Email).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored != "expiry" {
		t.Fatalf("API field wrote the legacy column: %q", stored)
	}
	assertDepletion(t, db, row.Email, row.DepletedAt, "traffic")
}

func TestNodeQuotaEditPreservesContinuousExpiry(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "partial-node", 41001)
	now := time.Now()
	at := now.Add(-26 * time.Hour).UnixMilli()
	cs := xray.ClientTraffic{Email: "partial-node", Up: 100, Total: 100, ExpiryTime: now.Add(-time.Hour).UnixMilli(), DepletedAt: at}
	svc := &InboundService{}
	initial := cs
	initial.Up, initial.DepletedAt, initial.Enable = 0, 0, true
	syncNode(t, svc, 1, "partial-node", initial)
	syncNode(t, svc, 1, "partial-node", cs)
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "old-master", Email: cs.Email, Up: 100, UpdatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, cs.Email, at, "both")
	cs.Total = 200
	syncNode(t, svc, 1, "partial-node", cs)
	assertDepletion(t, db, cs.Email, at, "expiry")
}

func TestLegacyClientTrafficLookupDerivesCurrentReason(t *testing.T) {
	db := initDepletionTestDB(t)
	ib := model.Inbound{Tag: "legacy-lookup", Port: 41001, Protocol: model.VLESS, Settings: `{"clients":[{"id":"legacy-uuid","email":"legacy-lookup","enable":true}]}`}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	row := xray.ClientTraffic{Email: "legacy-lookup", InboundId: ib.Id, Up: 100, Total: 100, ExpiryTime: time.Now().Add(-time.Hour).UnixMilli(), DepletedAt: 1700000000000}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	got, err := (&InboundService{}).GetClientTrafficByEmail(row.Email)
	if err != nil || got == nil || got.UUID != "legacy-uuid" || got.DepletedAt != row.DepletedAt || got.DepletedReason != "both" {
		t.Fatalf("legacy lookup = %+v, %v", got, err)
	}
}

func TestLocalRenewalDoesNotResetIndependentNodePeriods(t *testing.T) {
	db := initDepletionTestDB(t)
	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	client := model.Client{Email: "shared-renewal", ID: "11111111-1111-1111-1111-111111111111", TotalGB: 100, ExpiryTime: past, Reset: 30}
	local := mkInbound(t, 41001, model.VLESS, clientsSettings(t, []model.Client{client}))
	createNodeInbound(t, db, 1, "shared-renew-node", 41002)
	var remote model.Inbound
	if err := db.Where("tag = ?", "shared-renew-node").First(&remote).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&remote).Update("settings", clientsSettings(t, []model.Client{client})).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	for _, ib := range []*model.Inbound{local, &remote} {
		if err := svc.clientService.SyncInbound(db, ib.Id, []model.Client{client}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&xray.ClientTraffic{Email: client.Email, InboundId: local.Id, Up: 100, Total: 100, ExpiryTime: past, Reset: 30, DepletedAt: past}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: client.Email, Up: 100}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, count, err := svc.autoRenewClients(tx, newTrafficMutationBatch())
		if count != 1 {
			t.Fatalf("renewed %d clients", count)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, client.Email, 0, "")
	if got := readTraffic(t, db, client.Email); got.DepletedAt != 0 {
		t.Fatalf("renewal retained the previous period: %+v", got)
	}
	if got := pendingResetEmails(t, 1); len(got) != 0 {
		t.Fatalf("metadata introduced a cross-node reset: %v", got)
	}
	var baseline model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", 1, client.Email).First(&baseline).Error; err != nil {
		t.Fatal(err)
	}
	if baseline.Up != 100 {
		t.Fatalf("metadata altered node accounting: %+v", baseline)
	}
}

func TestNodeRenewalDoesNotResetOtherNodesPeriods(t *testing.T) {
	db := initDepletionTestDB(t)
	const email = "shared-node-renewal"
	createNodeInboundWithClient(t, db, 1, "period-node-one", 41001, email)
	createNodeInboundWithClient(t, db, 2, "period-node-two", 41002, email)
	svc := &InboundService{}
	cs := xray.ClientTraffic{Email: email, Up: 100, Total: 100, ExpiryTime: renewFirstExpiry, Reset: 30, DepletedAt: time.Now().Add(-time.Hour).UnixMilli()}
	syncNode(t, svc, 1, "period-node-one", cs)
	syncNode(t, svc, 2, "period-node-two", cs)
	cs.Up, cs.ExpiryTime, cs.Enable, cs.ResetCount, cs.DepletedAt = 0, renewSecondExpiry, true, 1, 0
	syncNode(t, svc, 1, "period-node-one", cs)
	assertDepletion(t, db, email, 0, "")
	if got := pendingResetEmails(t, 1); len(got) != 0 {
		t.Fatalf("renewal origin must retain its new-period counters: %v", got)
	}
	if got := pendingResetEmails(t, 2); len(got) != 0 {
		t.Fatalf("metadata introduced a reset of another node: %v", got)
	}
	var baseline model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", 2, email).First(&baseline).Error; err != nil {
		t.Fatal(err)
	}
	if baseline.Up != 100 {
		t.Fatalf("another node's baseline changed: %+v", baseline)
	}
}

func TestCurrentReasonUsesSharedAccumulatorAcrossNodes(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "shared-accounting-one", 41001)
	createNodeInbound(t, db, 2, "shared-accounting-two", 41002)
	svc := &InboundService{}
	cs := xray.ClientTraffic{Email: "shared-accounting", Total: 100, Enable: true}
	syncNode(t, svc, 1, "shared-accounting-one", cs)
	syncNode(t, svc, 2, "shared-accounting-two", cs)
	cs.Up = 60
	syncNode(t, svc, 1, "shared-accounting-one", cs)
	assertDepletion(t, db, cs.Email, 0, "")
	syncNode(t, svc, 2, "shared-accounting-two", cs)
	assertDepletion(t, db, cs.Email, 0, "traffic")
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, _, err := svc.disableInvalidClients(tx, newTrafficMutationBatch())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := readTraffic(t, db, cs.Email); got.Up != 120 || got.DepletedAt <= 0 || got.Enable {
		t.Fatalf("shared quota accounting: %+v", got)
	}
}

func TestDepletionCounterSumsDoNotOverflow(t *testing.T) {
	db := initDepletionTestDB(t)
	rows := []xray.ClientTraffic{
		{Email: "large-local", Enable: true, Up: database.TrafficMax, Down: database.TrafficMax, Total: 100},
		{Email: "large-global", Enable: true, Total: 100},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: "large-global", Up: database.TrafficMax, Down: database.TrafficMax, UpdatedAt: time.Now().UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		assertDepletion(t, db, row.Email, 0, "traffic")
	}
	svc := &InboundService{}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, count, _, err := svc.disableInvalidClients(tx, newTrafficMutationBatch())
		if err == nil && count != 2 {
			t.Fatalf("disabled %d large-counter clients", count)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "large-local").Update("enable", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, count, _, err := svc.disableInvalidClients(tx, newTrafficMutationBatch())
		if err == nil && count != 1 {
			t.Fatalf("local-only path disabled %d clients", count)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredDeadlineExtensionPreservesContinuousDepletion(t *testing.T) {
	for _, recovered := range []bool{false, true} {
		t.Run(fmt.Sprint(recovered), func(t *testing.T) {
			db := initDepletionTestDB(t)
			now := time.Now()
			at := now.Add(-26 * time.Hour).UnixMilli()
			row := xray.ClientTraffic{Email: "extended-expired", Total: 100, ExpiryTime: at, DepletedAt: at}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "old-master", Email: row.Email, Up: 100, UpdatedAt: at}).Error; err != nil {
				t.Fatal(err)
			}
			expiry, wantAt, reason := now.Add(-time.Hour).UnixMilli(), at, "expiry"
			if recovered {
				expiry, wantAt, reason = now.Add(time.Hour).UnixMilli(), 0, ""
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				return (&InboundService{}).UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 100, ExpiryTime: expiry})
			}); err != nil {
				t.Fatal(err)
			}
			assertDepletion(t, db, row.Email, wantAt, reason)
			if got := readTraffic(t, db, row.Email); got.DepletedAt != wantAt {
				t.Fatalf("expiry edit changed persisted continuity: %+v", got)
			}
		})
	}
}

func TestInboundUpdateResponseCannotEchoDepletionInput(t *testing.T) {
	db := initDepletionTestDB(t)
	client := model.Client{Email: "response-client", ID: "11111111-1111-1111-1111-111111111111", Enable: true, TotalGB: 100}
	svc := &InboundService{}
	ib := makeImportInbound("response-client", 49102, clientsSettings(t, []model.Client{client}), []xray.ClientTraffic{{Email: client.Email, Up: 100, Total: 100}})
	created, _, err := svc.AddInbound(ib)
	if err != nil {
		t.Fatal(err)
	}
	const at = int64(1700000000000)
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", client.Email).Update("depleted_at", at).Error; err != nil {
		t.Fatal(err)
	}
	foreign := xray.ClientTraffic{Email: "foreign-response", Up: 100, Total: 100, DepletedAt: at}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	created.ClientStats = []xray.ClientTraffic{
		{Email: client.Email, Up: 7, Total: 8, DepletedAt: 1, DepletedReason: "expiry"},
		{Email: foreign.Email, DepletedAt: 2, DepletedReason: "both"},
	}
	updated, _, err := svc.UpdateInbound(created)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.ClientStats) != 2 || updated.ClientStats[0].DepletedAt != at || updated.ClientStats[0].DepletedReason != "traffic" || updated.ClientStats[0].Up != 7 || updated.ClientStats[0].Total != 8 {
		t.Fatalf("update response metadata = %+v", updated.ClientStats)
	}
	if got := updated.ClientStats[1]; got.DepletedAt != 0 || got.DepletedReason != "" {
		t.Fatalf("response exposed metadata outside inbound membership: %+v", got)
	}
	if got := readTraffic(t, db, client.Email); got.DepletedAt != at || got.Up != 100 || got.Total != 100 {
		t.Fatalf("request wrote traffic/depletion fields: %+v", got)
	}
	if got := readTraffic(t, db, foreign.Email); got.DepletedAt != at || got.Up != 100 {
		t.Fatalf("unrelated traffic was modified: %+v", got)
	}
}

func TestNullableLegacyLimitsDoNotCarryRecoveredTimestamp(t *testing.T) {
	db := initDepletionTestDB(t)
	row := xray.ClientTraffic{Email: "nullable-limits", Up: 100, Total: 100, DepletedAt: 1700000000000}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE client_traffics SET total = NULL, expiry_time = NULL WHERE email = ?", row.Email).Error; err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return (&InboundService{}).UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 100})
	}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "traffic")
	if got := readTraffic(t, db, row.Email); got.DepletedAt != 0 {
		t.Fatalf("new quota inherited a recovered legacy event: %+v", got)
	}
}
