package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type xrayRestartRunner interface {
	IsXrayRunning() bool
	RestartXray(isForce bool) error
}

// XrayPeriodicRestartJob periodically restarts Xray according to the configured schedule.
type XrayPeriodicRestartJob struct {
	xrayService xrayRestartRunner
}

// NewXrayPeriodicRestartJob creates a new periodic Xray restart job instance.
func NewXrayPeriodicRestartJob() *XrayPeriodicRestartJob {
	return &XrayPeriodicRestartJob{
		xrayService: &service.XrayService{},
	}
}

// Run executes the scheduled restart of Xray.
func (j *XrayPeriodicRestartJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		logger.Info("Skipping scheduled periodic Xray restart: Xray is not running")
		return
	}
	logger.Info("Executing scheduled periodic Xray restart...")
	if err := j.xrayService.RestartXray(true); err != nil {
		logger.Warning("Scheduled periodic Xray restart failed:", err)
	} else {
		logger.Info("Scheduled periodic Xray restart completed successfully")
	}
}

