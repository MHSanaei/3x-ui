package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	redisutil "github.com/mhsanaei/3x-ui/v2/util/redis"
)

// AnalyticsService handles traffic analytics
type AnalyticsService struct {
	inboundService InboundService
}

// TrafficStats represents traffic statistics
type TrafficStats struct {
	Time        time.Time `json:"time"`
	Up          int64     `json:"up"`
	Down        int64     `json:"down"`
	Total       int64     `json:"total"`
	ClientCount int       `json:"client_count"`
}

// HourlyStats represents hourly traffic statistics
type HourlyStats struct {
	Hour  int   `json:"hour"`
	Up    int64 `json:"up"`
	Down  int64 `json:"down"`
	Total int64 `json:"total"`
}

// DailyStats represents daily traffic statistics
type DailyStats struct {
	Date  string `json:"date"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
	Total int64  `json:"total"`
}

// GetHourlyStats gets hourly traffic statistics for the last 24 hours
func (s *AnalyticsService) GetHourlyStats(inboundID int) ([]HourlyStats, error) {
	now := time.Now()
	stats := make([]HourlyStats, 24)

	for i := 0; i < 24; i++ {
		hour := now.Add(-time.Duration(23-i) * time.Hour)

		var up, down int64
		// Query traffic from database or Redis
		// This is simplified - in production, aggregate from Xray logs or API
		key := fmt.Sprintf("traffic:hourly:%d:%d", inboundID, hour.Hour())
		data, _ := redisutil.HGetAll(key)
		if upStr, ok := data["up"]; ok && upStr != "" {
			if parsed, err := strconv.ParseInt(upStr, 10, 64); err == nil {
				up = parsed
			}
		}
		if downStr, ok := data["down"]; ok && downStr != "" {
			if parsed, err := strconv.ParseInt(downStr, 10, 64); err == nil {
				down = parsed
			}
		}

		stats[i] = HourlyStats{
			Hour:  hour.Hour(),
			Up:    up,
			Down:  down,
			Total: up + down,
		}
	}

	return stats, nil
}

// GetDailyStats gets daily traffic statistics for the last 30 days
func (s *AnalyticsService) GetDailyStats(inboundID int) ([]DailyStats, error) {
	stats := make([]DailyStats, 30)
	now := time.Now()

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -29+i)
		dateStr := date.Format("2006-01-02")

		// Query from database or Redis
		key := fmt.Sprintf("traffic:daily:%d:%s", inboundID, dateStr)
		data, _ := redisutil.HGetAll(key)

		var up, down int64
		if upStr, ok := data["up"]; ok && upStr != "" {
			if parsed, err := strconv.ParseInt(upStr, 10, 64); err == nil {
				up = parsed
			}
		}
		if downStr, ok := data["down"]; ok && downStr != "" {
			if parsed, err := strconv.ParseInt(downStr, 10, 64); err == nil {
				down = parsed
			}
		}

		stats[i] = DailyStats{
			Date:  dateStr,
			Up:    up,
			Down:  down,
			Total: up + down,
		}
	}

	return stats, nil
}

// GetTopClients gets top clients by traffic
func (s *AnalyticsService) GetTopClients(inboundID int, limit int) ([]model.Client, error) {
	db := database.GetDB()
	var inbound model.Inbound
	if err := db.First(&inbound, inboundID).Error; err != nil {
		return nil, err
	}

	clients, err := s.inboundService.GetClients(&inbound)
	if err != nil {
		return nil, err
	}

	// Sort by traffic (simplified)
	// In production, get from Xray API or aggregate from logs
	return clients[:min(limit, len(clients))], nil
}

// RecordTraffic records traffic for analytics
func (s *AnalyticsService) RecordTraffic(inboundID int, email string, up, down int64) error {
	if inboundID <= 0 {
		return fmt.Errorf("invalid inbound ID")
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if up < 0 || down < 0 {
		return fmt.Errorf("traffic values cannot be negative")
	}

	now := time.Now()

	// Record hourly (aggregate)
	hourKey := fmt.Sprintf("traffic:hourly:%d:%d", inboundID, now.Hour())
	currentUpStr, _ := redisutil.HGet(hourKey, "up")
	currentDownStr, _ := redisutil.HGet(hourKey, "down")

	var currentUp, currentDown int64
	if currentUpStr != "" {
		if parsed, err := strconv.ParseInt(currentUpStr, 10, 64); err == nil {
			currentUp = parsed
		}
	}
	if currentDownStr != "" {
		if parsed, err := strconv.ParseInt(currentDownStr, 10, 64); err == nil {
			currentDown = parsed
		}
	}

	redisutil.HSet(hourKey, "up", currentUp+up)
	redisutil.HSet(hourKey, "down", currentDown+down)
	redisutil.Expire(hourKey, 25*time.Hour)

	// Record daily (aggregate)
	dateKey := fmt.Sprintf("traffic:daily:%d:%s", inboundID, now.Format("2006-01-02"))
	dailyUpStr, _ := redisutil.HGet(dateKey, "up")
	dailyDownStr, _ := redisutil.HGet(dateKey, "down")

	var dailyUp, dailyDown int64
	if dailyUpStr != "" {
		if parsed, err := strconv.ParseInt(dailyUpStr, 10, 64); err == nil {
			dailyUp = parsed
		}
	}
	if dailyDownStr != "" {
		if parsed, err := strconv.ParseInt(dailyDownStr, 10, 64); err == nil {
			dailyDown = parsed
		}
	}

	redisutil.HSet(dateKey, "up", dailyUp+up)
	redisutil.HSet(dateKey, "down", dailyDown+down)
	redisutil.Expire(dateKey, 32*24*time.Hour)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
