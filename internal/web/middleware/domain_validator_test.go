package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func domainStatus(t *testing.T, domain, host string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(DomainValidatorMiddleware(domain))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

// A domain saved as "Panel.Example.com" or with a stray space answered every
// request with 403, locking the admin out of the panel.
func TestDomainValidatorComparesHostNamesCaseInsensitively(t *testing.T) {
	tests := []struct {
		name, domain, host string
		want               int
	}{
		{"capitals in saved domain", "Panel.Example.com", "panel.example.com", http.StatusOK},
		{"space in saved domain", "panel.example.com ", "panel.example.com:2053", http.StatusOK},
		{"capitals in Host header", "panel.example.com", "PANEL.example.com", http.StatusOK},
		{"trailing dot in Host header", "panel.example.com", "panel.example.com.:443", http.StatusOK},
		{"bracketed IPv6 host", "2001:db8::1", "[2001:db8::1]:2053", http.StatusOK},
		{"other host", "panel.example.com", "evil.example.com", http.StatusForbidden},
		{"domain as a prefix", "panel.example.com", "panel.example.com.evil.net", http.StatusForbidden},
		{"empty host", "panel.example.com", "", http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domainStatus(t, tc.domain, tc.host); got != tc.want {
				t.Errorf("domain %q, host %q: status %d, want %d", tc.domain, tc.host, got, tc.want)
			}
		})
	}
}
