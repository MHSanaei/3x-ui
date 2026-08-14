package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/integration"
)

type WarpIpJob struct {
	settingService service.SettingService
	warpService    integration.WarpService
	xrayService    service.XrayService
}

func NewWarpIpJob() *WarpIpJob {
	return &WarpIpJob{}
}

func (j *WarpIpJob) Run() {
	allSetting, err := j.settingService.GetAllSetting()
	if err != nil {
		return
	}
	interval := allSetting.WarpUpdateInterval
	if interval <= 0 {
		return
	}

	lastUpdate, err := j.settingService.GetWarpLastUpdate()
	if err != nil {
		logger.Warning("Failed to read scheduled WARP IP update time: ", err)
		return
	}
	now := time.Now().Unix()

	// First run after the feature is enabled (e.g. interval set via direct
	// DB edit): establish a baseline instead of rotating immediately.
	if lastUpdate == 0 {
		if err := j.settingService.SetWarpLastUpdate(now); err != nil {
			logger.Warning("Failed to establish scheduled WARP IP update time: ", err)
		}
		return
	}

	if now-lastUpdate >= int64(interval*24*3600) {
		logger.Info("Starting scheduled WARP IP update...")
		_, err := j.warpService.ChangeWarpIP()
		if err != nil {
			logger.Warning("Failed to update WARP IP: ", err)
			return
		}

		if err := j.settingService.SetWarpLastUpdate(now); err != nil {
			logger.Warning("WARP IP changed but the next-update time was not saved: ", err)
		}
		j.xrayService.SetToNeedRestart()
		logger.Info("Successfully updated WARP IP and scheduled Xray restart")
	}
}
