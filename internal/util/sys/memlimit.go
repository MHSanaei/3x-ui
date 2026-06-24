package sys

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

// memLimitHeadroomPercent is the share of detected memory used for the soft
// limit, leaving room for non-heap (stacks, mmap, the xray child) before the OS
// OOM-kills the process.
const memLimitHeadroomPercent = 90

// ApplyMemoryLimit sets a Go soft memory limit (the runtime's GOMEMLIMIT) when
// one is not already configured, so a long-running panel in a memory-capped
// container or VPS triggers GC as it approaches the cap instead of growing RSS
// until the OS OOM-kills it. Precedence: an explicit GOMEMLIMIT env is left to
// the runtime; otherwise XUI_MEMORY_LIMIT (in MiB) wins; otherwise the limit is
// derived from the cgroup memory limit, falling back to total system RAM.
// Returns the limit applied in bytes (0 when none) and a short source label.
func ApplyMemoryLimit() (int64, string) {
	if strings.TrimSpace(os.Getenv("GOMEMLIMIT")) != "" {
		return 0, "GOMEMLIMIT env (handled by the Go runtime)"
	}

	if v := strings.TrimSpace(os.Getenv("XUI_MEMORY_LIMIT")); v != "" {
		if mb, err := strconv.ParseInt(v, 10, 64); err == nil && mb > 0 {
			limit := mb << 20
			debug.SetMemoryLimit(limit)
			return limit, "XUI_MEMORY_LIMIT=" + v + "MiB"
		}
	}

	total, source := detectAvailableMemory()
	if total <= 0 {
		return 0, "undetectable; left at Go default"
	}
	limit := total / 100 * memLimitHeadroomPercent
	debug.SetMemoryLimit(limit)
	return limit, source
}

func detectAvailableMemory() (int64, string) {
	if v, ok := cgroupMemoryLimit(); ok {
		return v, "cgroup limit"
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm.Total > 0 {
		return int64(vm.Total), "system RAM"
	}
	return 0, ""
}

// cgroupMemoryLimit reads the container memory limit from cgroup v2 then v1.
// A "max" value or the v1 unlimited sentinel (~8 EiB) means no limit at this
// level, so it reports not-found and the caller falls back to system RAM. The
// files are absent off Linux, which also yields not-found.
func cgroupMemoryLimit() (int64, bool) {
	const unlimited = int64(1) << 62

	if b, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" && s != "max" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 && v < unlimited {
				return v, true
			}
		}
	}

	if b, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil && v > 0 && v < unlimited {
			return v, true
		}
	}

	return 0, false
}
