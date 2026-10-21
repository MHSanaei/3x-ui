package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The empty-diff branch of tryHotApply makes no gRPC call, so it can prove the
// hot-apply path refreshes config.json without a running core.
func TestTryHotApplyWritesConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	routing := json_util.RawMessage(`{"rules":[]}`)
	process := xray.NewTestProcess(&xray.Config{RouterConfig: routing}, path)

	if applied, err := (&XrayService{}).tryHotApply(process, &xray.Config{RouterConfig: routing}); err != nil || !applied {
		t.Fatal("tryHotApply = false, want true for an unchanged config")
	}
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatalf("config.json not written after hot apply (err %v)", err)
	}
}

func TestTuicChangeWithoutAPIDoesNotFallBackToCoreRestart(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	previous := &xray.Config{InboundConfigs: []xray.InboundConfig{{
		Tag: "tuic-test", Listen: json_util.RawMessage(`"127.0.0.1"`), Port: 64001,
		Protocol: "socks", TuicRelay: true,
		Settings: json_util.RawMessage(`{"auth":"password","udp":true,"accounts":[{"user":"old@test","pass":"secret"}]}`),
	}}}
	next := &xray.Config{InboundConfigs: append([]xray.InboundConfig(nil), previous.InboundConfigs...)}
	next.InboundConfigs[0].Settings = json_util.RawMessage(`{"auth":"password","udp":true,"accounts":[{"user":"new@test","pass":"secret"}]}`)
	process := xray.NewTestProcess(previous, filepath.Join(t.TempDir(), "config.json"))
	applied, err := (&XrayService{}).tryHotApply(process, next)
	if applied || err == nil || err.Error() != "TUIC relay update requires the Xray HandlerService API" {
		t.Fatalf("missing API must defer TUIC changes, not fall back to restarting the core: applied=%t err=%v", applied, err)
	}
	if !process.GetConfig().Equals(previous) {
		t.Fatal("failed relay update advanced the running config snapshot")
	}
	if !isNeedXrayReconcile.Load() {
		t.Fatal("direct reconciliation failure did not schedule a retry")
	}
}
