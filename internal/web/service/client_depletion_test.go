package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func assertDepletion(t *testing.T, db *gorm.DB, email string, at int64, reason string) {
	t.Helper()
	var row xray.ClientTraffic
	if err := db.Scopes(WithClientDepletion).Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["depletedAt"] != float64(at) || fields["depletedReason"] != reason {
		t.Fatalf("%s depletion = %v/%v, want %d/%q", email, fields["depletedAt"], fields["depletedReason"], at, reason)
	}
}

func TestAutomaticDisableRecordsFirstDepletion(t *testing.T) {
	db := initDepletionTestDB(t)
	now := time.Now().Truncate(time.Millisecond)
	db = db.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	rows := []xray.ClientTraffic{
		{Email: "traffic", Enable: true, Up: 100, Total: 100},
		{Email: "expiry", Enable: true, ExpiryTime: now.UnixMilli() - 1000},
		{Email: "both", Enable: true, Up: 100, Total: 100, ExpiryTime: now.UnixMilli() - 2000},
		{Email: "tie", Enable: true, Up: 100, Total: 100, ExpiryTime: now.UnixMilli()},
		{Email: "global", Enable: true, Total: 100},
		{Email: "manual", Enable: false, Up: 100, Total: 100},
		{Email: "manual-diverged", Enable: true, Up: 100, Total: 100},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	record := model.ClientRecord{Email: "manual-diverged"}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&record).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{Email: "global", MasterGuid: "master", Up: 100, UpdatedAt: now.UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	first := now.UnixMilli()
	svc := &InboundService{}
	for poll := 0; poll < 2; poll++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			_, count, _, err := svc.disableInvalidClients(tx, &trafficMutationBatch{})
			want := int64(6)
			if poll == 1 {
				want = 0
			}
			if err == nil && count != want {
				t.Fatalf("poll %d disabled %d, want %d", poll, count, want)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		assertDepletion(t, db, "traffic", first, "traffic")
		assertDepletion(t, db, "expiry", first-1000, "expiry")
		assertDepletion(t, db, "both", first-2000, "both")
		assertDepletion(t, db, "tie", first, "both")
		assertDepletion(t, db, "global", first, "traffic")
		assertDepletion(t, db, "manual", 0, "traffic")
		assertDepletion(t, db, "manual-diverged", 0, "traffic")
		now = now.Add(time.Minute)
	}
}

func TestDepletionDisableRollsBackAndChunks(t *testing.T) {
	db := initDepletionTestDB(t)
	rows := make([]xray.ClientTraffic, 401)
	for i := range rows {
		rows[i] = xray.ClientTraffic{Email: fmt.Sprintf("depleted-%03d", i), Enable: true, Total: 10, Up: 10}
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	abort := errors.New("abort depletion")
	err := db.Transaction(func(tx *gorm.DB) error {
		_, count, _, err := svc.disableInvalidClients(tx, &trafficMutationBatch{})
		if err != nil {
			return err
		}
		if count != 401 {
			t.Fatalf("disabled %d, want 401", count)
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("transaction error = %v, want %v", err, abort)
	}
	for _, row := range rows {
		assertDepletion(t, db, row.Email, 0, "traffic")
		if !readTraffic(t, db, row.Email).Enable {
			t.Fatalf("rollback disabled %s", row.Email)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, _, err := svc.disableInvalidClients(tx, &trafficMutationBatch{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		got := readTraffic(t, db, row.Email)
		if got.DepletedAt <= 0 || got.Enable {
			t.Fatalf("committed depletion missing: %+v", got)
		}
		assertDepletion(t, db, row.Email, got.DepletedAt, "traffic")
	}
}

func TestDepletionClearsOnlyAfterFullRecovery(t *testing.T) {
	for _, operation := range []string{"reset", "bulk reset", "reset all", "correction", "limits"} {
		t.Run(operation, func(t *testing.T) {
			db := initDepletionTestDB(t)
			past := time.Now().Add(-time.Hour).UnixMilli()
			for _, email := range []string{"spent", "expired"} {
				row := xray.ClientTraffic{Email: email, Total: 100, Up: 100, DepletedAt: past, DepletedReason: "traffic"}
				if email == "expired" {
					row.ExpiryTime = past
				}
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			svc := &InboundService{}
			var err error
			switch operation {
			case "reset":
				for _, email := range []string{"spent", "expired"} {
					if err = svc.ResetClientTrafficByEmail(email); err != nil {
						t.Fatal(err)
					}
				}
			case "bulk reset":
				_, err = (&ClientService{}).BulkResetTraffic(svc, []string{"spent", "expired"})
			case "reset all":
				_, err = (&ClientService{}).ResetAllTraffics()
			case "correction":
				for _, email := range []string{"spent", "expired"} {
					if err = svc.UpdateClientTrafficByEmail(email, 1, 2); err != nil {
						t.Fatal(err)
					}
				}
			case "limits":
				err = db.Transaction(func(tx *gorm.DB) error {
					for _, email := range []string{"spent", "expired"} {
						expiry := int64(0)
						if email == "expired" {
							expiry = past
						}
						if err := svc.UpdateClientStat(tx, email, &model.Client{Email: email, TotalGB: 200, ExpiryTime: expiry}); err != nil {
							return err
						}
					}
					return nil
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			assertDepletion(t, db, "spent", 0, "")
			if got := readTraffic(t, db, "spent"); got.DepletedAt != 0 {
				t.Fatalf("recovered timestamp remains persisted: %+v", got)
			}
			assertDepletion(t, db, "expired", past, "expiry")
			if err := db.Transaction(func(tx *gorm.DB) error {
				return svc.UpdateClientStat(tx, "expired", &model.Client{Email: "expired", TotalGB: 200})
			}); err != nil {
				t.Fatal(err)
			}
			assertDepletion(t, db, "expired", 0, "")
			if got := readTraffic(t, db, "expired"); got.DepletedAt != 0 {
				t.Fatalf("full recovery retained a persisted timestamp: %+v", got)
			}
		})
	}
}

func TestDepletionSurvivesManualEnable(t *testing.T) {
	db := initDepletionTestDB(t)
	at := time.Now().Add(-time.Hour).UnixMilli()
	row := xray.ClientTraffic{Email: "manual-enable", Up: 100, Total: 100, DepletedAt: at, DepletedReason: "traffic"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, Enable: true, TotalGB: 100, ExpiryTime: at + 1000})
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, _, err := svc.disableInvalidClients(tx, &trafficMutationBatch{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, at, "both")
}

func TestInboundImportCannotWriteDepletionMetadata(t *testing.T) {
	db := initDepletionTestDB(t)
	settings := `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"import-depletion","enable":true,"totalGB":100}],"decryption":"none"}`
	inbound := makeImportInbound("import-depletion", 49101, settings, []xray.ClientTraffic{
		{Email: "import-depletion", Up: 111, Total: 100, DepletedAt: 1700000000000, DepletedReason: "expiry"},
	})
	created, _, err := (&InboundService{}).AddInbound(inbound)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.ClientStats) != 1 || created.ClientStats[0].DepletedAt != 0 || created.ClientStats[0].DepletedReason != "traffic" {
		t.Fatalf("import response did not derive current metadata: %+v", created.ClientStats)
	}
	assertDepletion(t, db, "import-depletion", 0, "traffic")
	if got := readTraffic(t, db, "import-depletion"); got.Up != 111 || got.Total != 100 {
		t.Fatalf("import changed counters or limits: %+v", got)
	}
}

func TestPortableImportLeavesDepletionHistoryUnknown(t *testing.T) {
	db := initDepletionTestDB(t)
	row := xray.ClientTraffic{Email: "portable-depletion", Up: 100, Total: 100, DepletedAt: 1700000000000, DepletedReason: "traffic"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := &ClientService{}
	result, _, err := svc.ImportClients(&InboundService{}, []ClientCreatePayload{
		{Client: model.Client{Email: row.Email, TotalGB: 100}, Traffic: &ClientPortableTraffic{Up: 111, Down: 2}},
	})
	if err != nil || result.Created != 1 || len(result.Skipped) != 0 {
		t.Fatalf("import = %+v, %v", result, err)
	}
	assertDepletion(t, db, row.Email, 0, "traffic")
	if got := readTraffic(t, db, row.Email); got.Up != 111 || got.Down != 2 {
		t.Fatalf("import lost usage: %+v", got)
	}
	exported, err := svc.ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"depletedAt"`) || strings.Contains(string(data), `"depletedReason"`) || len(exported) != 1 || exported[0].Traffic.Up != 111 {
		t.Fatalf("portable export contract changed: %s", data)
	}
}

func TestUnattachedClientLimitEditClearsDepletion(t *testing.T) {
	db := initDepletionTestDB(t)
	at := time.Now().Add(-time.Hour).UnixMilli()
	rec := model.ClientRecord{Email: "unattached", TotalGB: 100, Enable: false}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	row := xray.ClientTraffic{Email: rec.Email, Total: 100, Up: 100, DepletedAt: at, DepletedReason: "traffic"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	updated := rec.ToClient()
	updated.TotalGB = 200
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, *updated, 0); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, rec.Email, 0, "")
	if got := readTraffic(t, db, rec.Email); got.Total != 200 || got.Up != 100 {
		t.Fatalf("unattached accounting changed incorrectly: %+v", got)
	}
}

func TestUnattachedClientsDoNotAutoRenew(t *testing.T) {
	db := initDepletionTestDB(t)
	past := time.Now().Add(-time.Hour).UnixMilli()
	rec := model.ClientRecord{Email: "parked", Reset: 30, ExpiryTime: past, Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	row := xray.ClientTraffic{Email: rec.Email, Up: 100, Total: 100, Reset: 30, ExpiryTime: past, DepletedAt: past}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, count, err := (&InboundService{}).autoRenewClients(tx, newTrafficMutationBatch())
		if count != 0 {
			t.Fatalf("renewed parked clients: %d", count)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got := readTraffic(t, db, rec.Email)
	if got.Up != 100 || got.ResetCount != 0 || got.ExpiryTime != past || got.Enable {
		t.Fatalf("parked lifecycle changed: %+v", got)
	}
}

func TestDepletionRecoveryRollsBackWithLimitEdit(t *testing.T) {
	db := initDepletionTestDB(t)
	row := xray.ClientTraffic{Email: "rollback-recovery", Up: 100, Total: 100, DepletedAt: 1700000000000, DepletedReason: "traffic"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	abort := errors.New("reject limit edit")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := (&InboundService{}).UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 200}); err != nil {
			return err
		}
		assertDepletion(t, tx, row.Email, 0, "")
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("rollback error = %v, want %v", err, abort)
	}
	assertDepletion(t, db, row.Email, 1700000000000, "traffic")
	if got := readTraffic(t, db, row.Email); got.Total != 100 || got.Up != 100 {
		t.Fatalf("rollback changed limits/usage: %+v", got)
	}
}

func TestDepletionBulkRecoveryIsScopedAcrossChunks(t *testing.T) {
	db := initDepletionTestDB(t)
	at := time.Now().Add(-time.Hour).UnixMilli()
	rows := make([]xray.ClientTraffic, 402)
	emails := make([]string, 401)
	for i := range rows {
		rows[i] = xray.ClientTraffic{Email: fmt.Sprintf("recover-%03d", i), Up: 100, Total: 100, DepletedAt: at, DepletedReason: "traffic"}
		if i < len(emails) {
			emails[i] = rows[i].Email
		}
	}
	rows[400].ExpiryTime = at
	if err := db.CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	count, err := (&ClientService{}).BulkResetTraffic(&InboundService{}, emails)
	if err != nil || count != 401 {
		t.Fatalf("bulk reset = %d, %v; want 401, nil", count, err)
	}
	for i := 0; i < 400; i++ {
		assertDepletion(t, db, rows[i].Email, 0, "")
	}
	assertDepletion(t, db, rows[400].Email, at, "expiry")
	assertDepletion(t, db, rows[401].Email, at, "traffic")
	if got := readTraffic(t, db, rows[401].Email); got.Up != 100 || got.Enable {
		t.Fatalf("unselected row changed: %+v", got)
	}
}

func TestDepletionRecoveryUsesFreshGlobalQuotas(t *testing.T) {
	db := initDepletionTestDB(t)
	now := time.Now().Truncate(time.Millisecond)
	db = db.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	at := now.Add(-time.Hour).UnixMilli()
	rows := []xray.ClientTraffic{
		{Email: "fresh-global", Total: 100, DepletedAt: at, DepletedReason: "traffic"},
		{Email: "stale-global", Total: 100, DepletedAt: at, DepletedReason: "traffic"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		stamp := now.UnixMilli()
		if i == 1 {
			stamp = globalTrafficFreshSince(now) - 1
		}
		if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: row.Email, Up: 100, UpdatedAt: stamp}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, _, err := (&InboundService{}).disableInvalidClients(tx, newTrafficMutationBatch())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, "fresh-global", at, "traffic")
	assertDepletion(t, db, "stale-global", 0, "")
}

func TestDepletionSurvivesRestartAndConcurrentPartialRecovery(t *testing.T) {
	db := initDepletionTestDB(t)
	at := time.Now().Add(-time.Hour).UnixMilli()
	row := xray.ClientTraffic{Email: "concurrent-event", Up: 100, Total: 100, ExpiryTime: at, DepletedAt: at, DepletedReason: "expiry"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(filepath.Join(os.Getenv("XUI_DB_FOLDER"), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	db = database.GetDB()
	assertDepletion(t, db, row.Email, at, "both")
	resetTrafficWriterForTest(t)
	StartTrafficWriter()
	var wg sync.WaitGroup
	errors := make(chan error, 16)
	svc := &InboundService{}
	for range 8 {
		wg.Go(func() { errors <- svc.ResetClientTrafficByEmail(row.Email) })
		wg.Go(func() {
			errors <- runSerializedTx(func(tx *gorm.DB) error {
				_, _, _, err := svc.disableInvalidClients(tx, newTrafficMutationBatch())
				return err
			})
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertDepletion(t, db, row.Email, at, "expiry")
	if err := runSerializedTx(func(tx *gorm.DB) error {
		return svc.UpdateClientStat(tx, row.Email, &model.Client{Email: row.Email, TotalGB: 200})
	}); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, row.Email, 0, "")
}

func TestClientDepletionPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN")
	}
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"disable and precedence", TestAutomaticDisableRecordsFirstDepletion},
		{"rollback and chunks", TestDepletionDisableRollsBackAndChunks},
		{"recovery chunks", TestDepletionBulkRecoveryIsScopedAcrossChunks},
		{"global recovery", TestDepletionRecoveryUsesFreshGlobalQuotas},
		{"recovery rollback", TestDepletionRecoveryRollsBackWithLimitEdit},
		{"partial and full recovery", TestDepletionClearsOnlyAfterFullRecovery},
		{"unattached limit edits", TestUnattachedClientLimitEditClearsDepletion},
		{"node origin", TestNodeDepletionPreservesOriginAndOlderNodes},
		{"node renewal between polls", TestNodeRenewalKeepsNewPeriodEventWhenMasterMissesRecovery},
		{"node event validation", TestNodeDepletionRejectsInvalidEventsAndPreservesExpiry},
		{"current transitions", TestCurrentDepletionTransitionsWithoutTimestampWrites},
		{"independent masters", TestCurrentReasonUsesEnforcementTotalsBeforeDisplayOverlay},
		{"freshness gap", TestGlobalFreshnessGapDoesNotReusePreviousTimestamp},
		{"freshness before expiry", TestGlobalFreshnessGapBeforeExpiryStartsUnknownPeriod},
		{"local usage after gap", TestLocalTrafficAfterGlobalFreshnessGapClearsOldPeriod},
		{"local partial recovery", TestLocalPartialRecoveryIgnoresExpiredGlobalEvidence},
		{"legacy node adoption", TestLegacyNodeAdoptionPreservesUpstreamAccounting},
		{"shared node accounting", TestCurrentReasonUsesSharedAccumulatorAcrossNodes},
		{"local renewal scope", TestLocalRenewalDoesNotResetIndependentNodePeriods},
		{"node renewal scope", TestNodeRenewalDoesNotResetOtherNodesPeriods},
		{"node partial recovery", TestNodeQuotaEditPreservesContinuousExpiry},
		{"legacy reason", TestLegacyPersistedReasonIsIgnoredWithoutBeingDropped},
		{"legacy lookup", TestLegacyClientTrafficLookupDerivesCurrentReason},
		{"bounded counter sums", TestDepletionCounterSumsDoNotOverflow},
		{"expired deadline extension", TestExpiredDeadlineExtensionPreservesContinuousDepletion},
		{"inbound import response", TestInboundImportCannotWriteDepletionMetadata},
		{"inbound update response", TestInboundUpdateResponseCannotEchoDepletionInput},
		{"nullable legacy limits", TestNullableLegacyLimitsDoNotCarryRecoveredTimestamp},
		{"idle node scope", TestIdleNodeSnapshotDoesNotSweepUnrelatedDepletion},
		{"unattached lifecycle", TestUnattachedClientsDoNotAutoRenew},
		{"concurrent recovery", TestDepletionSurvivesRestartAndConcurrentPartialRecovery},
	} {
		t.Run(tc.name, tc.run)
	}
}

func initDepletionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cleanup, err := testpg.IsolatePackage("depletion")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return initTrafficTestDB(t)
}
