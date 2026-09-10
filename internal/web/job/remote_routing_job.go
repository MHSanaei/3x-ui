package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// RemoteRoutingJob keeps remote Happ and Clash/Mihomo routing URLs warm: all
// network work runs here (cron + startup warm), never in a request handler.
type RemoteRoutingJob struct {
	settingService service.SettingService
}

func NewRemoteRoutingJob() *RemoteRoutingJob {
	return &RemoteRoutingJob{}
}

func (j *RemoteRoutingJob) Run() {
	happ, err := j.settingService.GetSubRoutingRules()
	if err != nil {
		logger.Warning("Could not read Happ routing source:", err)
		return
	}
	clash, err := j.settingService.GetSubClashRules()
	if err != nil {
		logger.Warning("Could not read Clash routing source:", err)
		return
	}
	jsonRouting, err := j.settingService.GetSubJsonRoutingRules()
	if err != nil {
		logger.Warning("Could not read JSON subscription routing source:", err)
		return
	}
	sub.RefreshRemoteRoutingSources(happ, clash, jsonRouting)
}
