//go:build !linux

package panel

import "os/exec"

func setDetachedProcess(cmd *exec.Cmd) {}

// processAlive is never meaningfully consulted outside Linux: startUpdate
// itself is gated to runtime.GOOS == "linux" before any process is ever
// launched, so no real PID is ever recorded on this platform.
func processAlive(pid int) bool {
	return false
}
