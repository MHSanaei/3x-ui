package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type Period string

type PeriodicTrafficResetJob struct {
	inboundService service.InboundService
	period         Period
}

func NewPeriodicTrafficResetJob(period Period) *PeriodicTrafficResetJob {
	return &PeriodicTrafficResetJob{
		period: period,
	}
}

func (j *PeriodicTrafficResetJob) Run() {
	inbounds, err := j.inboundService.GetInboundsByTrafficReset(string(j.period))
	logger.Infof("Running periodic traffic reset job for period: %s", j.period)
	if err != nil {
		logger.Warning("Failed to get inbounds for traffic reset:", err)
		return
	}

	resetCount := 0

	for _, inbound := range inbounds {
		if err := j.inboundService.ResetAllClientTraffics(inbound.Id); err != nil {
			logger.Warning("Failed to reset traffic for inbound", inbound.Id, ":", err)
			continue
		}

		resetCount++
		logger.Infof("Reset traffic for inbound %d (%s)", inbound.Id, inbound.Remark)
	}

	if resetCount > 0 {
		logger.Infof("Periodic traffic reset completed: %d inbounds reset", resetCount)
	}
}
