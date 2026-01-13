// Package service provides HWID (Hardware ID) management for clients.
// HWID is provided explicitly by client applications via HTTP headers (x-hwid).
// Server MUST NOT generate or derive HWID from IP, User-Agent, or access logs.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"gorm.io/gorm"
)

// ClientHWIDService provides business logic for managing client HWIDs.
type ClientHWIDService struct{}

// GetHWIDsForClient retrieves all HWIDs associated with a client.
func (s *ClientHWIDService) GetHWIDsForClient(clientId int) ([]*model.ClientHWID, error) {
	db := database.GetDB()
	var hwids []*model.ClientHWID
	err := db.Where("client_id = ?", clientId).Order("last_seen_at DESC").Find(&hwids).Error
	return hwids, err
}

// AddHWIDForClient adds a new HWID for a client with device metadata.
// HWID must be provided explicitly (not generated).
// If the client has HWID restrictions enabled, checks if the limit is exceeded.
func (s *ClientHWIDService) AddHWIDForClient(clientId int, hwid string, deviceOS string, deviceModel string, osVersion string, ipAddress string, userAgent string) (*model.ClientHWID, error) {
	// Normalize HWID (trim, but preserve case - HWID is opaque identifier from client)
	hwid = strings.TrimSpace(hwid)
	if hwid == "" {
		return nil, fmt.Errorf("HWID cannot be empty")
	}

	// Get client to check restrictions
	clientService := ClientService{}
	client, err := clientService.GetClient(clientId)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("client not found")
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Check if HWID already exists for this client
	var existingHWID model.ClientHWID
	err = tx.Where("client_id = ? AND hwid = ?", clientId, hwid).First(&existingHWID).Error
	if err == nil {
		// HWID exists - update last seen and IP
		now := time.Now().Unix()
		updates := map[string]interface{}{
			"last_seen_at": now,
			"ip_address":   ipAddress,
		}
		if userAgent != "" {
			updates["user_agent"] = userAgent
		}
		// Update device metadata if provided
		if deviceOS != "" {
			updates["device_os"] = deviceOS
		}
		if deviceModel != "" {
			updates["device_model"] = deviceModel
		}
		if osVersion != "" {
			updates["os_version"] = osVersion
		}
		existingHWID.IsActive = true
		err = tx.Model(&existingHWID).Updates(updates).Error
		if err != nil {
			return nil, err
		}
		// Reload to get updated fields
		tx.First(&existingHWID, existingHWID.Id)
		return &existingHWID, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing HWID: %w", err)
	}

	// HWID doesn't exist - check if we can add it
	var activeHWIDCount int64
	if client.HWIDEnabled {
		// Count active HWIDs for this client
		err = tx.Model(&model.ClientHWID{}).Where("client_id = ? AND is_active = ?", clientId, true).Count(&activeHWIDCount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to count active HWIDs: %w", err)
		}

		// Check limit (0 means unlimited)
		if client.MaxHWID > 0 && int(activeHWIDCount) >= client.MaxHWID {
			return nil, fmt.Errorf("HWID limit exceeded: max %d devices allowed, current: %d", client.MaxHWID, activeHWIDCount)
		}
	} else {
		// Count all HWIDs for device naming even if restriction is disabled
		err = tx.Model(&model.ClientHWID{}).Where("client_id = ?", clientId).Count(&activeHWIDCount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to count HWIDs: %w", err)
		}
	}

	// Create new HWID record
	now := time.Now().Unix()
	newHWID := &model.ClientHWID{
		ClientId:    clientId,
		HWID:        hwid,
		DeviceOS:    deviceOS,
		DeviceModel: deviceModel,
		OSVersion:   osVersion,
		IPAddress:   ipAddress,
		FirstSeenIP: ipAddress,
		UserAgent:   userAgent,
		IsActive:    true,
		FirstSeenAt: now,
		LastSeenAt:  now,
		DeviceName:  fmt.Sprintf("Device %d", activeHWIDCount+1), // Legacy field, deprecated
	}

	err = tx.Create(newHWID).Error
	if err != nil {
		logger.Errorf("Failed to create HWID record in database: %v", err)
		return nil, fmt.Errorf("failed to create HWID: %w", err)
	}

	logger.Debugf("Successfully created HWID record: clientId=%d, hwid=%s, hwidId=%d", clientId, hwid, newHWID.Id)
	return newHWID, nil
}

// RemoveHWID removes a HWID from a client.
func (s *ClientHWIDService) RemoveHWID(hwidId int) error {
	db := database.GetDB()
	return db.Delete(&model.ClientHWID{}, hwidId).Error
}

// DeactivateHWID deactivates a HWID (marks as inactive instead of deleting).
func (s *ClientHWIDService) DeactivateHWID(hwidId int) error {
	db := database.GetDB()
	return db.Model(&model.ClientHWID{}).Where("id = ?", hwidId).Update("is_active", false).Error
}

// CheckHWIDAllowed checks if a HWID is allowed for a client.
// Returns true if HWID restriction is disabled, or if HWID is in the allowed list.
// NOTE: This method does NOT auto-register HWID. Use RegisterHWIDFromHeaders for registration.
// Behavior depends on hwidMode setting:
//   - "off": Always returns true (HWID tracking disabled)
//   - "client_header": Requires explicit HWID registration, checks against registered devices
//   - "legacy_fingerprint": Legacy mode (deprecated)
func (s *ClientHWIDService) CheckHWIDAllowed(clientId int, hwid string) (bool, error) {
	// Check HWID mode setting
	settingService := SettingService{}
	hwidMode, err := settingService.GetHwidMode()
	if err != nil {
		logger.Warningf("Failed to get hwidMode setting, defaulting to client_header: %v", err)
		hwidMode = "client_header"
	}

	// If HWID tracking is disabled globally, allow all
	if hwidMode == "off" {
		return true, nil
	}

	// Normalize HWID (trim, but preserve case - HWID is opaque identifier from client)
	hwid = strings.TrimSpace(hwid)
	if hwid == "" {
		// In client_header mode, empty HWID means "unknown device" - don't count, but allow
		if hwidMode == "client_header" {
			return true, nil // Allow but don't count as registered device
		}
		return false, fmt.Errorf("HWID cannot be empty")
	}

	// Get client
	clientService := ClientService{}
	client, err := clientService.GetClient(clientId)
	if err != nil {
		return false, fmt.Errorf("failed to get client: %w", err)
	}
	if client == nil {
		return false, fmt.Errorf("client not found")
	}

	// If HWID restriction is disabled for this client, allow all
	if !client.HWIDEnabled {
		return true, nil
	}

	// In client_header mode, HWID must be explicitly registered
	if hwidMode == "client_header" {
		// Check if HWID exists and is active
		db := database.GetDB()
		var hwidRecord model.ClientHWID
		err = db.Where("client_id = ? AND hwid = ? AND is_active = ?", clientId, hwid, true).First(&hwidRecord).Error
		if err == nil {
			// HWID exists and is active - update last seen
			db.Model(&hwidRecord).Update("last_seen_at", time.Now().Unix())
			return true, nil
		} else if err == gorm.ErrRecordNotFound {
			// HWID not found - check if we're under limit (allows registration)
			var activeHWIDCount int64
			err = db.Model(&model.ClientHWID{}).Where("client_id = ? AND is_active = ?", clientId, true).Count(&activeHWIDCount).Error
			if err != nil {
				return false, fmt.Errorf("failed to count active HWIDs: %w", err)
			}

			// If under limit, allow (registration can happen via RegisterHWIDFromHeaders)
			if client.MaxHWID == 0 || int(activeHWIDCount) < client.MaxHWID {
				return true, nil
			}

			// Limit reached, HWID not registered
			return false, fmt.Errorf("HWID limit exceeded: max %d devices allowed, current: %d", client.MaxHWID, activeHWIDCount)
		}

		return false, fmt.Errorf("failed to check HWID: %w", err)
	}

	// Legacy fingerprint mode (deprecated) - kept for backward compatibility
	// This mode may use fingerprint-based HWID generation (not recommended)
	if hwidMode == "legacy_fingerprint" {
		// Check if HWID exists and is active
		db := database.GetDB()
		var hwidRecord model.ClientHWID
		err = db.Where("client_id = ? AND hwid = ? AND is_active = ?", clientId, hwid, true).First(&hwidRecord).Error
		if err == nil {
			// HWID exists and is active - update last seen
			db.Model(&hwidRecord).Update("last_seen_at", time.Now().Unix())
			return true, nil
		} else if err == gorm.ErrRecordNotFound {
			// HWID not found - check limit
			var activeHWIDCount int64
			err = db.Model(&model.ClientHWID{}).Where("client_id = ? AND is_active = ?", clientId, true).Count(&activeHWIDCount).Error
			if err != nil {
				return false, fmt.Errorf("failed to count active HWIDs: %w", err)
			}

			// If under limit, allow (legacy mode may auto-register via job)
			if client.MaxHWID == 0 || int(activeHWIDCount) < client.MaxHWID {
				return true, nil
			}

			// Limit reached, HWID not in list
			return false, nil
		}

		return false, fmt.Errorf("failed to check HWID: %w", err)
	}

	// Unknown mode - default to allowing (fail open)
	logger.Warningf("Unknown hwidMode: %s, allowing request", hwidMode)
	return true, nil
}

// RegisterHWIDFromHeaders registers a HWID from HTTP headers provided by client application.
// This is the primary method for HWID registration in client_header mode.
// Headers:
//   - x-hwid (required): Hardware ID provided by client
//   - x-device-os (optional): Device operating system
//   - x-device-model (optional): Device model
//   - x-ver-os (optional): OS version
//   - user-agent (optional): User agent string
func (s *ClientHWIDService) RegisterHWIDFromHeaders(clientId int, hwid string, deviceOS string, deviceModel string, osVersion string, ipAddress string, userAgent string) (*model.ClientHWID, error) {
	// HWID must be provided explicitly
	hwid = strings.TrimSpace(hwid)
	if hwid == "" {
		return nil, fmt.Errorf("HWID is required (x-hwid header missing)")
	}

	// Get client to check restrictions
	clientService := ClientService{}
	client, err := clientService.GetClient(clientId)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("client not found")
	}

	// Check HWID mode setting
	settingService := SettingService{}
	hwidMode, err := settingService.GetHwidMode()
	if err != nil {
		logger.Warningf("Failed to get hwidMode setting, defaulting to client_header: %v", err)
		hwidMode = "client_header"
	}

	// In client_header mode, HWID must be provided explicitly (which it is, since we're here)
	// In legacy_fingerprint mode, this method should not be called (use legacy methods)
	if hwidMode == "off" {
		// HWID tracking disabled - allow but don't register
		return nil, nil
	}

	// Register or update HWID
	logger.Debugf("RegisterHWIDFromHeaders: calling AddHWIDForClient for clientId=%d, hwid=%s", clientId, hwid)
	return s.AddHWIDForClient(clientId, hwid, deviceOS, deviceModel, osVersion, ipAddress, userAgent)
}

// UpdateHWIDLastSeen updates the last seen timestamp and IP address for a HWID.
func (s *ClientHWIDService) UpdateHWIDLastSeen(clientId int, hwid string, ipAddress string) error {
	hwid = strings.TrimSpace(hwid) // Preserve case - HWID is opaque identifier
	if hwid == "" {
		return fmt.Errorf("HWID cannot be empty")
	}

	db := database.GetDB()
	return db.Model(&model.ClientHWID{}).
		Where("client_id = ? AND hwid = ?", clientId, hwid).
		Updates(map[string]interface{}{
			"last_seen_at": time.Now().Unix(),
			"ip_address":   ipAddress,
		}).Error
}

// GenerateFingerprintHWID generates a fingerprint-based HWID from connection parameters.
// DEPRECATED: This method is only for legacy_fingerprint mode (backward compatibility).
// In client_header mode, HWID must be provided explicitly by client via x-hwid header.
// Do NOT use this method for new implementations.
func (s *ClientHWIDService) GenerateFingerprintHWID(email string, ipAddress string, userAgent string) string {
	// DEPRECATED: This method should only be used in legacy_fingerprint mode
	// Combine parameters to create a fingerprint
	fingerprint := fmt.Sprintf("%s|%s|%s", email, ipAddress, userAgent)
	
	// Hash the fingerprint to create a stable HWID
	// NOTE: This approach is deprecated and may cause false positives
	// when IP addresses change or clients reconnect from different networks
	hash := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(hash[:])[:32] // Use first 32 chars of hash
}
