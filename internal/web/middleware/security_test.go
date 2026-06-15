package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestCSRFMiddlewareAllowsSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CSRFMiddleware())
	router.GET("/safe", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCSRFMiddlewareRejectsMissingTokenAndAcceptsValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("01234567890123456789012345678901"))
	router.Use(sessions.Sessions("3x-ui", store))
	router.GET("/token", func(c *gin.Context) {
		token, err := session.EnsureCSRFToken(c)
		if err != nil {
			t.Fatal(err)
		}
		c.String(http.StatusOK, token)
	})
	router.POST("/submit", CSRFMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	tokenRec := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodGet, "/token", nil)
	router.ServeHTTP(tokenRec, tokenReq)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("token status = %d, want %d", tokenRec.Code, http.StatusOK)
	}
	cookies := tokenRec.Result().Cookies()
	token := tokenRec.Body.String()

	missingRec := httptest.NewRecorder()
	missingReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	for _, cookie := range cookies {
		missingReq.AddCookie(cookie)
	}
	router.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusForbidden {
		t.Fatalf("missing token status = %d, want %d", missingRec.Code, http.StatusForbidden)
	}

	validRec := httptest.NewRecorder()
	validReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	for _, cookie := range cookies {
		validReq.AddCookie(cookie)
	}
	validReq.Header.Set(session.CSRFHeaderName, token)
	router.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("valid token status = %d, want %d", validRec.Code, http.StatusOK)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeadersMiddleware(true))
	var capturedNonce string
	router.GET("/", func(c *gin.Context) {
		capturedNonce = c.GetString("csp_nonce")
		c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)

	headers := rec.Result().Header
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := headers.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := headers.Get("Strict-Transport-Security"); got == "" {
		t.Fatal("Strict-Transport-Security should be set for direct HTTPS")
	}

	// CSP is the highest-value header here: assert it stays nonce-bound with its hardening
	// directives, so weakening it (unsafe-inline, dropped frame-ancestors, broken nonce) fails.
	csp := headers.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header must be set")
	}
	if capturedNonce == "" {
		t.Fatal("csp_nonce context value must be set (the injected inline script reads it)")
	}
	if want := "script-src 'self' 'nonce-" + capturedNonce + "'"; !strings.Contains(csp, want) {
		t.Fatalf("CSP script-src must be bound to the per-request nonce %q; got %q", want, csp)
	}
	for _, directive := range []string{"object-src 'none'", "frame-ancestors 'none'", "base-uri 'self'", "form-action 'self'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing hardening directive %q; got %q", directive, csp)
		}
	}
	// script-src must NOT allow 'unsafe-inline' (it would defeat the nonce). Check the
	// script-src directive in isolation, since style-src legitimately uses unsafe-inline.
	scriptDir := csp[strings.Index(csp, "script-src"):]
	if i := strings.Index(scriptDir, ";"); i >= 0 {
		scriptDir = scriptDir[:i]
	}
	if strings.Contains(scriptDir, "unsafe-inline") {
		t.Errorf("CSP script-src must not allow 'unsafe-inline': %q", scriptDir)
	}
}

func TestSecurityHeadersMiddlewareSkipsHSTSWithoutDirectHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeadersMiddleware(false))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)

	if got := rec.Result().Header.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty", got)
	}
}
