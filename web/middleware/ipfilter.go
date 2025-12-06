package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/logger"
	redisutil "github.com/mhsanaei/3x-ui/v2/util/redis"
)

// IPFilterConfig configures IP filtering
type IPFilterConfig struct {
	WhitelistEnabled bool
	BlacklistEnabled bool
	GeoIPEnabled     bool
	BlockedCountries []string
	SkipPaths        []string // Paths to skip IP filtering
}

// shouldSkip checks if path should be skipped
func (config IPFilterConfig) shouldSkip(path string) bool {
	for _, skipPath := range config.SkipPaths {
		if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
			return true
		}
	}
	return false
}

// IPFilterMiddleware creates IP filtering middleware
func IPFilterMiddleware(config IPFilterConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip IP filtering for certain paths
		if config.shouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		ip := c.ClientIP()

		// Validate IP format
		if !ValidateIP(ip) {
			logger.Warningf("Invalid IP format: %s", ip)
			c.Next()
			return
		}

		// Check blacklist first
		if config.BlacklistEnabled {
			isBlocked, err := redisutil.SIsMember("ip:blacklist", ip)
			if err == nil && isBlocked {
				logger.Warningf("Blocked IP attempted access: %s", ip)
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"msg":     "Access denied",
				})
				c.Abort()
				return
			}
		}

		// Check whitelist if enabled
		if config.WhitelistEnabled {
			isWhitelisted, err := redisutil.SIsMember("ip:whitelist", ip)
			if err == nil && !isWhitelisted {
				logger.Warningf("Non-whitelisted IP attempted access: %s", ip)
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"msg":     "Access denied",
				})
				c.Abort()
				return
			}
		}

		// Check GeoIP blocking
		if config.GeoIPEnabled && len(config.BlockedCountries) > 0 {
			country, err := getCountryFromIP(ip)
			if err == nil && country != "" {
				for _, blockedCountry := range config.BlockedCountries {
					if strings.EqualFold(country, blockedCountry) {
						logger.Warningf("Blocked country attempted access: %s from %s", country, ip)
						c.JSON(http.StatusForbidden, gin.H{
							"success": false,
							"msg":     "Access denied",
						})
						c.Abort()
						return
					}
				}
			}
		}

		c.Next()
	}
}

// getCountryFromIP gets country code from IP (simplified version)
// In production, use MaxMind GeoIP2 database
func getCountryFromIP(ip string) (string, error) {
	// Check cache first
	cacheKey := "geoip:" + ip
	country, err := redisutil.Get(cacheKey)
	if err == nil && country != "" {
		return country, nil
	}

	// For now, return empty (will be implemented with MaxMind)
	// This is a placeholder
	return "", nil
}

// AddToBlacklist adds IP to blacklist
func AddToBlacklist(ip string) error {
	if !ValidateIP(ip) {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return redisutil.SAdd("ip:blacklist", ip)
}

// RemoveFromBlacklist removes IP from blacklist
func RemoveFromBlacklist(ip string) error {
	return redisutil.SRem("ip:blacklist", ip)
}

// AddToWhitelist adds IP to whitelist
func AddToWhitelist(ip string) error {
	if !ValidateIP(ip) {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return redisutil.SAdd("ip:whitelist", ip)
}

// RemoveFromWhitelist removes IP from whitelist
func RemoveFromWhitelist(ip string) error {
	return redisutil.SRem("ip:whitelist", ip)
}

// IsIPBlocked checks if IP is blocked
func IsIPBlocked(ip string) (bool, error) {
	return redisutil.SIsMember("ip:blacklist", ip)
}

// ValidateIP validates IP address format
func ValidateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil
}
