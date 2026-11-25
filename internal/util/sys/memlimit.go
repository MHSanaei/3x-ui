package sys

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
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

// cgroupMemoryLimit returns the tightest memory limit between this process's own
// cgroup and the cgroup root it can see: a systemd MemoryMax sits below that root.
func cgroupMemoryLimit() (int64, bool) {
	self, _ := os.ReadFile("/proc/self/cgroup")
	return cgroupMemoryLimitAt(string(self), "/sys/fs/cgroup")
}

func cgroupMemoryLimitAt(selfCgroup, root string) (int64, bool) {
	if limit, ok := tightestLimit(root, ownCgroupPath(selfCgroup, "0", ""), "memory.max"); ok {
		return limit, true
	}
	return tightestLimit(filepath.Join(root, "memory"), ownCgroupPath(selfCgroup, "", "memory"), "memory.limit_in_bytes")
}

// ownCgroupPath picks the /proc/self/cgroup line of the v2 hierarchy (id "0") or the
// v1 hierarchy carrying controller; "/" when there is no such line.
func ownCgroupPath(selfCgroup, id, controller string) string {
	for line := range strings.Lines(selfCgroup) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 {
			continue
		}
		if (id != "" && fields[0] == id && fields[1] == "") ||
			(controller != "" && slices.Contains(strings.Split(fields[1], ","), controller)) {
			return fields[2]
		}
	}
	return "/"
}

// tightestLimit walks from root/rel up to root. "max" and the v1 unlimited
// sentinel (~8 EiB) do not parse as a limit; files are absent off Linux.
func tightestLimit(root, rel, name string) (int64, bool) {
	const unlimited = int64(1) << 62
	var limit int64
	for dir := filepath.Join(root, filepath.Clean("/"+rel)); ; dir = filepath.Dir(dir) {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
			if err == nil && v > 0 && v < unlimited && (limit == 0 || v < limit) {
				limit = v
			}
		}
		if len(dir) <= len(root) {
			break
		}
	}
	return limit, limit > 0
}
