//go:build linux

package panel

import (
	"os/exec"
	"syscall"
)

func setDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// processAlive reports whether pid is still a live process, via the standard
// POSIX kill(pid, 0) liveness check: it sends no actual signal, only checking
// whether the target exists and is signalable. ESRCH means the process is
// gone; any other result (including a permission error, which can only mean
// the PID exists and belongs to someone) is treated as alive, since this is
// used to decide whether it is safe to let a second update start.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
