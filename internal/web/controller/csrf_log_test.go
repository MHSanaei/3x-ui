package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestLoginCSRFRejectionLogsPeerIPNotForwardedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("csrf-log-test-secret"))))
	NewIndexController(engine.Group("/"))

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "198.51.100.9:40000"
	req.Header.Set("X-Forwarded-For", "203.0.113.66")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	var line string
	for _, l := range logger.GetLogs(50, "WARNING") {
		if strings.Contains(l, "CSRF validation failed") {
			line = l
			break
		}
	}
	if !strings.Contains(line, "IP=198.51.100.9") || strings.Contains(line, "203.0.113.66") {
		t.Fatalf("CSRF rejection log = %q, want the untrusted peer 198.51.100.9 and not the forwarded header", line)
	}
}
