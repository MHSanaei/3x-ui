package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type adminApplyProbe struct {
	fakeNodeRuntime
	fail    atomic.Bool
	updates atomic.Int32
}

func (r *adminApplyProbe) UpdateUser(_ context.Context, _ *model.Inbound, email string, _ model.Client) error {
	r.updates.Add(1)
	var record model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&record).Error; err != nil {
		return err
	}
	if r.fail.Load() {
		return errors.New("runtime unavailable")
	}
	return nil
}

func adminWorkflowDB(t *testing.T, enabled bool, quota, expiry, used int64) (*ClientAdminWorkflowService, *adminApplyProbe) {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "workflow.db"))
	probe := &adminApplyProbe{}
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetLocalRuntimeOverride(probe)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	client := model.Client{Email: "ali", ID: "11111111-2222-4333-8444-555555555555", Password: "keep-password", SubID: "keep-sub", TgID: 9182, Enable: enabled, TotalGB: quota, ExpiryTime: expiry, TrafficReset: "never", TrafficResetDay: 1}
	for _, protocol := range []model.Protocol{model.VLESS, model.Trojan} {
		inbound := mkInbound(t, 8443, protocol, clientsSettings(t, []model.Client{client}))
		if err := (&ClientService{}).SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "ali", Enable: enabled, Total: quota, ExpiryTime: expiry, Up: used}).Error; err != nil {
		t.Fatal(err)
	}
	return &ClientAdminWorkflowService{}, probe
}

func TestAdminRenewalReplaceIsAtomicGlobalAwareAndRestartIdempotent(t *testing.T) {
	now := time.Now().UnixMilli()
	svc, probe := adminWorkflowDB(t, false, 10<<30, now-1000, 1<<30)
	db := database.GetDB()
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "master", Email: "ali", Up: 9 << 30, Down: 2 << 30, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if op.Preview.UsedBytes != 11<<30 || !op.Preview.ResetUsage || !op.Preview.EnabledAfter {
		t.Fatalf("incorrect global/depleted preview: %+v", op.Preview)
	}
	before := time.Now().UnixMilli()
	done, err := svc.Confirm(context.Background(), op.Token, 77)
	if err != nil {
		t.Fatal(err)
	}
	if done.State != "done" {
		t.Fatalf("not done: %+v", done)
	}
	var record model.ClientRecord
	if err := db.Where("email = ?", "ali").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TotalGB != 30<<30 || !record.Enable || record.ExpiryTime < before+30*86400000 || record.SubID != "keep-sub" || record.TgID != 9182 || record.UUID != "11111111-2222-4333-8444-555555555555" || record.Password != "keep-password" {
		t.Fatalf("wrong renewal or identity changed: %+v", record)
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", "ali").First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 0 || traffic.Down != 0 || !traffic.Enable || traffic.Total != record.TotalGB || traffic.ExpiryTime != record.ExpiryTime {
		t.Fatalf("traffic diverged: %+v", traffic)
	}
	var globalCount int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", "ali").Count(&globalCount).Error; err != nil || globalCount != 0 {
		t.Fatalf("old globals retained: %d %v", globalCount, err)
	}
	if probe.updates.Load() != 2 {
		t.Fatalf("reenabling did not dispatch both inbounds: %d", probe.updates.Load())
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "ali").Update("up", 1234).Error; err != nil {
		t.Fatal(err)
	}
	restarted := &ClientAdminWorkflowService{}
	if _, err := restarted.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("email = ?", "ali").First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 1234 || probe.updates.Load() != 2 {
		t.Fatalf("replay reset new-period usage or reapplied runtime: %+v", traffic)
	}
}

func TestAdminRenewalExtendPreservesUsageAndManualDisablement(t *testing.T) {
	for _, expiry := range []int64{0, -7 * 86400000, time.Now().Add(-time.Hour).UnixMilli(), time.Now().Add(time.Hour).UnixMilli()} {
		t.Run(time.UnixMilli(expiry).String(), func(t *testing.T) {
			quota := int64(10 << 30)
			if expiry == 0 {
				quota = 0
			}
			svc, _ := adminWorkflowDB(t, false, quota, expiry, 2<<30)
			op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 3})
			if err != nil {
				t.Fatal(err)
			}
			if op.Preview.ResetUsage {
				t.Fatal("extension resets usage")
			}
			if expiry >= 0 && (expiry == 0 || expiry > time.Now().UnixMilli()) && op.Preview.EnabledAfter {
				t.Fatal("manual disablement lost")
			}
			if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
				t.Fatal(err)
			}
			var traffic xray.ClientTraffic
			if err := database.GetDB().Where("email = ?", "ali").First(&traffic).Error; err != nil {
				t.Fatal(err)
			}
			if traffic.Up != 2<<30 {
				t.Fatal("extension reset consumption")
			}
			if expiry == 0 && (traffic.Total != 0 || traffic.ExpiryTime != 0 || traffic.Enable) {
				t.Fatalf("unlimited/manual state changed: %+v", traffic)
			}
			if expiry < 0 && traffic.ExpiryTime != -10*86400000 {
				t.Fatalf("first-use extension became a calendar date: %d", traffic.ExpiryTime)
			}
		})
	}
}

func TestAdminRenewalRejectsStaleConcurrentAndForeignConfirmations(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, time.Now().Add(time.Hour).UnixMilli(), 1<<30)
	a, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.PrepareRenew(88, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 7 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), a.Token, 88); err == nil {
		t.Fatal("foreign actor confirmed operation")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for _, op := range []*ClientAdminOperation{a, b} {
		wg.Go(func() {
			if _, err := svc.Confirm(context.Background(), op.Token, op.Owner); err == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent stale operations applied %d times", successes.Load())
	}
	var record model.ClientRecord
	if err := database.GetDB().Where("email = ?", "ali").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TotalGB != 15<<30 && record.TotalGB != 17<<30 {
		t.Fatalf("quota overwritten/added twice: %d", record.TotalGB)
	}
}

func TestAdminRenewalRollsBackAndRetriesRuntimeWithoutResettingAgain(t *testing.T) {
	svc, probe := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	if err := db.Exec(`CREATE TRIGGER fail_admin_renew BEFORE UPDATE ON clients BEGIN SELECT RAISE(ABORT, 'injected rollback'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err == nil {
		t.Fatal("failed transaction reported success")
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", "ali").First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 2<<30 || traffic.Total != 1<<30 || probe.updates.Load() != 0 {
		t.Fatalf("rollback leaked traffic/runtime mutation: %+v", traffic)
	}
	if err := db.Exec(`DROP TRIGGER fail_admin_renew`).Error; err != nil {
		t.Fatal(err)
	}
	probe.fail.Store(true)
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err == nil {
		t.Fatal("runtime failure acknowledged as complete")
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "ali").Update("up", 321).Error; err != nil {
		t.Fatal(err)
	}
	probe.fail.Store(false)
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("email = ?", "ali").First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 321 {
		t.Fatalf("retry reset new usage: %+v", traffic)
	}
}

func TestAdminCreateRequiresConfirmationAndReusesClientService(t *testing.T) {
	ids := planTestDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	plan, err := (&ClientPlanService{}).Save(ClientPlan{Name: "A", QuotaBytes: 30 << 30, Days: 30, InboundIDs: ids, Enabled: true, RenewalMode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	svc := &ClientAdminWorkflowService{}
	op, err := svc.PrepareCreate(77, "ali", plan.Name)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("preview created a client")
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	record, err := (&ClientService{}).GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if record.UUID == "" || record.Password == "" || record.SubID == "" || record.TotalGB != plan.QuotaBytes {
		t.Fatalf("creation did not generate protocol credentials or apply plan: %+v", record)
	}
	attached, err := (&ClientService{}).GetInboundIdsForRecord(record.Id)
	if err != nil || len(attached) != 2 {
		t.Fatalf("attachments %v %v", attached, err)
	}
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrepareCreate(77, "ali", plan.Name); err == nil {
		t.Fatal("duplicate email accepted")
	}
	second, err := svc.PrepareCreate(77, "bob", plan.Name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), second.Token, 77); err != nil {
		t.Fatal(err)
	}
	bob, err := (&ClientService{}).GetRecordByEmail(nil, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if bob.UUID == record.UUID || bob.Password == record.Password || bob.SubID == record.SubID {
		t.Fatal("new clients reused protocol credentials or subscription identity")
	}
	other, err := svc.PrepareCreate(77, "charlie", plan.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel(other.Token, 77); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), other.Token, 77); err == nil {
		t.Fatal("cancelled confirmation created client")
	}
	if _, err := (&ClientService{}).GetRecordByEmail(nil, "charlie"); err == nil {
		t.Fatal("cancelled client exists")
	}
}

func TestAdminRenewalPreservesPartialDepletionAndSupportsUnattachedClients(t *testing.T) {
	svc, _ := adminWorkflowDB(t, false, 10<<30, 0, 20<<30)
	db := database.GetDB()
	if err := db.Where("client_id IN (?)", db.Model(&model.ClientRecord{}).Select("id").Where("email = ?", "ali")).Delete(&model.ClientInbound{}).Error; err != nil {
		t.Fatal(err)
	}
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 1 << 30, Days: 3})
	if err != nil {
		t.Fatal(err)
	}
	if op.Preview.EnabledAfter {
		t.Fatal("partial quota recovery re-enabled exhausted client")
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	op, err = svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	record, err := (&ClientService{}).GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if !record.Enable || record.TotalGB != 30<<30 {
		t.Fatalf("unattached client not renewed: %+v", record)
	}
}

func TestAdminRenewalRejectsChangedPlanAndMissingGlobalAccounting(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	ids, err := (&ClientService{}).GetInboundIdsForEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := (&ClientPlanService{}).Save(ClientPlan{Name: "A", Days: 30, QuotaBytes: 30 << 30, InboundIDs: ids, RenewalMode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := svc.PrepareRenewPlan(77, "ali", "A")
	if err != nil {
		t.Fatal(err)
	}
	plan.QuotaBytes = 50 << 30
	if _, err := (&ClientPlanService{}).Save(*plan); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err == nil {
		t.Fatal("changed plan silently applied")
	}
	if err := database.GetDB().Migrator().DropTable(&model.ClientGlobalTraffic{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", Days: 30}); err == nil {
		t.Fatal("renewal proceeded without authoritative global accounting")
	}
}

func TestAdminCreateCrashRecoveryDoesNotOverwriteLaterChanges(t *testing.T) {
	ids := planTestDB(t)
	if _, err := (&ClientPlanService{}).Save(ClientPlan{Name: "A", Days: 30, QuotaBytes: 30 << 30, InboundIDs: ids, Enabled: true, RenewalMode: "replace"}); err != nil {
		t.Fatal(err)
	}
	svc := &ClientAdminWorkflowService{}
	op, err := svc.PrepareCreate(77, "ali", "A")
	if err != nil {
		t.Fatal(err)
	}
	op.State = "creating"
	op.ConfirmedAt = time.Now().UnixMilli()
	if err := saveClientOperation(database.GetDB(), op); err != nil {
		t.Fatal(err)
	}
	client := op.Plan.Payload("ali", op.SubID, op.ConfirmedAt).Client
	client.TotalGB = 99 << 30
	if err := database.GetDB().Create(client.ToRecord()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationChanged) {
		t.Fatalf("unsafe interrupted creation retry: %v", err)
	}
	record, err := (&ClientService{}).GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if record.TotalGB != 99<<30 {
		t.Fatal("later edit overwritten by old creation")
	}
}

type adminNodeResetProbe struct {
	fakeNodeRuntime
	fail   atomic.Bool
	resets atomic.Int32
}

func (r *adminNodeResetProbe) ResetClientTraffic(context.Context, *model.Inbound, string) error {
	r.resets.Add(1)
	if r.fail.Load() {
		return errors.New("node unavailable")
	}
	return nil
}

func TestAdminRenewalRetainsPendingNodeResetUntilAcknowledged(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	db := database.GetDB()
	node := &model.Node{Name: "renew-node", Address: "127.0.0.1", Port: 2096, ApiToken: "test", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	ids, err := (&ClientService{}).GetInboundIdsForEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", ids[0]).Updates(map[string]any{"node_id": node.Id, "enable": false}).Error; err != nil {
		t.Fatal(err)
	}
	probe := &adminNodeResetProbe{}
	probe.fail.Store(true)
	runtime.GetManager().SetRuntimeOverride(node.Id, probe)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("unacknowledged node reset reported complete: %v", err)
	}
	var pending int64
	if err := db.Model(&model.NodePendingReset{}).Where("node_id = ? AND email = ?", node.Id, "ali").Count(&pending).Error; err != nil || pending != 1 {
		t.Fatalf("reset outbox lost: %d %v", pending, err)
	}
	probe.fail.Store(false)
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.NodePendingReset{}).Where("node_id = ? AND email = ?", node.Id, "ali").Count(&pending).Error; err != nil || pending != 0 {
		t.Fatalf("acknowledged reset retained: %d %v", pending, err)
	}
	if probe.updateInbound.Load() == 0 || probe.resets.Load() != 2 {
		t.Fatalf("node was not synchronized: config=%d reset=%d", probe.updateInbound.Load(), probe.resets.Load())
	}
}

func TestAdminWorkflowReceiptsAndPlansSurviveDatabaseReopen(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	var files []struct{ Name, File string }
	if err := database.GetDB().Raw("PRAGMA database_list").Scan(&files).Error; err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("missing SQLite database path")
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(files[0].File); err != nil {
		t.Fatal(err)
	}
	restarted := &ClientAdminWorkflowService{}
	if _, err := restarted.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	var setting model.Setting
	if err := database.GetDB().Where("key = ?", clientOperationSettingPrefix+op.Token).First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	var receipt ClientAdminOperation
	if err := json.Unmarshal([]byte(setting.Value), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.State != "done" {
		t.Fatalf("receipt lost across restart: %+v", receipt)
	}
}

func TestAdminRenewalPreservesUnknownInboundMetadataExactly(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	db := database.GetDB()
	var inbound model.Inbound
	if err := db.First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	settings := strings.Replace(inbound.Settings, `"email": "ali"`, `"email": "ali", "customMetadata": {"number":9007199254740993}`, 1)
	if settings == inbound.Settings {
		t.Fatal("fixture did not add metadata")
	}
	if err := db.Model(&inbound).Update("settings", settings).Error; err != nil {
		t.Fatal(err)
	}
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&inbound, inbound.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inbound.Settings, "9007199254740993") {
		t.Fatalf("renewal rounded unrelated metadata: %s", inbound.Settings)
	}
}

func TestAdminRenewalConcurrentSameTokenAppliesOnce(t *testing.T) {
	svc, probe := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	startSerializedWriter(t)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	record, err := (&ClientService{}).GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if record.TotalGB != 15<<30 || probe.updates.Load() != 0 {
		t.Fatalf("replay added quota twice or unnecessarily changed runtime: %+v", record)
	}
}

func TestAdminRenewalExplicitlyPreservesDisabledDepletedClient(t *testing.T) {
	svc, probe := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !op.Preview.EnabledAfter {
		t.Fatal("fixture is not eligible for recovered enablement")
	}
	if _, err := svc.KeepDisabled(op.Token, 88); err == nil {
		t.Fatal("foreign actor changed enablement policy")
	}
	updated, err := svc.KeepDisabled(op.Token, 77)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Preview.EnabledAfter {
		t.Fatal("explicit disabled policy absent from preview")
	}
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	record, err := (&ClientService{}).GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if record.Enable || record.TotalGB != 30<<30 || probe.updates.Load() != 0 {
		t.Fatalf("explicit disabled policy lost: %+v", record)
	}
	if _, err := svc.KeepDisabled(op.Token, 77); err == nil {
		t.Fatal("completed operation policy was changed")
	}
}

func TestAdminRenewalCoordinatesWithExistingInboundMutationLock(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := (&ClientService{}).GetInboundIdsForEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	lock := lockInbound(ids[0])
	done := make(chan error, 1)
	go func() { _, err := svc.Confirm(context.Background(), op.Token, 77); done <- err }()
	select {
	case err := <-done:
		lock.Unlock()
		t.Fatalf("renewal bypassed the panel's inbound mutation lock: %v", err)
	case <-time.After(100 * time.Millisecond):
		lock.Unlock()
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("renewal failed to resume after the inbound mutation")
	}
}

func TestAdminRenewalKeepsFrozenNodeLifecycleUntilResetDelivery(t *testing.T) {
	inboundSvc := &InboundService{}
	db, _ := seedLatchedNodeClient(t, inboundSvc)
	probe := &adminNodeResetProbe{}
	probe.fail.Store(true)
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetRuntimeOverride(1, probe)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	svc := &ClientAdminWorkflowService{}
	op, err := svc.PrepareRenew(77, "reset-lost", ClientRenewalSpec{Mode: "replace", QuotaBytes: 300, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected pending node reset: %v", err)
	}
	settings := `{"clients":[{"email":"reset-lost","enable":false,"totalGB":100},{"email":"fresh-node","enable":true,"totalGB":1000,"expiryTime":0}]}`
	syncNodeWithSettings(t, inboundSvc, 1, "n1-in", settings,
		xray.ClientTraffic{Email: "reset-lost", Up: 60, Down: 60, Total: 100, Enable: false},
		xray.ClientTraffic{Email: "fresh-node", Total: 1000, Enable: true})
	traffic := readTraffic(t, db, "reset-lost")
	if !traffic.Enable || traffic.Up+traffic.Down != 0 || traffic.Total != 300 {
		t.Fatalf("stale node snapshot undid the package renewal: %+v", traffic)
	}
	fresh, err := (&ClientService{}).GetRecordByEmail(nil, "fresh-node")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.TotalGB != 1000 || !fresh.Enable {
		t.Fatalf("pending reset prevented fresh client adoption: %+v", fresh)
	}
	probe.fail.Store(false)
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRenewalRejectsTrailingSettingsJSON(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	var inbound model.Inbound
	db := database.GetDB()
	if err := db.First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&inbound).Update("settings", inbound.Settings+"{}").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30}); err == nil {
		t.Fatal("malformed settings were silently accepted and a pending renewal was persisted")
	}
	var receipts int64
	if err := db.Model(&model.Setting{}).Where("key LIKE ?", clientOperationSettingPrefix+"%").Count(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatal("invalid client settings created a receipt")
	}
}

func TestLiftedNodeLifecyclePreservesExactQuota(t *testing.T) {
	const quota = int64(9007199254740993)
	settings := `{"clients":[{"email":"large","enable":true,"totalGB":9007199254740993,"expiryTime":0}]}`
	lifted, changed := liftClientLifecycleInSettings(settings, map[string]*xray.ClientTraffic{"large": {Email: "large", Enable: true, Total: quota, ExpiryTime: 2000}})
	if !changed {
		t.Fatal("node lifecycle was not lifted")
	}
	clients, err := ParseInboundSettingsClients(lifted)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 || clients[0].TotalGB != quota || clients[0].ExpiryTime != 2000 {
		t.Fatalf("lifecycle lift corrupted the package: %+v", clients)
	}
}
