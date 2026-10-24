//go:build linux

package sys

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LXCReader struct {
	ProcRoot, CgroupRoot string
	mu                   sync.Mutex
	lastCPU              uint64
	lastAt               time.Time
	lastCapacity         float64
	sampled              bool
}

func (r *LXCReader) roots() (string, string) {
	proc := r.ProcRoot
	if proc == "" {
		proc = HostProc()
	}
	cg := r.CgroupRoot
	if cg == "" {
		cg = "/sys/fs/cgroup"
	}
	return proc, cg
}

type lxcMounts struct {
	sources    map[string]bool
	cgroupRoot bool
}

func (r *LXCReader) detect() (lxcMounts, error) {
	proc, cg := r.roots()
	data, err := os.ReadFile(filepath.Join(proc, "self/mountinfo"))
	if err != nil {
		return lxcMounts{}, err
	}
	mounts := lxcMounts{sources: map[string]bool{}}
	valid := false
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}
		mount, fs := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(mount) <= 4 || len(fs) == 0 {
			continue
		}
		valid = true
		if fs[0] == "cgroup2" && mount[3] == "/" && mount[4] == cg {
			mounts.cgroupRoot = true
		}
		if fs[0] == "fuse.lxcfs" {
			switch mount[4] {
			case "/proc/meminfo", "/proc/cpuinfo", "/proc/uptime":
				mounts.sources[mount[4]] = true
			}
		}
	}
	if !valid {
		return lxcMounts{}, errors.New("invalid mountinfo")
	}
	return mounts, nil
}

func (r *LXCReader) UseScopedHistory() bool {
	mounts, err := r.detect()
	return len(mounts.sources) > 0 || err != nil
}

func (r *LXCReader) resetCPU() {
	r.mu.Lock()
	r.sampled = false
	r.mu.Unlock()
}

// LXCFS supplies container swap and uptime; sysinfo bypasses those mounts.
func (r *LXCReader) Read(now time.Time) (*LXCStats, error) {
	mounts, err := r.detect()
	if len(mounts.sources) == 0 && err == nil {
		r.resetCPU()
		return nil, nil
	}
	out := &LXCStats{Scope: "container"}
	if err != nil {
		r.resetCPU()
		out.Scope = "unknown"
		out.Unavailable = []string{"cpu", "swap", "uptime"}
		return out, fmt.Errorf("cannot determine metric scope: %w", err)
	}
	proc, cg := r.roots()
	if mounts.sources["/proc/meminfo"] {
		mem, err := readLXCBytes(filepath.Join(proc, "meminfo"))
		total, tok := mem["SwapTotal"]
		free, fok := mem["SwapFree"]
		if err == nil && tok && fok && free <= total {
			out.SwapTotal = total
			out.SwapUsed = total - free
		} else {
			out.Unavailable = append(out.Unavailable, "swap")
		}
	} else {
		out.Unavailable = append(out.Unavailable, "swap")
	}
	if mounts.sources["/proc/uptime"] {
		data, err := os.ReadFile(filepath.Join(proc, "uptime"))
		fields := strings.Fields(string(data))
		if err == nil && len(fields) > 0 {
			value, e := strconv.ParseFloat(fields[0], 64)
			if e == nil && !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value < float64(math.MaxUint64) {
				out.Uptime = uint64(value)
			} else {
				err = errors.New("invalid uptime")
			}
		} else {
			err = errors.New("missing uptime")
		}
		if err != nil {
			out.Unavailable = append(out.Unavailable, "uptime")
		}
	} else {
		out.Unavailable = append(out.Unavailable, "uptime")
	}
	if !mounts.sources["/proc/cpuinfo"] || !mounts.cgroupRoot || !containerInitAtRoot(proc) {
		r.resetCPU()
		out.Unavailable = append(out.Unavailable, "cpu")
		return out, nil
	}
	capacity, cpus, err := r.cpuCapacity(proc, cg)
	out.CPUs = cpus
	usage, usageErr := readUsage(filepath.Join(cg, "cpu.stat"))
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil || usageErr != nil {
		r.sampled = false
		out.Unavailable = append(out.Unavailable, "cpu")
	} else {
		elapsed := now.Sub(r.lastAt).Seconds()
		if r.sampled && elapsed > 0 && usage >= r.lastCPU && capacity == r.lastCapacity {
			out.CPU = math.Min(100, float64(usage-r.lastCPU)/1e6/elapsed/capacity*100)
		} else {
			out.Unavailable = append(out.Unavailable, "cpu")
		}
		r.lastCPU, r.lastAt, r.lastCapacity, r.sampled = usage, now, capacity, true
	}
	return out, nil
}

// PID 1 must belong to this namespace root, not a host-visible LXC subtree.
func containerInitAtRoot(proc string) bool {
	data, err := os.ReadFile(filepath.Join(proc, "1/cgroup"))
	if err != nil {
		return false
	}
	switch strings.TrimSpace(string(data)) {
	case "0::/", "0::/init.scope":
		return true
	default:
		return false
	}
}

func readLXCBytes(path string) (map[string]uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{"SwapTotal": true, "SwapFree": true}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if !wanted[key] {
			continue
		}
		if len(fields) != 3 || fields[2] != "kB" {
			return nil, errors.New("invalid meminfo unit")
		}
		if _, exists := values[key]; exists {
			return nil, errors.New("duplicate meminfo field")
		}
		value, e := strconv.ParseUint(fields[1], 10, 64)
		if e != nil || value > math.MaxUint64/1024 {
			return nil, errors.New("invalid meminfo value")
		}
		values[key] = value * 1024
	}
	return values, nil
}

func (r *LXCReader) cpuCapacity(proc, cg string) (float64, int, error) {
	data, err := os.ReadFile(filepath.Join(proc, "cpuinfo"))
	if err != nil {
		return 0, 0, err
	}
	ids := map[uint64]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "processor" {
			continue
		}
		id, e := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if e != nil || ids[id] {
			return 0, 0, errors.New("invalid CPU list")
		}
		ids[id] = true
	}
	cpus := len(ids)
	if cpus == 0 {
		return 0, 0, errors.New("empty CPU list")
	}
	capacity := float64(cpus)
	data, err = os.ReadFile(filepath.Join(cg, "cpu.max"))
	if err != nil {
		return 0, cpus, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return 0, cpus, errors.New("invalid CPU quota")
	}
	period, e := strconv.ParseUint(fields[1], 10, 64)
	if e != nil || period == 0 {
		return 0, cpus, errors.New("invalid CPU period")
	}
	if fields[0] != "max" {
		quota, e := strconv.ParseUint(fields[0], 10, 64)
		if e != nil || quota == 0 {
			return 0, cpus, errors.New("invalid CPU quota")
		}
		capacity = math.Min(capacity, float64(quota)/float64(period))
	}
	return capacity, cpus, nil
}

func readUsage(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var usage uint64
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "usage_usec" {
			continue
		}
		if len(fields) != 2 || found {
			return 0, errors.New("invalid cgroup CPU usage")
		}
		usage, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		found = true
	}
	if !found {
		return 0, errors.New("missing cgroup CPU usage")
	}
	return usage, nil
}
