package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CheckCpuJob monitors CPU usage and publishes events when threshold is exceeded.
type CheckCpuJob struct {
	settingService service.SettingService
}

// NewCheckCpuJob creates a new CPU monitoring job instance.
func NewCheckCpuJob() *CheckCpuJob {
	return new(CheckCpuJob)
}

// Run checks CPU usage and publishes a cpu.high event with raw metric data.
func (j *CheckCpuJob) Run() {
	percent, err := cpu.Percent(1*time.Minute, false)
	if err != nil || len(percent) == 0 {
		return
	}

	if EventBus != nil {
		EventBus.Publish(eventbus.Event{
			Type: eventbus.EventCPUHigh,
			Data: &eventbus.SystemMetricData{
				Percent: percent[0],
			},
		})
	}
}
