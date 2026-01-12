// Package cache provides caching utilities with JSON serialization support.
package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

const (
	// Default TTL values
	TTLInbounds  = 30 * time.Second
	TTLClients   = 30 * time.Second
	TTLSettings  = 5 * time.Minute
	TTLSetting   = 10 * time.Minute // Increased from 5 to 10 minutes for better cache hit rate
)

// Cache keys
const (
	KeyInboundsPrefix = "inbounds:user:"
	KeyClientsPrefix  = "clients:user:"
	KeySettingsAll    = "settings:all"
	KeySettingPrefix  = "setting:"
)

// GetJSON retrieves a value from cache and unmarshals it as JSON.
func GetJSON(key string, dest interface{}) error {
	val, err := Get(key)
	if err != nil {
		// Check if it's a "key not found" error (redis.Nil)
		// This is expected and not a real error
		if err.Error() == "redis: nil" {
			return fmt.Errorf("key not found: %s", key)
		}
		return err
	}
	if val == "" {
		return fmt.Errorf("empty value for key: %s", key)
	}
	return json.Unmarshal([]byte(val), dest)
}

// SetJSON marshals a value as JSON and stores it in cache.
func SetJSON(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return Set(key, string(data), expiration)
}

// GetOrSet retrieves a value from cache, or computes it using fn if not found.
func GetOrSet(key string, dest interface{}, expiration time.Duration, fn func() (interface{}, error)) error {
	// Try to get from cache
	err := GetJSON(key, dest)
	if err == nil {
		logger.Debugf("Cache hit for key: %s", key)
		return nil
	}
	
	// Cache miss, compute value
	logger.Debugf("Cache miss for key: %s", key)
	value, err := fn()
	if err != nil {
		return err
	}
	
	// Store in cache
	if err := SetJSON(key, value, expiration); err != nil {
		logger.Warningf("Failed to set cache for key %s: %v", key, err)
	}
	
	// Copy value to dest
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// InvalidateInbounds invalidates all inbounds cache for a user.
func InvalidateInbounds(userId int) error {
	pattern := fmt.Sprintf("%s%d", KeyInboundsPrefix, userId)
	return DeletePattern(pattern)
}

// InvalidateAllInbounds invalidates all inbounds cache.
func InvalidateAllInbounds() error {
	pattern := KeyInboundsPrefix + "*"
	return DeletePattern(pattern)
}

// InvalidateClients invalidates all clients cache for a user.
func InvalidateClients(userId int) error {
	pattern := fmt.Sprintf("%s%d", KeyClientsPrefix, userId)
	return DeletePattern(pattern)
}

// InvalidateAllClients invalidates all clients cache.
func InvalidateAllClients() error {
	pattern := KeyClientsPrefix + "*"
	return DeletePattern(pattern)
}

// InvalidateSetting invalidates a specific setting cache.
// Note: We don't invalidate KeySettingsAll here to avoid unnecessary cache misses.
// KeySettingsAll will be invalidated only when settings are actually changed.
func InvalidateSetting(key string) error {
	settingKey := KeySettingPrefix + key
	return Delete(settingKey)
}

// InvalidateAllSettings invalidates all settings cache.
func InvalidateAllSettings() error {
	if err := Delete(KeySettingsAll); err != nil {
		return err
	}
	// Also invalidate all individual settings
	pattern := KeySettingPrefix + "*"
	return DeletePattern(pattern)
}
