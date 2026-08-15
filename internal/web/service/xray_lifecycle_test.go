package service

import (
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestXrayLifecycleSnapshotDoesNotOverwriteNewerResult(t *testing.T) {
	state := xrayLifecycle{}
	first := xray.NewProcess(&xray.Config{})
	second := xray.NewProcess(&xray.Config{})

	state.replace(first)
	state.storeResult(first, "first result")
	process, result := state.snapshot()
	if process != first || result != "first result" {
		t.Fatalf("snapshot = (%p, %q), want (%p, %q)", process, result, first, "first result")
	}
	state.replace(second)
	state.storeResult(first, "old result")

	process, result = state.snapshot()
	if process != second {
		t.Fatal("snapshot returned the replaced process")
	}
	if result != "" {
		t.Fatalf("snapshot result = %q, want empty", result)
	}
}

func TestXrayLifecycleConcurrentStatusResultAndTrafficReads(t *testing.T) {
	previousProcess, previousResult := xrayState.snapshot()
	t.Cleanup(func() {
		xrayState.mu.Lock()
		xrayState.process = previousProcess
		xrayState.result = previousResult
		xrayState.mu.Unlock()
	})

	first := xray.NewProcess(&xray.Config{})
	second := xray.NewProcess(&xray.Config{})
	service := XrayService{}
	var wg sync.WaitGroup

	wg.Go(func() {
		for range 200 {
			xrayState.replace(first)
			xrayState.replace(second)
		}
	})

	for range 4 {
		wg.Go(func() {
			for range 200 {
				_ = service.IsXrayRunning()
				_ = service.GetXrayResult()
				_, _, err := service.GetXrayTraffic()
				if err == nil || err.Error() != "xray is not running" {
					t.Errorf("GetXrayTraffic error = %v, want xray is not running", err)
				}
			}
		})
	}

	wg.Wait()
}
