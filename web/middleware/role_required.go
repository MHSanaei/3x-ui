package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RoleRequired проверяет, есть ли у пользователя нужная роль.
func RoleRequired(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role") // где-то до этого роль должна быть положена в контекст
		if !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		role, ok := roleVal.(string)
		if !ok || !allowed[role] {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
