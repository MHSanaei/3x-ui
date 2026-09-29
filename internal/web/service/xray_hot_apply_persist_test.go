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

	if !(&XrayService{}).tryHotApply(process, &xray.Config{RouterConfig: routing}) {
		t.Fatal("tryHotApply = false, want true for an unchanged config")
	}
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatalf("config.json not written after hot apply (err %v)", err)
	}
}
