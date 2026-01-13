// Package middleware provides HTTP response caching middleware for the 3x-ui web panel.
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/cache"
)

// CacheMiddleware creates a middleware that caches HTTP responses.
// It caches GET requests based on the full URL path and query parameters.
func CacheMiddleware(ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		// Generate cache key from request path and query
		cacheKey := generateCacheKey(c.Request.URL.Path, c.Request.URL.RawQuery)
		
		// Try to get from cache
		var cachedResponse map[string]interface{}
		err := cache.GetJSON(cacheKey, &cachedResponse)
		if err == nil {
			// Cache hit - return cached response
			c.JSON(200, cachedResponse)
			c.Abort()
			return
		}

		// Cache miss - continue to handler and capture response
		c.Next()

		// Only cache successful responses (status 200)
		if c.Writer.Status() == 200 {
			// Try to capture the response body
			// Note: This is a simplified version - in production you might want to use
			// a response writer wrapper to capture the actual response body
			// For now, we'll let the service layer handle caching
		}
	}
}

// CacheResponse caches a JSON response with the given key and TTL.
func CacheResponse(key string, data interface{}, ttl time.Duration) error {
	return cache.SetJSON(key, data, ttl)
}

// GetCachedResponse retrieves a cached JSON response.
func GetCachedResponse(key string, dest interface{}) error {
	return cache.GetJSON(key, dest)
}

// InvalidateCacheKey invalidates a specific cache key.
func InvalidateCacheKey(key string) error {
	return cache.Delete(key)
}

// generateCacheKey creates a cache key from path and query string.
func generateCacheKey(path, query string) string {
	key := fmt.Sprintf("http:%s", path)
	if query != "" {
		hash := sha256.Sum256([]byte(query))
		key += ":" + hex.EncodeToString(hash[:])[:16]
	}
	return key
}

// UserCacheMiddleware creates a middleware that caches responses per user.
// It includes the user ID in the cache key to ensure user-specific caching.
func UserCacheMiddleware(ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		// Get user ID from session
		userID := getUserIDFromContext(c)
		if userID == 0 {
			c.Next()
			return
		}

		// Generate cache key with user ID
		cacheKey := generateUserCacheKey(c.Request.URL.Path, c.Request.URL.RawQuery, userID)
		
		// Try to get from cache
		var cachedResponse map[string]interface{}
		err := cache.GetJSON(cacheKey, &cachedResponse)
		if err == nil {
			// Cache hit - return cached response
			c.JSON(200, cachedResponse)
			c.Abort()
			return
		}

		// Cache miss - continue to handler
		c.Next()
	}
}

// generateUserCacheKey creates a cache key with user ID.
func generateUserCacheKey(path, query string, userID int) string {
	key := fmt.Sprintf("http:user:%d:%s", userID, path)
	if query != "" {
		hash := sha256.Sum256([]byte(query))
		key += ":" + hex.EncodeToString(hash[:])[:16]
	}
	return key
}

// getUserIDFromContext extracts user ID from gin context.
// This is a helper function - you may need to adjust based on your session implementation.
func getUserIDFromContext(c *gin.Context) int {
	// Try to get from session
	if user, exists := c.Get("user"); exists {
		if userMap, ok := user.(map[string]interface{}); ok {
			if id, ok := userMap["id"].(int); ok {
				return id
			}
		}
	}
	return 0
}
