package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type CheckXrayRunningJob struct {
	xrayService service.XrayService

	checkTime int
}

func NewCheckXrayRunningJob() *CheckXrayRunningJob {
	return new(CheckXrayRunningJob)
}

func (j *CheckXrayRunningJob) Run() {
	if j.xrayService.IsXrayRunning() {
		j.checkTime = 0
	} else {
		j.checkTime++
		//only restart if it's down 2 times in a row
		if j.checkTime > 1 {
			err := j.xrayService.RestartXray(false)
			j.checkTime = 0
			if err != nil {
				logger.Error("Restart xray failed:", err)
			}
		}
	}
}
