package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type PeriodicClientTrafficResetJob struct {
	inboundService service.InboundService
	period         Period
}

func NewPeriodicClientTrafficResetJob(period Period) *PeriodicClientTrafficResetJob {
	return &PeriodicClientTrafficResetJob{
		period: period,
	}
}

func (j *PeriodicClientTrafficResetJob) Run() {
	clients, err := j.inboundService.GetClientsByTrafficReset(string(j.period))
	logger.Infof("Running periodic client traffic reset job for period: %s", j.period)
	if err != nil {
		logger.Warning("Failed to get clients for traffic reset:", err)
		return
	}

	resetCount := 0

	for _, client := range clients {
		if err := j.inboundService.ResetClientTrafficByEmail(client.Email); err != nil {
			logger.Warning("Failed to reset traffic for client", client.Email, ":", err)
			continue
		}

		resetCount++
		logger.Infof("Reset traffic for client %s", client.Email)
	}

	if resetCount > 0 {
		logger.Infof("Periodic client traffic reset completed: %d clients reseted", resetCount)
	}
}
