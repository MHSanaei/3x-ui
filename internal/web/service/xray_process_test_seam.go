package service

import "github.com/mhsanaei/3x-ui/v3/internal/xray"

// SetXrayProcessForTest installs p as the running process and returns the restore func,
// so tests in other packages can observe online state. Never call it in production.
func SetXrayProcessForTest(p *xray.Process) (restore func()) {
	previousProcess, previousResult := xrayState.snapshot()
	xrayState.replace(p)
	return func() {
		xrayState.mu.Lock()
		xrayState.process = previousProcess
		xrayState.result = previousResult
		xrayState.mu.Unlock()
	}
}
