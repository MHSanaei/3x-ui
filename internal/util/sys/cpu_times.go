package sys

import "github.com/shirou/gopsutil/v4/cpu"

// CPUBusyPercent is the busy share of CPU time between two cumulative samples,
// counting iowait as busy; ok is false when no time elapsed or the counters reset.
func CPUBusyPercent(prev, cur cpu.TimesStat) (percent float64, ok bool) {
	return cpuPercent(prev, cur, true)
}

// CPUWorkPercent is CPUBusyPercent with iowait counted as idle, as gopsutil's
// cpu.Percent does, so a host waiting on disk is not reported as CPU-bound.
func CPUWorkPercent(prev, cur cpu.TimesStat) (percent float64, ok bool) {
	return cpuPercent(prev, cur, false)
}

func cpuPercent(prev, cur cpu.TimesStat, iowaitIsBusy bool) (float64, bool) {
	// Guest and GuestNice are already counted inside User and Nice on Linux.
	idle := cur.Idle - prev.Idle
	busy := (cur.User - prev.User) +
		(cur.System - prev.System) +
		(cur.Nice - prev.Nice) +
		(cur.Irq - prev.Irq) +
		(cur.Softirq - prev.Softirq) +
		(cur.Steal - prev.Steal)
	if iowait := cur.Iowait - prev.Iowait; iowaitIsBusy {
		busy += iowait
	} else {
		idle += iowait
	}
	total := busy + idle
	if total <= 0 {
		return 0, false
	}
	return min(max(100*busy/total, 0), 100), true
}
