package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/logger"
	redisutil "github.com/mhsanaei/3x-ui/v2/util/redis"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

// DeviceFingerprint generates device fingerprint
func DeviceFingerprint(c *gin.Context) string {
	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()
	acceptLanguage := c.GetHeader("Accept-Language")
	acceptEncoding := c.GetHeader("Accept-Encoding")

	data := fmt.Sprintf("%s|%s|%s|%s", userAgent, ip, acceptLanguage, acceptEncoding)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// SessionSecurityMiddleware enforces session security
func SessionSecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := session.GetLoginUser(c)
		if user == nil {
			c.Next()
			return
		}

		// Get device fingerprint
		fingerprint := DeviceFingerprint(c)
		sessionKey := fmt.Sprintf("session:%d", user.Id)
		deviceKey := fmt.Sprintf("device:%d:%s", user.Id, fingerprint)

		// Check if device is registered
		deviceExists, err := redisutil.Exists(deviceKey)
		if err == nil && !deviceExists {
			// New device - check max devices limit
			// TODO: Get from settings
			maxDevices := 5 // Default, should be configurable
			devices, _ := redisutil.SMembers(fmt.Sprintf("devices:%d", user.Id))
			if len(devices) >= maxDevices {
				logger.Warningf("User %d attempted to login from too many devices", user.Id)
				session.ClearSession(c)
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"msg":     "Maximum number of devices reached",
				})
				c.Abort()
				return
			}

			// Register new device
			redisutil.SAdd(fmt.Sprintf("devices:%d", user.Id), fingerprint)
			redisutil.Set(deviceKey, time.Now().Format(time.RFC3339), 30*24*time.Hour)
		}

		// Check session validity
		sessionData, err := redisutil.HGetAll(sessionKey)
		if err == nil {
			// Check IP change
			if storedIP, ok := sessionData["ip"]; ok && storedIP != c.ClientIP() {
				logger.Warningf("IP change detected for user %d: %s -> %s", user.Id, storedIP, c.ClientIP())
				// Optionally force re-login on IP change
				// session.ClearSession(c)
				// c.Abort()
				// return
			}

			// Update last activity
			redisutil.HSet(sessionKey, "last_activity", time.Now().Unix())
			redisutil.HSet(sessionKey, "ip", c.ClientIP())
			redisutil.Expire(sessionKey, 24*time.Hour)
		}

		c.Next()
	}
}

// ForceLogoutDevice forces logout from specific device
func ForceLogoutDevice(userId int, fingerprint string) error {
	deviceKey := fmt.Sprintf("device:%d:%s", userId, fingerprint)
	return redisutil.Del(deviceKey)
}

// GetUserDevices returns all devices for user
func GetUserDevices(userId int) ([]string, error) {
	return redisutil.SMembers(fmt.Sprintf("devices:%d", userId))
}
