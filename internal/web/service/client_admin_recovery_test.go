package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type adminRecoveryRuntime struct {
	adminApplyProbe
	mu      sync.Mutex
	clients []model.Client
}

func (r *adminRecoveryRuntime) UpdateUser(ctx context.Context, ib *model.Inbound, email string, client model.Client) error {
	if err := r.adminApplyProbe.UpdateUser(ctx, ib, email, client); err != nil {
		return err
	}
	r.mu.Lock()
	r.clients = append(r.clients, client)
	r.mu.Unlock()
	return nil
}

func TestCommittedAdminRenewalReconcilesCurrentStateAfterLifecycleChanges(t *testing.T) {
	for _, change := range []string{"timestamp", "depletion", "first-use"} {
		t.Run(change, func(t *testing.T) {
			svc, _ := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
			probe := &adminRecoveryRuntime{}
			probe.fail.Store(true)
			mgr := runtime.NewManager(runtime.LocalDeps{})
			mgr.SetLocalRuntimeOverride(probe)
			runtime.SetManager(mgr)
			op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30, FirstUse: change == "first-use"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
				t.Fatalf("expected committed failure: %v", err)
			}
			db := database.GetDB()
			wantEnable, wantExpiry := true, readTraffic(t, db, "ali").ExpiryTime
			switch change {
			case "timestamp":
				if err := db.Model(&model.ClientRecord{}).Where("email = ?", "ali").Updates(map[string]any{"updated_at": time.Now().UnixMilli() + 100, "comment": "later edit"}).Error; err != nil {
					t.Fatal(err)
				}
			case "depletion":
				wantEnable = false
				if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "ali").Update("enable", false).Error; err != nil {
					t.Fatal(err)
				}
			case "first-use":
				wantExpiry = time.Now().Add(30 * 24 * time.Hour).UnixMilli()
				if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "ali").Update("expiry_time", wantExpiry).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "ali").Update("up", 321).Error; err != nil {
				t.Fatal(err)
			}
			var files []struct{ Name, File string }
			if err := db.Raw("PRAGMA database_list").Scan(&files).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.CloseDB(); err != nil {
				t.Fatal(err)
			}
			if err := database.InitDB(files[0].File); err != nil {
				t.Fatal(err)
			}
			probe.fail.Store(false)
			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
			stored, err := svc.Load(op.Token, 77)
			if err != nil || stored.State != "done" {
				t.Fatalf("committed receipt could not finish: %+v %v", stored, err)
			}
			traffic := readTraffic(t, database.GetDB(), "ali")
			if traffic.Up != 321 || traffic.Total != 30<<30 || traffic.ExpiryTime != wantExpiry {
				t.Fatalf("retry repeated renewal/reset: %+v", traffic)
			}
			probe.mu.Lock()
			defer probe.mu.Unlock()
			if len(probe.clients) != 2 {
				t.Fatalf("expected one current apply per inbound, got %d", len(probe.clients))
			}
			for _, client := range probe.clients {
				if client.Enable != wantEnable || client.ExpiryTime != wantExpiry {
					t.Fatalf("stale lifecycle replayed: %+v", client)
				}
			}
		})
	}
}

func TestPendingCommittedRenewalIsVisibleBeforeExplicitFutureRenewal(t *testing.T) {
	svc, probe := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
	probe.fail.Store(true)
	spec := ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 30}
	op, err := svc.PrepareRenew(77, "ali", spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected committed failure: %v", err)
	}
	next, err := svc.PrepareRenew(77, "ali", spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Pending) != 1 || next.Pending[0].Token != op.Token || next.Pending[0].Owner != 77 {
		t.Fatalf("unfinished committed renewal was hidden: %+v", next.Pending)
	}
	if got := readTraffic(t, database.GetDB(), "ali").Total; got != 6<<30 {
		t.Fatalf("preview applied another renewal: %d", got)
	}
	probe.fail.Store(false)
	if _, err := svc.Confirm(context.Background(), next.Token, 77); err != nil {
		t.Fatal(err)
	}
	want := readTraffic(t, database.GetDB(), "ali")
	if want.Total != 11<<30 {
		t.Fatalf("explicit future renewal not applied: %+v", want)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal("original receipt cannot reconcile the latest canonical package:", err)
	}
	got := readTraffic(t, database.GetDB(), "ali")
	if got.Total != want.Total || got.ExpiryTime != want.ExpiryTime || got.Up != want.Up {
		t.Fatalf("older retry replayed package changes: %+v want %+v", got, want)
	}
	last, err := svc.PrepareRenew(88, "ali", spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Pending) != 0 {
		t.Fatalf("completed receipts still displayed as pending: %+v", last.Pending)
	}
}

func TestLegacyCommittedReceiptIsIndexedAndNotPruned(t *testing.T) {
	svc, probe := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
	probe.fail.Store(true)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatal(err)
	}
	receipt, err := svc.Load(op.Token, 77)
	if err != nil {
		t.Fatal(err)
	}
	oldToken := time.Now().UTC().AddDate(0, 0, -40).Format("20060102") + op.Token[8:]
	db := database.GetDB()
	if err := db.Where("key IN ?", []string{pendingClientOperationKey(receipt.RecordID), clientOperationSettingPrefix + op.Token}).Delete(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
	receipt.Token = oldToken
	if err := saveClientOperation(db, receipt); err != nil {
		t.Fatal(err)
	}
	next, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 1 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Pending) != 1 || next.Pending[0].Token != oldToken {
		t.Fatalf("legacy committed receipt hidden: %+v", next.Pending)
	}
	if stored, err := svc.Load(oldToken, 77); err != nil || stored.State != "committed" {
		t.Fatalf("pruned unresolved committed receipt: %+v %v", stored, err)
	}
}
