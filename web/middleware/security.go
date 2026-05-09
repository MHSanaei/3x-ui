package middleware

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds browser hardening headers to panel responses.
func SecurityHeadersMiddleware(directHTTPS bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if directHTTPS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// CSRFMiddleware rejects unsafe requests that do not include the session CSRF token.
// Bearer-token-authenticated callers (api_authed flag set by APIController.checkAPIAuth)
// short-circuit the CSRF check — they are not browser sessions, so the
// cross-site request forgery threat model doesn't apply to them.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("api_authed") {
			c.Next()
			return
		}
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		if !session.ValidateCSRFToken(c) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
