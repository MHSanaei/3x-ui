// Package job provides background job implementations for the 3x-ui web panel,
// including traffic monitoring, system checks, and periodic maintenance tasks.
package job

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// EventBus is set from web layer to publish events.
var EventBus *eventbus.Bus

// CheckXrayRunningJob monitors the selected proxy core and restarts it if it crashes.
type CheckXrayRunningJob struct {
	xrayService  service.XrayService
	singBox      service.SingBoxService
	setting      service.SettingService
	checkTime    int
}

// NewCheckXrayRunningJob creates the core health check job instance.
func NewCheckXrayRunningJob() *CheckXrayRunningJob {
	return new(CheckXrayRunningJob)
}

// Run checks the selected core and restarts it after it is down for 2 consecutive checks.
func (j *CheckXrayRunningJob) Run() {
	coreType, err := j.setting.GetCoreType()
	if err != nil {
		coreType = service.CoreTypeXray
	}
	if coreType == service.CoreTypeSingBox {
		if !j.singBox.IsRunning() {
			j.checkTime++
			if j.checkTime > 1 {
				err := j.singBox.Restart(context.Background())
				j.checkTime = 0
				if err != nil {
					logger.Error("Restart sing-box failed:", err)
				}
			}
			return
		}
		j.checkTime = 0
		return
	}
	if !j.xrayService.DidXrayCrash() {
		j.checkTime = 0
		return
	}
	j.checkTime++
	if j.checkTime > 1 {
		err := j.xrayService.RestartXray(false)
		j.checkTime = 0
		if err != nil {
			logger.Error("Restart xray failed:", err)
		}
	}
}
