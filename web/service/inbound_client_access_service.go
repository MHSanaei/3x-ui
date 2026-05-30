package service

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/logger"
)

// CheckClientAccess validates if a client can access based on IP limit
// Combines IP validation and access logging
func (s *InboundService) CheckClientAccess(clientEmail string, clientIP string, limitIP int) (bool, string, error) {
	if limitIP <= 0 {
		return true, "Access allowed (no limit)", nil
	}

	// Check IP limit
	ipSvc := &IPLimitService{}
	ipAllowed, err := ipSvc.CheckIPLimit(clientEmail, limitIP, clientIP)
	if err != nil {
		logger.Error("[IPLimit] Check error for", clientEmail, err)
		return false, "IP limit check error", err
	}

	if !ipAllowed {
		msg := fmt.Sprintf("IP limit exceeded (max %d IPs allowed)", limitIP)
		logger.Warn("[IPLimit] Limit exceeded for", clientEmail, "IP:", clientIP)
		return false, msg, nil
	}

	// Record IP access
	err = ipSvc.RecordIPAccess(clientEmail, clientIP)
	if err != nil {
		logger.Error("[IPLimit] Failed to record IP access:", err)
		return false, "Failed to record IP access", err
	}

	logger.Info("[IPLimit] Access allowed for", clientEmail, "IP:", clientIP)
	return true, "Access allowed", nil
}

// ValidateClientIP performs comprehensive IP validation
func (s *InboundService) ValidateClientIP(clientEmail string, clientIP string, limitIP int) (bool, error) {
	ipSvc := &IPLimitService{}
	return ipSvc.CheckIPLimit(clientEmail, limitIP, clientIP)
}

// GetClientIPList retrieves all registered IPs for a client
func (s *InboundService) GetClientIPList(clientEmail string) ([]string, error) {
	ipSvc := &IPLimitService{}
	return ipSvc.GetClientIPs(clientEmail)
}

// RemoveClientIP removes a specific IP from client's registry
func (s *InboundService) RemoveClientIP(clientEmail string, ipToRemove string) error {
	ipSvc := &IPLimitService{}
	return ipSvc.RemoveIP(clientEmail, ipToRemove)
}
