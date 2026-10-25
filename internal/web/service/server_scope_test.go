//go:build linux

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
)

func TestContainerStatusOnlyOverridesCPUSwapAndUptime(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", root)
	dbtest.InitDB(t, filepath.Join(root, "test.db"))
	sys.SelfRSS()
	proc, cg := filepath.Join(root, "proc"), filepath.Join(root, "cg")
	fixtures := map[string]string{
		filepath.Join(proc, "self/mountinfo"): "1 0 0:92 /proc/meminfo /proc/meminfo rw - fuse.lxcfs lxcfs rw\n2 0 0:92 /proc/cpuinfo /proc/cpuinfo rw - fuse.lxcfs lxcfs rw\n3 0 0:92 /proc/uptime /proc/uptime rw - fuse.lxcfs lxcfs rw\n4 0 0:28 / " + cg + " rw - cgroup2 cgroup rw\n",
		filepath.Join(proc, "1/cgroup"):       "0::/init.scope\n",
		filepath.Join(proc, "meminfo"):        "MemTotal: 2097152 kB\nMemAvailable: 1966080 kB\nSwapTotal: 524288 kB\nSwapFree: 505384 kB\n",
		filepath.Join(proc, "cpuinfo"):        "processor : 0\ncpu cores : 16\nprocessor : 1\ncpu cores : 16\n",
		filepath.Join(proc, "uptime"):         "52624.93 52624.93\n",
		filepath.Join(proc, "loadavg"):        "1.25 2.50 3.75 1/5 1\n",
		filepath.Join(proc, "diskstats"):      "8 0 vda 1 0 2 0 1 0 4 0 0 0 0\n",
		filepath.Join(proc, "stat"):           "cpu 100 0 50 1000 0 0 0 0\n",
		filepath.Join(proc, "net/dev"):        "Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n",
		filepath.Join(cg, "cpu.stat"):         "usage_usec 222772076\n",
		filepath.Join(cg, "cpu.max"):          "max 100000\n",
		filepath.Join(cg, "io.stat"):          "259:0 rbytes=999999999 wbytes=999999999\n",
	}
	for path, content := range fixtures {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOST_PROC", proc)
	t.Setenv("HOST_SYS", filepath.Join(root, "sys"))
	svc := &ServerService{cachedIPv4: "192.0.2.1", cachedIPv6: "N/A", nextIPv4Lookup: time.Now().Add(time.Hour), nextIPv6Lookup: time.Now().Add(time.Hour), cachedCpuSpeedMhz: 5000, lxcMetrics: sys.LXCReader{ProcRoot: proc, CgroupRoot: cg}}
	previous := &Status{T: time.Now().Add(-time.Second)}
	got := svc.GetStatus(previous)
	if got.ResourceScope != "container" || got.CpuCores != 2 || got.LogicalPro != 2 || got.CpuSpeedMhz != 0 || got.Swap.Total != 512<<20 || got.Swap.Current != 19357696 || got.Uptime != 52624 {
		t.Fatalf("incorrect corrected resources: %+v", got)
	}
	if got.Mem.Total != 2<<30 || got.Mem.Current != 128<<20 || !reflect.DeepEqual(got.Loads, []float64{1.25, 2.5, 3.75}) || got.DiskTraffic.Read != 1024 || got.DiskTraffic.Write != 2048 || got.DiskIO.Read == 0 || got.DiskIO.Write == 0 {
		t.Fatalf("unrelated metric path changed: %+v", got)
	}
	if !slices.Equal(got.UnavailableMetrics, []string{"cpu"}) {
		t.Fatalf("unexpected unavailable metrics: %v", got.UnavailableMetrics)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cpu", "cpuCores", "logicalPro", "swap", "uptime", "mem", "loads", "diskIO", "diskTraffic", "resourceScope", "unavailableMetrics"} {
		if _, ok := wire[key]; !ok {
			t.Fatalf("API field lost: %s", key)
		}
	}
	if err = os.WriteFile(filepath.Join(proc, "self/mountinfo"), []byte("1 0 0:1 / / rw - ext4 /dev/root rw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	native := svc.GetStatus(previous)
	if native.ResourceScope != "" || len(native.UnavailableMetrics) != 0 || native.CpuSpeedMhz != 5000 || native.Mem != got.Mem || !reflect.DeepEqual(native.Loads, got.Loads) || native.DiskTraffic != got.DiskTraffic {
		t.Fatalf("native regression: %+v", native)
	}
}

func TestScopedResourcesLeaveUnrelatedStatusUntouched(t *testing.T) {
	status := &Status{Loads: []float64{1, 2, 3}, CpuSpeedMhz: 0}
	status.Mem.Current, status.Mem.Total = 123, 456
	status.DiskTraffic.Read, status.DiskTraffic.Write = 99, 101
	status.DiskIO.Read, status.DiskIO.Write = 11, 12
	before := *status
	applyLXCStats(status, &sys.LXCStats{Scope: "container", CPUs: 2, SwapUsed: 8, SwapTotal: 512, Uptime: 15})
	if status.Mem != before.Mem || status.DiskTraffic != before.DiskTraffic || status.DiskIO != before.DiskIO || !reflect.DeepEqual(status.Loads, before.Loads) {
		t.Fatalf("unrelated status overwritten: %+v", status)
	}
}

func TestContainerHistorySeparatesOnlyCPUAndSwap(t *testing.T) {
	old := systemMetrics
	systemMetrics = newMetricHistory()
	defer func() { systemMetrics = old }()
	now := time.Now()
	systemMetrics.append("cpu", now, 90)
	systemMetrics.append("swap", now, 62)
	for key, value := range map[string]float64{"mem": 6, "load1": 1.25, "diskRead": 1024, "netUp": 7} {
		systemMetrics.append(key, now, value)
	}
	status := &Status{T: now, ResourceScope: "container", Cpu: 10, Loads: []float64{1.25, 2.5, 3.75}, UnavailableMetrics: []string{"swap"}}
	status.Mem.Total, status.Mem.Current = 100, 6
	status.DiskIO.Read, status.NetIO.Up = 1024, 7
	svc := &ServerService{lastStatus: status}
	svc.AppendStatusSample(now, status)
	cpu := svc.AggregateCpuHistory(2, 60)
	if len(cpu) != 1 || cpu[0]["cpu"] != float64(10) {
		t.Fatalf("CPU history=%v", cpu)
	}
	native := &ServerService{lastStatus: &Status{}}
	if host := native.AggregateCpuHistory(2, 60); len(host) != 1 || host[0]["cpu"] != float64(90) {
		t.Fatalf("legacy CPU changed: %v", host)
	}
	if points := svc.AggregateSystemMetric("swap", 2, 60); len(points) != 0 {
		t.Fatalf("missing swap became zero: %v", points)
	}
	for key, want := range map[string]float64{"mem": 6, "load1": 1.25, "diskRead": 1024, "netUp": 7} {
		if svc.systemMetricKey(key) != key {
			t.Fatalf("unrelated history renamed: %s", key)
		}
		points := svc.AggregateSystemMetric(key, 2, 60)
		if len(points) != 1 || points[0]["v"] != want {
			t.Fatalf("unrelated %s history=%v", key, points)
		}
	}
	status.UnavailableMetrics = nil
	svc.AppendStatusSample(now, status)
	if points := svc.AggregateSystemMetric("swap", 2, 60); len(points) != 1 || points[0]["v"] != float64(0) {
		t.Fatalf("disabled swap lost: %v", points)
	}
	snapshot := systemMetrics.snapshot()
	for _, key := range []string{"cpu", "swap", containerHistoryPrefix + "cpu", containerHistoryPrefix + "swap"} {
		if _, ok := snapshot[key]; !ok {
			t.Fatalf("missing preserved series: %s", key)
		}
	}
	for key := range snapshot {
		if len(key) > len(containerHistoryPrefix) && key[:len(containerHistoryPrefix)] == containerHistoryPrefix && key != containerHistoryPrefix+"cpu" && key != containerHistoryPrefix+"swap" {
			t.Fatalf("unexpected scoped metric: %s", key)
		}
	}
}
