package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAdminWorkflowPostgresDurability(t *testing.T) {
	db := durablePostgresDB(t)
	if _, _, err := database.EnsureFirstUser(); err != nil {
		t.Fatal(err)
	}
	startSerializedWriter(t)
	mgr := runtime.NewManager(runtime.LocalDeps{})
	probe := &adminApplyProbe{}
	mgr.SetLocalRuntimeOverride(probe)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	var ids []int
	for _, protocol := range []model.Protocol{model.VLESS, model.Trojan} {
		ids = append(ids, mkInbound(t, 8443, protocol, `{"clients":[]}`).Id)
	}
	suffix := uuid.NewString()[:8]
	plan, err := (&ClientPlanService{}).Save(ClientPlan{Name: "PG_" + suffix, Days: 30, QuotaBytes: 1 << 30, Enabled: true, InboundIDs: ids, RenewalMode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	email := "pg-admin-" + suffix
	svc := &ClientAdminWorkflowService{}
	create, err := svc.PrepareCreate(77, email, plan.Name)
	if err != nil {
		t.Fatal(err)
	}
	tokens := []string{create.Token}
	t.Cleanup(func() {
		current := database.GetDB()
		var record model.ClientRecord
		if current.Where("email = ?", email).First(&record).Error == nil {
			_ = current.Where("key = ?", pendingClientOperationKey(record.Id)).Delete(&model.Setting{}).Error
			_ = current.Where("key = ?", clientResetOperationKey(record.Id)).Delete(&model.Setting{}).Error
		}
		_ = current.Where("client_id IN (?)", current.Model(&model.ClientRecord{}).Select("id").Where("email = ?", email)).Delete(&model.ClientInbound{}).Error
		_ = current.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error
		_ = current.Where("email = ?", email).Delete(&model.ClientGlobalTraffic{}).Error
		_ = current.Where("email = ?", email).Delete(&model.ClientRecord{}).Error
		_ = current.Where("id IN ?", ids).Delete(&model.Inbound{}).Error
		for _, token := range tokens {
			_ = current.Where("key = ?", clientOperationSettingPrefix+token).Delete(&model.Setting{}).Error
		}
		_ = (&ClientPlanService{}).Delete(plan.ID, plan.Revision)
	})
	if _, err := svc.Confirm(context.Background(), create.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("up", 1<<29).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "pg-master", Email: email, Up: 2 << 30, UpdatedAt: time.Now().UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	op, err := svc.PrepareRenew(77, email, ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	tokens = append(tokens, op.Token)
	if op.Preview.UsedBytes != 2<<30 {
		t.Fatalf("global PostgreSQL preview: %+v", op.Preview)
	}
	callback := "test:admin-workflow-commit-failure"
	installDeferredCommitFailure(t, db, "update", callback, "clients", "admin_commit_parent", "admin_commit_child")
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err == nil {
		t.Fatal("deferred commit failure reported success")
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 1<<29 || traffic.Total != 1<<30 {
		t.Fatalf("failed PostgreSQL transaction changed traffic: %+v", traffic)
	}
	pending, err := svc.Load(op.Token, 77)
	if err != nil || pending.State != "pending" {
		t.Fatalf("receipt advanced on rollback: %+v %v", pending, err)
	}
	if err := db.Callback().Update().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var applied atomic.Int32
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
				t.Error(err)
			} else {
				applied.Add(1)
			}
		})
	}
	wg.Wait()
	if applied.Load() != 6 {
		t.Fatalf("idempotent confirmations: %d", applied.Load())
	}
	if err := db.Exec("DROP TABLE admin_commit_child").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP TABLE admin_commit_parent").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("up", 123).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(""); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ClientAdminWorkflowService{}).Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Where("email = ?", email).First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 123 || traffic.Total != 30<<30 {
		t.Fatalf("PostgreSQL restart replay changed new period: %+v", traffic)
	}
	stored, err := (&ClientPlanService{}).Resolve(plan.Name)
	if err != nil || stored.ID != plan.ID {
		t.Fatalf("plan lost on PostgreSQL reopen: %+v %v", stored, err)
	}
	current := database.GetDB()
	for _, row := range []any{&model.ClientRecord{}, &xray.ClientTraffic{}} {
		if err := current.Model(row).Where("email = ?", email).Updates(map[string]any{"enable": false, "expiry_time": time.Now().Add(-time.Hour).UnixMilli()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	recovery, err := svc.PrepareRenew(77, email, ClientRenewalSpec{Mode: "replace", QuotaBytes: 40 << 30, Days: 30, FirstUse: true})
	if err != nil {
		t.Fatal(err)
	}
	tokens = append(tokens, recovery.Token)
	probe.fail.Store(true)
	if _, err := svc.Confirm(context.Background(), recovery.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected committed PostgreSQL synchronization failure: %v", err)
	}
	activated := time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	if err := current.Model(&xray.ClientTraffic{}).Where("email = ?", email).Updates(map[string]any{"up": 321, "expiry_time": activated}).Error; err != nil {
		t.Fatal(err)
	}
	if err := current.Model(&model.ClientRecord{}).Where("email = ?", email).Update("updated_at", time.Now().UnixMilli()+100).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(""); err != nil {
		t.Fatal(err)
	}
	next, err := svc.PrepareRenew(77, email, ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	tokens = append(tokens, next.Token)
	if len(next.Pending) != 1 || next.Pending[0].Token != recovery.Token {
		t.Fatalf("reopened PostgreSQL lost unresolved receipt: %+v", next.Pending)
	}
	probe.fail.Store(false)
	for range 4 {
		wg.Go(func() {
			if _, err := svc.Confirm(context.Background(), recovery.Token, 77); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := database.GetDB().Where("email = ?", email).First(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 321 || traffic.Total != 40<<30 || traffic.ExpiryTime != activated {
		t.Fatalf("committed recovery reapplied PostgreSQL accounting: %+v", traffic)
	}
}
