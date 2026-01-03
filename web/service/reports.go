package service

import (
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// ReportsService handles client usage reports
type ReportsService struct {
	inboundService   InboundService
	analyticsService AnalyticsService
}

// ClientReport represents a client usage report
type ClientReport struct {
	Email           string    `json:"email"`
	Period          string    `json:"period"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	TotalUp         int64     `json:"total_up"`
	TotalDown       int64     `json:"total_down"`
	TotalTraffic    int64     `json:"total_traffic"`
	QuotaUsed       float64   `json:"quota_used_percent"`
	ActiveDays      int       `json:"active_days"`
	TopCountries    []string  `json:"top_countries"`
	Recommendations []string  `json:"recommendations"`
}

// GenerateClientReport generates a usage report for a client
func (s *ReportsService) GenerateClientReport(email string, period string) (*ClientReport, error) {
	// Get period dates
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "weekly":
		startDate = now.AddDate(0, 0, -7)
		endDate = now
	case "monthly":
		startDate = now.AddDate(0, -1, 0)
		endDate = now
	default:
		startDate = now.AddDate(0, 0, -7)
		endDate = now
	}

	// Get client data
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	var client *model.Client
	for i := range inbounds {
		inbound := inbounds[i]
		clients, _ := s.inboundService.GetClients(inbound)
		for j := range clients {
			if clients[j].Email == email {
				client = &clients[j]
				break
			}
		}
		if client != nil {
			break
		}
	}

	if client == nil {
		return nil, fmt.Errorf("client not found: %s", email)
	}

	// Calculate traffic (simplified - in production, get from analytics)
	report := &ClientReport{
		Email:     email,
		Period:    period,
		StartDate: startDate,
		EndDate:   endDate,
		TotalUp:   0, // Get from analytics
		TotalDown: 0, // Get from analytics
	}

	report.TotalTraffic = report.TotalUp + report.TotalDown

	// Calculate quota usage
	if client.TotalGB > 0 {
		report.QuotaUsed = float64(report.TotalTraffic) / float64(client.TotalGB*1024*1024*1024) * 100
	}

	// Generate recommendations
	report.Recommendations = s.generateRecommendations(report, client)

	return report, nil
}

// generateRecommendations generates usage recommendations
func (s *ReportsService) generateRecommendations(report *ClientReport, client *model.Client) []string {
	recommendations := make([]string, 0)

	if report.QuotaUsed > 80 {
		recommendations = append(recommendations, "You are using more than 80% of your quota. Consider upgrading your plan.")
	}

	if report.ActiveDays < 3 {
		recommendations = append(recommendations, "Low activity detected. Your VPN connection may need attention.")
	}

	if client.ExpiryTime > 0 && time.Now().UnixMilli() > client.ExpiryTime-7*24*3600*1000 {
		recommendations = append(recommendations, "Your subscription expires soon. Please renew to avoid service interruption.")
	}

	return recommendations
}

// SendWeeklyReports sends weekly reports to all clients
func (s *ReportsService) SendWeeklyReports() error {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return err
	}

	for i := range inbounds {
		inbound := inbounds[i]
		clients, _ := s.inboundService.GetClients(inbound)
		for _, client := range clients {
			_, err := s.GenerateClientReport(client.Email, "weekly")
			if err != nil {
				logger.Warningf("Failed to generate report for %s: %v", client.Email, err)
				continue
			}

			// Send report (implement email/telegram sending)
			logger.Infof("Generated weekly report for %s", client.Email)
		}
	}

	return nil
}

// SendMonthlyReports sends monthly reports to all clients
func (s *ReportsService) SendMonthlyReports() error {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return err
	}

	for i := range inbounds {
		inbound := inbounds[i]
		clients, _ := s.inboundService.GetClients(inbound)
		for _, client := range clients {
			_, err := s.GenerateClientReport(client.Email, "monthly")
			if err != nil {
				logger.Warningf("Failed to generate report for %s: %v", client.Email, err)
				continue
			}

			// Send report
			logger.Infof("Generated monthly report for %s", client.Email)
		}
	}

	return nil
}
