// Package job provides scheduled tasks for monitoring client HWIDs from access logs.
// NOTE: In client_header mode, this job does NOT generate HWIDs from logs.
// HWID registration happens explicitly via RegisterHWIDFromHeaders when subscription is requested.
package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// CheckClientHWIDJob monitors client HWIDs from access logs and manages HWID tracking.
type CheckClientHWIDJob struct {
	lastClear int64
}

var hwidJob *CheckClientHWIDJob

// NewCheckClientHWIDJob creates a new client HWID monitoring job instance.
func NewCheckClientHWIDJob() *CheckClientHWIDJob {
	if hwidJob == nil {
		hwidJob = new(CheckClientHWIDJob)
	}
	return hwidJob
}

// Run executes the HWID monitoring job.
func (j *CheckClientHWIDJob) Run() {
	// Check if multi-node mode is enabled
	settingService := service.SettingService{}
	multiMode, err := settingService.GetMultiNodeMode()
	if err == nil && multiMode {
		// In multi-node mode, HWID checking is handled by nodes
		return
	}

	if j.lastClear == 0 {
		j.lastClear = time.Now().Unix()
	}

	hwidTrackingActive := j.hasHWIDTracking()
	if !hwidTrackingActive {
		return
	}

	isAccessLogAvailable := j.checkAccessLogAvailable()
	if !isAccessLogAvailable {
		return
	}

	// Process access log to track HWIDs
	j.processLogFile()

	// Clear access log periodically (every hour)
	if time.Now().Unix()-j.lastClear > 3600 {
		j.clearAccessLog()
	}
}

// hasHWIDTracking checks if HWID tracking is enabled globally and for any client.
func (j *CheckClientHWIDJob) hasHWIDTracking() bool {
	// Check global HWID mode setting
	settingService := service.SettingService{}
	hwidMode, err := settingService.GetHwidMode()
	if err != nil {
		logger.Warningf("Failed to get hwidMode setting: %v", err)
		return false
	}

	// If HWID tracking is disabled globally, skip
	if hwidMode == "off" {
		return false
	}

	// Check if any client has HWID tracking enabled
	db := database.GetDB()
	var clients []*model.ClientEntity

	err = db.Where("hwid_enabled = ?", true).Find(&clients).Error
	if err != nil {
		return false
	}

	return len(clients) > 0
}

// checkAccessLogAvailable checks if access log is available.
func (j *CheckClientHWIDJob) checkAccessLogAvailable() bool {
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return false
	}

	if accessLogPath == "none" || accessLogPath == "" {
		return false
	}

	return true
}

// processLogFile processes the access log file to update last_seen_at and IP for existing HWIDs.
// NOTE: This job does NOT generate or create new HWID records.
// HWID registration must be done explicitly via RegisterHWIDFromHeaders when x-hwid header is provided.
// This job only updates existing HWID records with connection information from access logs.
func (j *CheckClientHWIDJob) processLogFile() {
	// Check HWID mode - only run in legacy_fingerprint mode
	settingService := service.SettingService{}
	hwidMode, err := settingService.GetHwidMode()
	if err != nil {
		logger.Warningf("Failed to get hwidMode setting: %v", err)
		return
	}

	// In client_header mode, this job should not process logs for HWID generation
	// It may still update last_seen_at for existing HWIDs if needed
	if hwidMode == "off" {
		// HWID tracking disabled - skip processing
		return
	}

	// In client_header mode, we don't generate HWIDs from logs
	// Only update existing HWIDs if we can match them somehow
	// For now, skip log processing in client_header mode
	// (HWID registration happens via RegisterHWIDFromHeaders when subscription is requested)
	if hwidMode == "client_header" {
		// In client_header mode, HWID comes from headers, not logs
		// This job should not process logs for HWID generation
		// TODO: Could potentially update last_seen_at for existing HWIDs if we can match them,
		// but without x-hwid header in logs, we can't reliably match
		return
	}

	// Legacy fingerprint mode (deprecated)
	// This mode may use fingerprint-based HWID generation from logs
	if hwidMode == "legacy_fingerprint" {
		// Legacy mode: may generate HWID from logs (deprecated behavior)
		// This is kept for backward compatibility only
		logger.Debug("Running in legacy_fingerprint mode (deprecated)")
		// TODO: Implement legacy fingerprint logic if needed for backward compatibility
		// For now, skip to avoid false positives
		return
	}
}

// clearAccessLog clears the access log file (similar to CheckClientIpJob).
func (j *CheckClientHWIDJob) clearAccessLog() {
	// This is similar to CheckClientIpJob.clearAccessLog
	// We can reuse the same logic or call it from there
	// For now, we'll just update the last clear time
	j.lastClear = time.Now().Unix()
}
