package middleware

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// RequireAdmin aborts the request unless the authenticated user has the admin
// role. It is the server-side enforcement point for every admin-only route and
// API — the frontend hides admin UI, but this is what actually prevents a
// non-admin from reaching admin functionality by calling the API directly.
//
// Bearer-token API callers are resolved to the first (admin) user by
// APIController.checkAPIAuth, so they pass. Session callers are checked against
// their stored role.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := session.GetLoginUser(c)
		if user == nil || !user.IsAdmin() {
			if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"msg":     "forbidden: admin role required",
				})
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}
