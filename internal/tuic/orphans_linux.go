//go:build linux

package tuic

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func killStrayTuicProcesses(binaryPath string) int {
	base := filepath.Base(binaryPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return 0
	}
	configDir := filepath.Clean(ConfigDir())
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if procExeBase(pid) != base && cmdlineArgv0Base(pid) != base {
			continue
		}
		if !isManagedTuicCmdline(pid, configDir) {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
			killed++
			time.Sleep(50 * time.Millisecond)
			if err := syscall.Kill(pid, 0); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
	return killed
}

func isManagedTuicCmdline(pid int, configDir string) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return false
	}
	args := strings.Split(string(data), "\x00")
	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			cfg := filepath.Clean(args[i+1])
			if strings.HasPrefix(cfg, configDir) {
				return true
			}
		}
	}
	return false
}

func procExeBase(pid int) string {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return ""
	}
	return filepath.Base(exe)
}

func cmdlineArgv0Base(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	first := data
	for i, b := range data {
		if b == 0 {
			first = data[:i]
			break
		}
	}
	return filepath.Base(string(first))
}
