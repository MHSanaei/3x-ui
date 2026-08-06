package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// RemoteRoutingJob keeps permanent Happ and Clash/Mihomo routing URLs warm.
// Network work runs in cron (and once asynchronously at panel startup), never
// in a subscription request handler.
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
	sub.RefreshRemoteRoutingSources(happ, clash)
}
