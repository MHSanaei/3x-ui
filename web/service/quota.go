package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	redisutil "github.com/mhsanaei/3x-ui/v2/util/redis"
)

// QuotaService handles bandwidth quota management
type QuotaService struct {
	inboundService InboundService
}

// QuotaInfo represents quota information for a client
type QuotaInfo struct {
	Email        string  `json:"email"`
	UsedBytes    int64   `json:"used_bytes"`
	TotalBytes   int64   `json:"total_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	ResetTime    int64   `json:"reset_time"`
	Status       string  `json:"status"` // normal, warning, exceeded
}

// CheckQuota checks if client has exceeded quota
func (s *QuotaService) CheckQuota(email string, inbound *model.Inbound) (bool, *QuotaInfo, error) {
	clients, err := s.inboundService.GetClients(inbound)
	if err != nil {
		return false, nil, err
	}

	var client *model.Client
	for i := range clients {
		if clients[i].Email == email {
			client = &clients[i]
			break
		}
	}

	if client == nil {
		return false, nil, nil
	}

	// Get traffic from Xray API or database
	trafficKey := "traffic:" + email
	usedBytesStr, err := redisutil.Get(trafficKey)
	var usedBytes int64
	if err == nil && usedBytesStr != "" {
		if parsed, parseErr := strconv.ParseInt(usedBytesStr, 10, 64); parseErr == nil {
			usedBytes = parsed
		}
	}

	totalBytes := client.TotalGB * 1024 * 1024 * 1024
	var usagePercent float64
	if totalBytes > 0 {
		usagePercent = float64(usedBytes) / float64(totalBytes) * 100
	} else {
		// Unlimited quota
		usagePercent = 0
	}

	quotaInfo := &QuotaInfo{
		Email:        email,
		UsedBytes:    usedBytes,
		TotalBytes:   totalBytes,
		UsagePercent: usagePercent,
		ResetTime:    client.ExpiryTime,
	}

	// Determine status
	if totalBytes > 0 {
		if usagePercent >= 100 {
			quotaInfo.Status = "exceeded"
			return false, quotaInfo, nil
		} else if usagePercent >= 80 {
			quotaInfo.Status = "warning"
			return true, quotaInfo, nil
		}
	}

	quotaInfo.Status = "normal"
	return true, quotaInfo, nil
}

// ThrottleClient throttles client speed when quota exceeded
func (s *QuotaService) ThrottleClient(email string, inbound *model.Inbound, throttle bool) error {
	// This would integrate with Xray API to throttle speed
	// For now, we'll just log it
	if throttle {
		logger.Infof("Throttling client %s due to quota", email)
	} else {
		logger.Infof("Removing throttle for client %s", email)
	}
	return nil
}

// GetQuotaInfo gets quota information for all clients
func (s *QuotaService) GetQuotaInfo(inbound *model.Inbound) ([]QuotaInfo, error) {
	clients, err := s.inboundService.GetClients(inbound)
	if err != nil {
		return nil, err
	}

	quotaInfos := make([]QuotaInfo, 0, len(clients))
	for _, client := range clients {
		_, quotaInfo, err := s.CheckQuota(client.Email, inbound)
		if err != nil {
			continue
		}
		if quotaInfo != nil {
			quotaInfos = append(quotaInfos, *quotaInfo)
		}
	}

	return quotaInfos, nil
}

// ResetQuota resets quota for a client
func (s *QuotaService) ResetQuota(email string) error {
	trafficKey := "traffic:" + email
	return redisutil.Del(trafficKey)
}

// UpdateQuotaUsage updates quota usage from Xray traffic
func (s *QuotaService) UpdateQuotaUsage(email string, up, down int64) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if up < 0 || down < 0 {
		return fmt.Errorf("traffic values cannot be negative")
	}

	trafficKey := "traffic:" + email
	currentStr, err := redisutil.Get(trafficKey)
	var current int64
	if err == nil && currentStr != "" {
		if parsed, parseErr := strconv.ParseInt(currentStr, 10, 64); parseErr == nil {
			current = parsed
		}
	}

	newTotal := current + up + down
	return redisutil.Set(trafficKey, newTotal, 30*24*time.Hour)
}
