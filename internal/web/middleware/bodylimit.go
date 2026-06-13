package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes caps the request body size for state-changing requests. It wraps
// the body in an http.MaxBytesReader so that any handler reading it (gin's
// ShouldBind, manual io.ReadAll, etc.) receives an error once the limit is
// exceeded, which the existing bind-failure path reports as a 400 rather than
// allocating an unbounded buffer or starting a long DB transaction.
//
// Methods without a body (GET/HEAD/OPTIONS/TRACE) and a non-positive limit are
// passed through untouched.
func MaxBodyBytes(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			default:
				if c.Request.Body != nil {
					c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
				}
			}
		}
		c.Next()
	}
}
