package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type XrayTrafficJob struct {
	xrayService    service.XrayService
	inboundService service.InboundService
}

func NewXrayTrafficJob() *XrayTrafficJob {
	return new(XrayTrafficJob)
}

func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}

	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Warning("get xray traffic failed:", err)
		return
	}
	err, needRestart := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add traffic failed:", err)
	}
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}

}
