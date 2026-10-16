//go:build linux

package psiphon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// killStrayPsiphonProcesses terminates orphaned psiphon sidecars left over from a
// previous x-ui run and returns how many were killed.
func killStrayPsiphonProcesses(binaryPath string) int {
	base := filepath.Base(binaryPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return 0
	}
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
		exe := procExeBase(pid)
		argv0 := cmdlineArgv0Base(pid)
		if exe != base && argv0 != base && exe != "ConsoleClient" && argv0 != "ConsoleClient" &&
			exe != "psiphon-core" && argv0 != "psiphon-core" {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			killed++
		}
	}
	return killed
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
	argv0 := data
	if i := strings.IndexByte(string(data), 0); i >= 0 {
		argv0 = data[:i]
	}
	if len(argv0) == 0 {
		return ""
	}
	return filepath.Base(string(argv0))
}
