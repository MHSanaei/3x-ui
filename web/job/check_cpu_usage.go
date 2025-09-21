package job

import (
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CheckCpuJob monitors CPU usage and sends Telegram notifications when usage exceeds the configured threshold.
type CheckCpuJob struct {
	tgbotService   service.Tgbot
	settingService service.SettingService
}

// NewCheckCpuJob creates a new CPU monitoring job instance.
func NewCheckCpuJob() *CheckCpuJob {
	return new(CheckCpuJob)
}

// Run checks CPU usage over the last minute and sends a Telegram alert if it exceeds the threshold.
func (j *CheckCpuJob) Run() {
	threshold, _ := j.settingService.GetTgCpu()

	// get latest status of server
	percent, err := cpu.Percent(1*time.Minute, false)
	if err == nil && percent[0] > float64(threshold) {
		msg := j.tgbotService.I18nBot("tgbot.messages.cpuThreshold",
			"Percent=="+strconv.FormatFloat(percent[0], 'f', 2, 64),
			"Threshold=="+strconv.Itoa(threshold))

		j.tgbotService.SendMsgToTgbotAdmins(msg)
	}
}
