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

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// cleanupLegacySidecar terminates any orphaned Rust tuic-server processes left over
// from earlier versions and cleans up obsolete sidecar files from bin/.
func cleanupLegacySidecar() {
	cleanupLegacyProcesses()
	cleanupLegacyFiles()
}

func cleanupLegacyProcesses() int {
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
		if isLegacyTuicProcess(pid) {
			if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
				killed++
				time.Sleep(20 * time.Millisecond)
				if err := syscall.Kill(pid, 0); err == nil {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}
		}
	}
	if killed > 0 {
		logger.Warningf("tuic: terminated %d orphaned legacy tuic-server process(es)", killed)
	}
	return killed
}

func isLegacyTuicProcess(pid int) bool {
	// Match executable base name
	if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
		base := filepath.Base(exe)
		base = strings.TrimSuffix(base, " (deleted)")
		if strings.HasPrefix(base, "tuic-server") {
			return true
		}
	}
	// Match argv[0] from command line
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil && len(cmdline) > 0 {
		args := strings.Split(string(cmdline), "\x00")
		if len(args) > 0 {
			argv0 := filepath.Base(args[0])
			if strings.HasPrefix(argv0, "tuic-server") {
				return true
			}
		}
	}
	return false
}

func cleanupLegacyFiles() {
	binDir := config.GetBinFolderPath()
	if binDir == "" {
		binDir = "bin"
	}
	_ = os.RemoveAll(filepath.Join(binDir, "tuic"))
	_ = os.Remove(filepath.Join(binDir, "tuic-server"))
}
