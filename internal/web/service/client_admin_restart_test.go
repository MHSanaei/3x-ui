package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type adminCreateFailure struct{ fakeNodeRuntime }

func (*adminCreateFailure) AddUser(context.Context, *model.Inbound, map[string]any) error {
	return errors.New("local handler unavailable")
}

func TestAdminCreateSchedulesRestartIncludingPartialFailure(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "created", true: "partial"}[partial], func(t *testing.T) {
			ids := planTestDB(t)
			mgr := runtime.NewManager(runtime.LocalDeps{})
			mgr.SetLocalRuntimeOverride(&adminCreateFailure{})
			runtime.SetManager(mgr)
			t.Cleanup(func() { runtime.SetManager(nil) })
			xraySvc := &XrayService{}
			_ = xraySvc.IsNeedRestartAndSetFalse()
			t.Cleanup(func() { _ = xraySvc.IsNeedRestartAndSetFalse() })
			plan, err := (&ClientPlanService{}).Save(ClientPlan{Name: "A", Days: 30, Enabled: true, InboundIDs: ids, RenewalMode: "replace"})
			if err != nil {
				t.Fatal(err)
			}
			if partial {
				if err := database.GetDB().Exec(`CREATE TRIGGER fail_one_admin_add BEFORE UPDATE ON inbounds WHEN OLD.protocol = 'trojan' BEGIN SELECT RAISE(ABORT, 'injected partial add'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			svc := &ClientAdminWorkflowService{}
			op, err := svc.PrepareCreate(77, "restart-client", plan.Name)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.Confirm(context.Background(), op.Token, 77)
			if partial && (err == nil || !strings.Contains(err.Error(), "injected partial add")) {
				t.Fatalf("partial result: %v", err)
			}
			if !partial && err != nil {
				t.Fatal(err)
			}
			if !xraySvc.IsNeedRestartAndSetFalse() {
				t.Fatal("committed local add failure did not request restart")
			}
			if _, err := (&ClientService{}).GetRecordByEmail(nil, "restart-client"); err != nil {
				t.Fatal("lost committed client:", err)
			}
		})
	}
}

func TestAdminRenewalSchedulesLocalRestartWithoutRepeatingAccounting(t *testing.T) {
	svc, probe := adminWorkflowDB(t, false, 1<<30, time.Now().Add(-time.Hour).UnixMilli(), 2<<30)
	xraySvc := &XrayService{}
	_ = xraySvc.IsNeedRestartAndSetFalse()
	t.Cleanup(func() { _ = xraySvc.IsNeedRestartAndSetFalse() })
	probe.fail.Store(true)
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "replace", QuotaBytes: 30 << 30, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected committed pending result: %v", err)
	}
	if !xraySvc.IsNeedRestartAndSetFalse() {
		t.Fatal("failed local renewal did not request restart")
	}
	stored, err := svc.Load(op.Token, 77)
	if err != nil || stored.State != "committed" {
		t.Fatalf("receipt lost its committed state: %+v %v", stored, err)
	}
	if traffic := readTraffic(t, database.GetDB(), "ali"); traffic.Up != 0 || traffic.Total != 30<<30 {
		t.Fatalf("incorrect committed accounting: %+v", traffic)
	}
}

type adminRemoteFailure struct{ fakeNodeRuntime }

func (*adminRemoteFailure) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	return errors.New("remote synchronization unavailable")
}

func TestAdminRemoteFailureKeepsNodeRecoverySeparateFromLocalRestart(t *testing.T) {
	svc, _ := adminWorkflowDB(t, true, 10<<30, 0, 1<<30)
	db := database.GetDB()
	if err := db.Create(&model.Node{Id: 1, Name: "remote"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inbound{}).Where("id > 0").Update("node_id", 1).Error; err != nil {
		t.Fatal(err)
	}
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetRuntimeOverride(1, &adminRemoteFailure{})
	runtime.SetManager(mgr)
	xraySvc := &XrayService{}
	_ = xraySvc.IsNeedRestartAndSetFalse()
	op, err := svc.PrepareRenew(77, "ali", ClientRenewalSpec{Mode: "extend", QuotaBytes: 5 << 30, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected remote pending: %v", err)
	}
	if xraySvc.IsNeedRestartAndSetFalse() {
		t.Fatal("remote error requested a local core restart")
	}
	var node model.Node
	if err := db.First(&node, 1).Error; err != nil {
		t.Fatal(err)
	}
	if !node.ConfigDirty {
		t.Fatal("remote recovery lost its dirty flag")
	}
}
