package sys

import "github.com/shirou/gopsutil/v4/cpu"

// CPUBusyPercent is the busy share of CPU time between two cumulative samples;
// ok is false when no time elapsed between them or the counters reset.
func CPUBusyPercent(prev, cur cpu.TimesStat) (percent float64, ok bool) {
	// Guest and GuestNice are already counted inside User and Nice on Linux.
	idle := cur.Idle - prev.Idle
	busy := (cur.User - prev.User) +
		(cur.System - prev.System) +
		(cur.Nice - prev.Nice) +
		(cur.Iowait - prev.Iowait) +
		(cur.Irq - prev.Irq) +
		(cur.Softirq - prev.Softirq) +
		(cur.Steal - prev.Steal)
	total := busy + idle
	if total <= 0 {
		return 0, false
	}
	return min(max(100*busy/total, 0), 100), true
}
