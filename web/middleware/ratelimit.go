package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/logger"
	redisutil "github.com/mhsanaei/3x-ui/v2/util/redis"
)

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
	KeyFunc           func(c *gin.Context) string
	SkipPaths         []string // Paths to skip rate limiting
}

// DefaultRateLimitConfig returns default rate limit config
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP()
		},
		SkipPaths: []string{"/assets/", "/favicon.ico"},
	}
}

// shouldSkip checks if path should be skipped
func (config RateLimitConfig) shouldSkip(path string) bool {
	for _, skipPath := range config.SkipPaths {
		if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
			return true
		}
	}
	return false
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for certain paths
		if config.shouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		key := config.KeyFunc(c)
		rateLimitKey := "ratelimit:" + key + ":" + c.Request.URL.Path

		// Get current count
		countStr, err := redisutil.Get(rateLimitKey)
		var count int
		if err != nil {
			// Key doesn't exist, start with 0
			count = 0
		} else {
			count, _ = strconv.Atoi(countStr)
		}

		if count >= config.RequestsPerMinute {
			logger.Warningf("Rate limit exceeded for %s on %s (count: %d)", key, c.Request.URL.Path, count)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"msg":     "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		// Increment counter
		newCount, err := redisutil.Incr(rateLimitKey)
		if err != nil {
			logger.Warning("Rate limit increment failed:", err)
			c.Next()
			return
		}

		// Set expiration on first request
		if newCount == 1 {
			redisutil.Expire(rateLimitKey, time.Minute)
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(config.RequestsPerMinute-int(newCount)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		c.Next()
	}
}
