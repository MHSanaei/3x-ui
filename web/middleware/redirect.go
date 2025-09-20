package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RedirectMiddleware returns a Gin middleware that handles URL redirections.
// It provides backward compatibility by redirecting old '/xui' paths to new '/panel' paths,
// including API endpoints. The middleware performs permanent redirects (301) for SEO purposes.
func RedirectMiddleware(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Redirect from old '/xui' path to '/panel'
		redirects := map[string]string{
			"panel/API": "panel/api",
			"xui/API":   "panel/api",
			"xui":       "panel",
		}

		path := c.Request.URL.Path
		for from, to := range redirects {
			from, to = basePath+from, basePath+to

			if strings.HasPrefix(path, from) {
				newPath := to + path[len(from):]

				c.Redirect(http.StatusMovedPermanently, newPath)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
