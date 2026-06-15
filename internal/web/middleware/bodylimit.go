package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes caps the request body size for state-changing requests. It wraps
// the body in an http.MaxBytesReader so that any handler reading it (gin's
// ShouldBind, manual io.ReadAll, etc.) receives an error once the limit is
// exceeded, which the existing bind-failure path reports as a 400 rather than
// allocating an unbounded buffer or starting a long DB transaction.
//
// Methods without a body (GET/HEAD/OPTIONS/TRACE) and a non-positive limit are
// passed through untouched. Paths ending in one of skipSuffixes are also passed
// through uncapped — these are routes that legitimately accept a large upload
// (e.g. database restore, which streams a multi-MiB SQLite file).
func MaxBodyBytes(limit int64, skipSuffixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			default:
				if c.Request.Body != nil && !hasSuffix(c.Request.URL.Path, skipSuffixes) {
					c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
				}
			}
		}
		c.Next()
	}
}

// hasSuffix reports whether path ends in any of the given suffixes.
func hasSuffix(path string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
