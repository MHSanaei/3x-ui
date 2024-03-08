package job

import (
	"strconv"
	"time"

	"x-ui/web/service"

	"github.com/shirou/gopsutil/v3/cpu"
)

type CheckCpuJob struct {
	tgbotService   service.Tgbot
	settingService service.SettingService
}

func NewCheckCpuJob() *CheckCpuJob {
	return new(CheckCpuJob)
}

// Here run is a interface method of Job interface
func (j *CheckCpuJob) Run() {
	threshold, _ := j.settingService.GetTgCpu()

	// get latest status of server
	percent, err := cpu.Percent(1*time.Second, false)
	if err == nil && percent[0] > float64(threshold) {
		msg := j.tgbotService.I18nBot("tgbot.messages.cpuThreshold",
			"Percent=="+strconv.FormatFloat(percent[0], 'f', 2, 64),
			"Threshold=="+strconv.Itoa(threshold))

		j.tgbotService.SendMsgToTgbotAdmins(msg)
	}
}
