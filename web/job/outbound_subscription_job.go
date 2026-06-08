package job

import (
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"
)

// OutboundSubscriptionJob periodically re-fetches enabled outbound subscriptions,
// updates the stored outbounds (with stable tags), and signals that xray
// should be reloaded so the new outbounds take effect.
type OutboundSubscriptionJob struct {
	subService *service.OutboundSubscriptionService
	xraySvc    *service.XrayService
}

// NewOutboundSubscriptionJob creates the job (zero-value services are populated
// on first Run via method calls, same pattern as other jobs).
func NewOutboundSubscriptionJob() *OutboundSubscriptionJob {
	return &OutboundSubscriptionJob{
		subService: &service.OutboundSubscriptionService{},
		xraySvc:    &service.XrayService{},
	}
}

// Run is invoked by the cron scheduler.
func (j *OutboundSubscriptionJob) Run() {
	if j.subService == nil {
		j.subService = &service.OutboundSubscriptionService{}
	}
	if j.xraySvc == nil {
		j.xraySvc = &service.XrayService{}
	}

	count, err := j.subService.RefreshAllEnabled()
	if err != nil {
		logger.Warning("outbound subscription auto-update error:", err)
		return
	}
	if count > 0 {
		logger.Infof("Refreshed %d outbound subscription(s)", count)
		// Ask the xray manager to restart/reload on the next 30s check.
		j.xraySvc.SetToNeedRestart()
		// Also broadcast an invalidate so the UI can refresh the xray setting
		// view (new outbounds will be visible after the reload cycle).
		websocket.BroadcastInvalidate(websocket.MessageTypeOutbounds)
	}
}