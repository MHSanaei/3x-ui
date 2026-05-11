package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/web/session"

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
	router.GET("/", func(c *gin.Context) {
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
