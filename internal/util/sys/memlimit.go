package sys

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	memLimitHeadroomPercent = 90
	defaultGCPercent        = 75
	defaultReleaseMinutes   = 10
)

// ApplyMemoryTuning configures the Go runtime for a lower, steadier footprint and
// returns one log line per decision. It does NOT derive a soft limit from total
// system RAM: on a shared or uncontrolled host that gives no benefit (GOGC, not
// the limit, paces GC while the heap is far below it) and risks GC thrashing, so
// memory is kept low via GOGC plus the periodic release job instead.
func ApplyMemoryTuning() []string {
	lines := []string{applyGCPercent()}
	if limit, source := applyMemoryLimit(); limit > 0 {
		lines = append(lines, fmt.Sprintf("Go memory soft limit set to %d MiB (%s)", limit>>20, source))
	} else {
		lines = append(lines, "Go memory soft limit not enforced: "+source)
	}
	return lines
}

// applyGCPercent lowers GOGC so the heap high-water mark, and thus RSS, stays
// smaller. An explicit GOGC env (including GOGC=off) is left to the runtime.
func applyGCPercent() string {
	if _, ok := os.LookupEnv("GOGC"); ok {
		return "GC percent: GOGC env (handled by the Go runtime)"
	}

	pct := defaultGCPercent
	if v := strings.TrimSpace(os.Getenv("XUI_GOGC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pct = n
		}
	}

	if pct <= 0 {
		return "GC percent left at Go default"
	}
	debug.SetGCPercent(pct)
	return fmt.Sprintf("GC percent set to %d", pct)
}

// applyMemoryLimit sets the soft limit only from an explicit budget: GOMEMLIMIT
// env (left to the runtime), XUI_MEMORY_LIMIT in MiB, or a real cgroup limit at
// 90% to leave headroom for non-heap and the xray child. No budget -> Go default.
func applyMemoryLimit() (int64, string) {
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

	if v, ok := cgroupMemoryLimit(); ok {
		limit := v / 100 * memLimitHeadroomPercent
		debug.SetMemoryLimit(limit)
		return limit, "cgroup limit"
	}

	return 0, "no explicit budget; soft limit left at Go default"
}

// MemoryReleaseIntervalMinutes reports how often freed heap memory is returned to
// the OS via debug.FreeOSMemory. XUI_MEMORY_RELEASE_INTERVAL overrides the
// default; an explicit 0 disables the periodic release.
func MemoryReleaseIntervalMinutes() int {
	v := strings.TrimSpace(os.Getenv("XUI_MEMORY_RELEASE_INTERVAL"))
	if v == "" {
		return defaultReleaseMinutes
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return n
	}
	return defaultReleaseMinutes
}

// cgroupMemoryLimit reads the container memory limit from cgroup v2 then v1.
// A "max" value or the v1 unlimited sentinel (~8 EiB) means no limit at this
// level, so it reports not-found and the caller falls back to the Go default. The
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
