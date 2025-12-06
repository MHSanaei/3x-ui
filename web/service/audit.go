package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// AuditLogService handles audit logging
type AuditLogService struct{}

// AuditAction represents an audit log entry
type AuditAction struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`   // CREATE, UPDATE, DELETE, LOGIN, LOGOUT, etc.
	Resource   string    `json:"resource"` // inbound, client, setting, etc.
	ResourceID int       `json:"resource_id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Details    string    `json:"details"` // JSON string with additional details
	Timestamp  time.Time `json:"timestamp"`
}

// LogAction logs an audit action with error handling
func (s *AuditLogService) LogAction(userID int, username, action, resource string, resourceID int, ip, userAgent string, details map[string]interface{}) error {
	db := database.GetDB()

	detailsJSON := ""
	if details != nil {
		jsonData, err := json.Marshal(details)
		if err != nil {
			logger.Warning("Failed to marshal audit log details:", err)
		} else {
			detailsJSON = string(jsonData)
		}
	}

	auditLog := model.AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IP:         ip,
		UserAgent:  userAgent,
		Details:    detailsJSON,
		Timestamp:  time.Now(),
	}

	if err := db.Create(&auditLog).Error; err != nil {
		logger.Warningf("Failed to create audit log: user=%d, action=%s, resource=%s, error=%v", userID, action, resource, err)
		return err
	}

	return nil
}

// GetAuditLogs retrieves audit logs with filters and pagination
func (s *AuditLogService) GetAuditLogs(userID, limit, offset int, action, resource string, startTime, endTime *time.Time) ([]AuditAction, int64, error) {
	db := database.GetDB()

	query := db.Model(&model.AuditLog{})

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if startTime != nil {
		query = query.Where("timestamp >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("timestamp <= ?", endTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	if err := query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	actions := make([]AuditAction, len(logs))
	for i, log := range logs {
		actions[i] = AuditAction{
			ID:         log.ID,
			UserID:     log.UserID,
			Username:   log.Username,
			Action:     log.Action,
			Resource:   log.Resource,
			ResourceID: log.ResourceID,
			IP:         log.IP,
			UserAgent:  log.UserAgent,
			Details:    log.Details,
			Timestamp:  log.Timestamp,
		}
	}

	return actions, total, nil
}

// CleanOldLogs removes audit logs older than specified days
func (s *AuditLogService) CleanOldLogs(days int) error {
	if days <= 0 {
		return fmt.Errorf("days must be greater than 0")
	}

	db := database.GetDB()
	cutoff := time.Now().AddDate(0, 0, -days)

	result := db.Where("timestamp < ?", cutoff).Delete(&model.AuditLog{})
	if result.Error != nil {
		return result.Error
	}

	logger.Infof("Cleaned %d old audit logs (older than %d days)", result.RowsAffected, days)
	return nil
}

// GetAuditStats returns statistics about audit logs
func (s *AuditLogService) GetAuditStats(startTime, endTime *time.Time) (map[string]interface{}, error) {
	db := database.GetDB()

	query := db.Model(&model.AuditLog{})
	if startTime != nil {
		query = query.Where("timestamp >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("timestamp <= ?", endTime)
	}

	var totalLogs int64
	if err := query.Count(&totalLogs).Error; err != nil {
		return nil, err
	}

	// Count by action
	var actionCounts []struct {
		Action string
		Count  int64
	}
	query.Select("action, COUNT(*) as count").
		Group("action").
		Scan(&actionCounts)

	// Count by resource
	var resourceCounts []struct {
		Resource string
		Count    int64
	}
	query.Select("resource, COUNT(*) as count").
		Group("resource").
		Scan(&resourceCounts)

	stats := map[string]interface{}{
		"total_logs":      totalLogs,
		"action_counts":   actionCounts,
		"resource_counts": resourceCounts,
	}

	return stats, nil
}
