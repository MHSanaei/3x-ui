package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func DomainValidatorMiddleware(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := strings.Split(c.Request.Host, ":")[0]

		if host != domain {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
