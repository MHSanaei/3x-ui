package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

type WarpIpJob struct {
	settingService service.SettingService
	warpService    service.WarpService
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

	lastUpdate, _ := j.settingService.GetWarpLastUpdate()
	now := time.Now().Unix()

	// First run after the feature is enabled (e.g. interval set via direct
	// DB edit): establish a baseline instead of rotating immediately.
	if lastUpdate == 0 {
		_ = j.settingService.SetWarpLastUpdate(now)
		return
	}

	if now-lastUpdate >= int64(interval*24*3600) {
		logger.Info("Starting scheduled WARP IP update...")
		_, err := j.warpService.ChangeWarpIP()
		if err != nil {
			logger.Warning("Failed to update WARP IP: ", err)
			return
		}

		_ = j.settingService.SetWarpLastUpdate(now)
		j.xrayService.SetToNeedRestart()
		logger.Info("Successfully updated WARP IP and scheduled Xray restart")
	}
}
