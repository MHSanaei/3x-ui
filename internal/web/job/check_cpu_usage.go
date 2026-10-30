package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CheckCpuJob monitors CPU usage and publishes events when threshold is exceeded.
type CheckCpuJob struct {
	readTimes func(percpu bool) ([]cpu.TimesStat, error)
	last      cpu.TimesStat
	hasLast   bool
}

// NewCheckCpuJob creates a new CPU monitoring job instance.
func NewCheckCpuJob() *CheckCpuJob {
	return &CheckCpuJob{readTimes: cpu.Times}
}

// Run publishes a cpu.high event with the CPU usage since the previous run. It
// must not sample in place: cron.Stop, and so every panel restart, waits on it.
func (j *CheckCpuJob) Run() {
	times, err := j.readTimes(false)
	if err != nil || len(times) == 0 {
		return
	}
	prev, hadPrev := j.last, j.hasLast
	j.last, j.hasLast = times[0], true
	if !hadPrev {
		return
	}
	percent, ok := sys.CPUBusyPercent(prev, times[0])
	if !ok {
		return
	}

	if EventBus != nil {
		EventBus.Publish(eventbus.Event{
			Type: eventbus.EventCPUHigh,
			Data: &eventbus.SystemMetricData{
				Percent: percent,
			},
		})
	}
}
