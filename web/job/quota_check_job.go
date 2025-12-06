package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// QuotaCheckJob checks quota usage and throttles clients
type QuotaCheckJob struct {
	quotaService   service.QuotaService
	inboundService service.InboundService
	settingService service.SettingService
}

// NewQuotaCheckJob creates a new quota check job
func NewQuotaCheckJob() *QuotaCheckJob {
	return &QuotaCheckJob{
		quotaService:   service.QuotaService{},
		inboundService: service.InboundService{},
		settingService: service.SettingService{},
	}
}

// Run checks quota for all clients and throttles if needed
func (j *QuotaCheckJob) Run() {
	logger.Debug("Quota check job started")

	// Get all inbounds
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds for quota check:", err)
		return
	}

	if len(inbounds) == 0 {
		return
	}

	for i := range inbounds {
		inbound := inbounds[i]
		quotaInfos, err := j.quotaService.GetQuotaInfo(inbound)
		if err != nil {
			logger.Warningf("Failed to get quota info for inbound %s: %v", inbound.Tag, err)
			continue
		}

		for _, quotaInfo := range quotaInfos {
			// Throttle if quota exceeded
			if quotaInfo.Status == "exceeded" {
				j.quotaService.ThrottleClient(quotaInfo.Email, inbound, true)
				logger.Infof("Throttled client %s due to quota exceeded", quotaInfo.Email)
			} else if quotaInfo.Status == "warning" {
				// Send warning notification
				logger.Infof("Client %s quota warning: %.2f%% used", quotaInfo.Email, quotaInfo.UsagePercent)
			}
		}
	}
}
