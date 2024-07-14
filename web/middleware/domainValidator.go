package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func DomainValidatorMiddleware(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.GetHeader("X-Real-IP")
		}
		if host == "" {
			host = c.Request.Host
			if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
				host, _, _ = net.SplitHostPort(host)
			}
		}

		if host != domain {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
