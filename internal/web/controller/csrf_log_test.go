package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
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

// A failed login keeps its username in the log buffer, the limiter table and the
// notifier queue; an unauthenticated caller must not be able to make that unbounded.
func TestLoginRefusesOversizedUsernameBeforeRecordingIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	engine := gin.New()
	engine.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("login-length-test-secret"))))
	engine.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewIndexController(engine.Group("/"))

	tokenReq := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	tokenRec := httptest.NewRecorder()
	engine.ServeHTTP(tokenRec, tokenReq)
	var token struct{ Obj string }
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &token); err != nil || token.Obj == "" {
		t.Fatalf("csrf token: %v %s", err, tokenRec.Body.String())
	}
	username := strings.Repeat("u", panel.MaxUsernameLength+1)
	form := url.Values{"username": {username}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.RemoteAddr = "198.51.100.10:40000"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(session.CSRFHeaderName, token.Obj)
	for _, cookie := range tokenRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("login = %d %s, want the generic failure", rec.Code, rec.Body.String())
	}
	for _, line := range logger.GetLogs(100, "DEBUG") {
		if strings.Contains(line, username) {
			t.Fatal("the oversized username reached the log buffer")
		}
	}
	defaultLoginLimiter.mu.Lock()
	defer defaultLoginLimiter.mu.Unlock()
	if _, kept := defaultLoginLimiter.attempts[loginLimitKey("198.51.100.10", username)]; kept {
		t.Fatal("the oversized username was kept in the limiter table")
	}
}
