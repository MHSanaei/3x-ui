package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

// AuditMiddleware logs all actions to audit log
func AuditMiddleware() gin.HandlerFunc {
	auditService := service.AuditLogService{}

	return func(c *gin.Context) {
		// Skip audit for certain paths
		path := c.Request.URL.Path
		if shouldSkipAudit(path) {
			c.Next()
			return
		}

		// Get user info
		user := session.GetLoginUser(c)
		if user == nil {
			c.Next()
			return
		}

		// Log after request completes
		c.Next()

		// Extract action and resource from path
		action, resource, resourceID := extractActionFromPath(c.Request.Method, path)

		// Log the action
		details := map[string]interface{}{
			"method": c.Request.Method,
			"path":   path,
		}

		if err := auditService.LogAction(
			user.Id,
			user.Username,
			action,
			resource,
			resourceID,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			details,
		); err != nil {
			logger.Warning("Failed to log audit action:", err)
		}
	}
}

// shouldSkipAudit checks if path should be skipped from audit
func shouldSkipAudit(path string) bool {
	skipPaths := []string{
		"/assets/",
		"/favicon.ico",
		"/ws",
		"/api/",
	}
	for _, skipPath := range skipPaths {
		if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
			return true
		}
	}
	return false
}

// extractActionFromPath extracts action, resource and resource ID from path
func extractActionFromPath(method, path string) (action, resource string, resourceID int) {
	// Map HTTP methods to actions
	switch method {
	case "POST":
		if contains(path, "/add") || contains(path, "/create") {
			action = "CREATE"
		} else if contains(path, "/update") || contains(path, "/modify") {
			action = "UPDATE"
		} else {
			action = "POST"
		}
	case "DELETE":
		action = "DELETE"
	case "GET":
		action = "READ"
	case "PUT":
		action = "UPDATE"
	default:
		action = method
	}

	// Extract resource type
	if contains(path, "/inbound") {
		resource = "inbound"
	} else if contains(path, "/client") {
		resource = "client"
	} else if contains(path, "/setting") {
		resource = "setting"
	} else if contains(path, "/user") {
		resource = "user"
	} else {
		resource = "unknown"
	}

	// Extract resource ID if present (simplified)
	// In production, parse from path parameters

	return action, resource, 0
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
