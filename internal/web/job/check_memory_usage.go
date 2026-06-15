package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/shirou/gopsutil/v4/mem"
)

// CheckMemJob monitors memory usage and publishes events when threshold is exceeded.
type CheckMemJob struct {
	settingService service.SettingService
}

// NewCheckMemJob creates a new memory monitoring job instance.
func NewCheckMemJob() *CheckMemJob {
	return new(CheckMemJob)
}

// Run checks memory usage and publishes a memory.high event with raw metric data.
func (j *CheckMemJob) Run() {
	memInfo, err := mem.VirtualMemory()
	if err != nil || memInfo == nil {
		return
	}

	if EventBus != nil {
		EventBus.Publish(eventbus.Event{
			Type: eventbus.EventMemoryHigh,
			Data: &eventbus.SystemMetricData{
				Percent: memInfo.UsedPercent,
			},
		})
	}
}
