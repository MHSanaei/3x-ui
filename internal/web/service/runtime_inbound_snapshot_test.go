package service

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/inbound"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// localRuntimeOnCore wires the panel's own runtime deps to an in-process core, whose
// handlers the test can then tell apart by identity.
func localRuntimeOnCore(t *testing.T, cfg *xray.Config) (*core.Instance, *xray.Process, *runtime.Local) {
	t.Helper()
	instance, _ := startIsolationInstance(t, cfg)
	process := xray.NewTestProcess(cfg, filepath.Join(t.TempDir(), "config.json"))
	previousProcess, previousResult := xrayState.snapshot()
	xrayState.replace(process)
	t.Cleanup(func() {
		xrayState.mu.Lock()
		xrayState.process = previousProcess
		xrayState.result = previousResult
		xrayState.mu.Unlock()
	})
	deps := (&XrayService{}).LocalRuntimeDeps()
	deps.APIPort = func() int { return cfg.InboundConfigs[0].Port }
	return instance, process, runtime.NewLocal(deps)
}

func coreInboundHandler(t *testing.T, instance *core.Instance, tag string) inbound.Handler {
	t.Helper()
	handler, err := instance.GetFeature(inbound.ManagerType()).(inbound.Manager).GetHandler(context.Background(), tag)
	if err != nil {
		return nil
	}
	return handler
}

func liveTestInbound(t *testing.T) *model.Inbound {
	t.Helper()
	return &model.Inbound{
		Tag: "added-live", Listen: "127.0.0.1", Port: freeLoopbackPort(t), Protocol: model.VLESS, Enable: true,
		Settings:       `{"clients":[{"id":"7c1c1d1e-0d0a-4b6e-9d5c-2b6f1d0c9a11","email":"live@test"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
}

func withInbound(cfg *xray.Config, ib *model.Inbound) *xray.Config {
	next := *cfg
	next.InboundConfigs = append(slices.Clone(cfg.InboundConfigs), *ib.GenXrayInboundConfig())
	return &next
}

func TestReconcileKeepsAnInboundAddedAtRuntime(t *testing.T) {
	cfg := isolationConfig(t)
	instance, process, local := localRuntimeOnCore(t, cfg)
	ib := liveTestInbound(t)
	if err := local.AddInbound(context.Background(), ib); err != nil {
		t.Fatal(err)
	}
	added := coreInboundHandler(t, instance, ib.Tag)
	if added == nil {
		t.Fatal("the runtime add did not reach the core")
	}

	if applied, err := (&XrayService{}).tryHotApply(process, withInbound(cfg, ib)); err != nil || !applied {
		t.Fatalf("reconcile: applied=%t err=%v", applied, err)
	}
	if coreInboundHandler(t, instance, ib.Tag) != added {
		t.Fatal("the reconcile rebuilt an inbound the core was already running: its listener closed and its clients were cut")
	}
}

func TestReconcileRestoresAnInboundRemovedAtRuntime(t *testing.T) {
	ib := liveTestInbound(t)
	cfg := withInbound(isolationConfig(t), ib)
	instance, process, local := localRuntimeOnCore(t, cfg)
	if err := local.DelInbound(context.Background(), ib); err != nil {
		t.Fatal(err)
	}
	if coreInboundHandler(t, instance, ib.Tag) != nil {
		t.Fatal("the runtime removal did not reach the core")
	}

	// The row is enabled again by a path that only asks for a reconcile.
	if applied, err := (&XrayService{}).tryHotApply(process, cfg); err != nil || !applied {
		t.Fatalf("reconcile: applied=%t err=%v", applied, err)
	}
	if coreInboundHandler(t, instance, ib.Tag) == nil {
		t.Fatal("the reconcile left out an inbound the core no longer runs: the snapshot still claimed it")
	}
}
