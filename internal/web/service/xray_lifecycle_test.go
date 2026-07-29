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
	state.replace(second)
	state.storeResult(first, "old result")

	process, result := state.snapshot()
	if process != second {
		t.Fatal("snapshot returned the replaced process")
	}
	if result != "" {
		t.Fatalf("snapshot result = %q, want empty", result)
	}
}

func TestXrayLifecycleConcurrentStatusAndTrafficReads(t *testing.T) {
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			xrayState.replace(first)
			xrayState.replace(second)
		}
	}()

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				_ = service.IsXrayRunning()
				_ = service.GetXrayResult()
				_, _, _ = service.GetXrayTraffic()
			}
		}()
	}

	wg.Wait()
}
