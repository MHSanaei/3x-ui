package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// AuditCleanupJob cleans up old audit logs
type AuditCleanupJob struct {
	auditService   service.AuditLogService
	settingService service.SettingService
}

// NewAuditCleanupJob creates a new audit cleanup job
func NewAuditCleanupJob() *AuditCleanupJob {
	return &AuditCleanupJob{
		auditService:   service.AuditLogService{},
		settingService: service.SettingService{},
	}
}

// Run cleans up old audit logs
func (j *AuditCleanupJob) Run() {
	logger.Debug("Audit cleanup job started")

	retentionDays, err := j.settingService.GetAuditLogRetentionDays()
	if err != nil || retentionDays <= 0 {
		retentionDays = 90 // Default 90 days
	}

	err = j.auditService.CleanOldLogs(retentionDays)
	if err != nil {
		logger.Warning("Failed to clean old audit logs:", err)
	} else {
		logger.Debugf("Audit cleanup completed (retention: %d days)", retentionDays)
	}
}
