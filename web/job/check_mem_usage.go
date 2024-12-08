package job

import (
	"strconv"

	"x-ui/web/service"
	"x-ui/logger"

	"github.com/shirou/gopsutil/v4/mem"
)

type CheckMemJob struct {
	tgbotService   service.Tgbot
	settingService service.SettingService
	serverService  service.ServerService
}

func NewCheckMemJob() *CheckMemJob {
	return new(CheckMemJob)
}

// Here run is a interface method of Job interface
func (j *CheckMemJob) Run() {
	threshold, _ := j.settingService.GetTgMem()
	needRestart, _ := j.settingService.GetRestartAtMemThreshold()

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Error("CheckMemJob -- get virtual memory failed:", err)
	} else {
		currentMem := memInfo.Used
		totalMem := memInfo.Total
		percentMem := int(currentMem / totalMem * 100)

		if percentMem >= int(threshold) && bool(needRestart) == true {
			msg := j.tgbotService.I18nBot("tgbot.messages.memThreshold", "Threshold=="+strconv.Itoa(threshold))
			j.tgbotService.SendMsgToTgbotAdmins(msg)

			err := j.serverService.RestartXrayService()
			if err != nil {
				logger.Error("CheckMemJob -- RestartXrayService failed:", err)
			} else {
				logger.Info("CheckMemJob -- RestartXrayService success")
			}
		}
	}
}
