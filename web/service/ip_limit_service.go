package service

import (
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"gorm.io/gorm"
)

type IPLimitService struct{}

// IPRecord stores client IP information
type IPRecord struct {
	IP        string `json:"ip"`
	LastSeen  int64  `json:"lastSeen"`
	FirstSeen int64  `json:"firstSeen"`
}

const (
	// IP stale cutoff: 30 days
	IPStaleCutoffDays = 30
)

// CheckIPLimit validates if a client has exceeded IP limit
// Returns (allowed, error)
func (s *IPLimitService) CheckIPLimit(clientEmail string, limit int, newIP string) (bool, error) {
	if limit <= 0 {
		return true, nil // No limit
	}

	db := database.GetDB()
	var record model.InboundClientIPs

	err := db.Where("client_email = ?", clientEmail).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil // No IPs recorded yet
		}
		return false, err
	}

	// Parse IP list
	ipList := strings.Split(record.IPs, ",")
	var cleanIPs []string
	for _, ip := range ipList {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			cleanIPs = append(cleanIPs, trimmed)
		}
	}

	// Check if new IP exists
	ipExists := false
	for _, ip := range cleanIPs {
		if ip == newIP {
			ipExists = true
			break
		}
	}

	// Add new IP if not seen before
	if !ipExists {
		cleanIPs = append(cleanIPs, newIP)
	}

	// Check if exceeded limit
	if len(cleanIPs) > limit {
		return false, nil // Limit exceeded
	}

	return true, nil
}

// RecordIPAccess records or updates IP information
func (s *IPLimitService) RecordIPAccess(clientEmail, clientIP string) error {
	db := database.GetDB()
	now := time.Now().Unix()

	var record model.InboundClientIPs
	err := db.Where("client_email = ?", clientEmail).First(&record).Error

	// Parse existing IPs
	var ipList []string
	if err == nil {
		for _, ip := range strings.Split(record.IPs, ",") {
			if trimmed := strings.TrimSpace(ip); trimmed != "" {
				ipList = append(ipList, trimmed)
			}
		}
	}

	// Add or update IP
	ipExists := false
	for _, ip := range ipList {
		if ip == clientIP {
			ipExists = true
			break
		}
	}

	if !ipExists {
		ipList = append(ipList, clientIP)
	}

	// Join IPs back to string
	updatedIPs := strings.Join(ipList, ",")

	if err == nil && record.Id > 0 {
		// Update existing record
		record.IPs = updatedIPs
		record.UpdatedAt = now
		return db.Save(&record).Error
	}

	// Create new record
	return db.Create(&model.InboundClientIPs{
		ClientEmail: clientEmail,
		IPs:         updatedIPs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error
}

// GetClientIPs returns all IPs for a client
func (s *IPLimitService) GetClientIPs(clientEmail string) ([]string, error) {
	db := database.GetDB()
	var record model.InboundClientIPs

	err := db.Where("client_email = ?", clientEmail).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []string{}, nil
		}
		return nil, err
	}

	var ips []string
	for _, ip := range strings.Split(record.IPs, ",") {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			ips = append(ips, trimmed)
		}
	}

	return ips, nil
}

// RemoveIP removes a specific IP from client's IP list
func (s *IPLimitService) RemoveIP(clientEmail, ipToRemove string) error {
	db := database.GetDB()
	var record model.InboundClientIPs

	err := db.Where("client_email = ?", clientEmail).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	// Parse and filter IPs
	var filtered []string
	for _, ip := range strings.Split(record.IPs, ",") {
		if trimmed := strings.TrimSpace(ip); trimmed != "" && trimmed != ipToRemove {
			filtered = append(filtered, trimmed)
		}
	}

	if len(filtered) == 0 {
		return db.Delete(&record).Error
	}

	record.IPs = strings.Join(filtered, ",")
	record.UpdatedAt = time.Now().Unix()
	return db.Save(&record).Error
}

// ClearAllIPs removes all IPs for a client
func (s *IPLimitService) ClearAllIPs(clientEmail string) error {
	db := database.GetDB()
	return db.Where("client_email = ?", clientEmail).Delete(&model.InboundClientIPs{}).Error
}
