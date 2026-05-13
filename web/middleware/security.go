package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds browser hardening headers to panel responses.
func SecurityHeadersMiddleware(directHTTPS bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce := newCSPNonce()
		c.Set("csp_nonce", nonce)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws: wss:; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if directHTTPS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func newCSPNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(b[:])
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
