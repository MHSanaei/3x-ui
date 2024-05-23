package middleware

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

func DomainValidatorMiddleware(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.GetHeader("X-Real-IP")
		}
		if host == "" {
			host, _, _ := net.SplitHostPort(c.Request.Host)
			if host != domain {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		c.Next()
		}
	}
}
