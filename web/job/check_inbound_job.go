package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type CheckInboundJob struct {
	xrayService    service.XrayService
	inboundService service.InboundService
}

func NewCheckInboundJob() *CheckInboundJob {
	return new(CheckInboundJob)
}

func (j *CheckInboundJob) Run() {
	needRestart, count, err := j.inboundService.DisableInvalidClients()
	if err != nil {
		logger.Warning("Error in disabling invalid clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients disabled", count)
		if needRestart {
			j.xrayService.SetToNeedRestart()
		}
	}

	count, err = j.inboundService.DisableInvalidInbounds()
	if err != nil {
		logger.Warning("Error in disabling invalid inbounds:", err)
	} else if count > 0 {
		logger.Debugf("%v inbounds disabled", count)
		j.xrayService.SetToNeedRestart()
	}
}
