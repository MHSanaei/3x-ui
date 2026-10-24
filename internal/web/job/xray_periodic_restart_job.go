package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// XrayPeriodicRestartJob periodically restarts Xray according to the configured schedule.
type XrayPeriodicRestartJob struct {
	xrayService service.XrayService
}

// NewXrayPeriodicRestartJob creates a new periodic Xray restart job instance.
func NewXrayPeriodicRestartJob() *XrayPeriodicRestartJob {
	return new(XrayPeriodicRestartJob)
}

// Run executes the scheduled restart of Xray.
func (j *XrayPeriodicRestartJob) Run() {
	logger.Info("Executing scheduled periodic Xray restart...")
	if err := j.xrayService.RestartXray(false); err != nil {
		logger.Warning("Scheduled periodic Xray restart failed:", err)
	} else {
		logger.Info("Scheduled periodic Xray restart completed successfully")
	}
}
