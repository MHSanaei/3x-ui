package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

func newSPAFallbackTestEngine(t *testing.T) *gin.Engine {
	return newSPAFallbackTestEngineWithBasePath(t, "/admin-random/")
}

func newSPAFallbackTestEngineWithBasePath(t *testing.T, basePath string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDistFS := distFS
	SetDistFS(fstest.MapFS{
		"dist/index.html": {Data: []byte(`<!doctype html><html><head></head><body>spa shell</body></html>`)},
	})
	t.Cleanup(func() { SetDistFS(oldDistFS) })

	engine := gin.New()
	engine.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("spa-fallback-test-secret"))))
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		if c.GetHeader("X-Test-Login") == "1" {
			session.SetAPIAuthUser(c, &model.User{Id: 1, Username: "test"})
		}
		c.Next()
	})

	ctrl := NewXUIController(engine.Group(basePath))
	engine.NoRoute(func(c *gin.Context) {
		if ctrl.HandleNoRoutePanelSPA(c) {
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	})
	return engine
}

func TestPanelSPAFallbackServesRootBasePath(t *testing.T) {
	engine := newSPAFallbackTestEngineWithBasePath(t, "/")
	req := httptest.NewRequest(http.MethodGet, "/panel/hosts", nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("X-Test-Login", "1")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "spa shell") {
		t.Fatalf("body does not contain SPA shell: %s", w.Body.String())
	}
}

func TestPanelSPAFallbackServesAuthenticatedClientRoutes(t *testing.T) {
	engine := newSPAFallbackTestEngine(t)

	for _, target := range []string{
		"/admin-random/panel/hosts",
		"/admin-random/panel/some/future/route",
	} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set("Accept", "text/html")
			req.Header.Set("X-Test-Login", "1")
			w := httptest.NewRecorder()

			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", ct)
			}
			if !strings.Contains(w.Body.String(), "spa shell") {
				t.Fatalf("body does not contain SPA shell: %s", w.Body.String())
			}
		})
	}
}

func TestPanelSPAFallbackPreservesAuthSemantics(t *testing.T) {
	engine := newSPAFallbackTestEngine(t)

	t.Run("browser redirects to login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin-random/panel/hosts", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()

		engine.ServeHTTP(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want 307", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/admin-random/" {
			t.Fatalf("Location = %q, want /admin-random/", loc)
		}
	})

	t.Run("ajax gets json unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin-random/panel/hosts", nil)
		req.Header.Set("Accept", "text/html")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()

		engine.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
	})
}

func TestPanelSPAFallbackExclusions(t *testing.T) {
	engine := newSPAFallbackTestEngine(t)

	for _, tc := range []struct {
		target string
		want   int
	}{
		{target: "/admin-random/panel/api", want: http.StatusNotFound},
		{target: "/admin-random/panel/api/unknown", want: http.StatusNotFound},
		{target: "/admin-random/panel/csrf-token/", want: http.StatusMovedPermanently},
		{target: "/admin-random/panel/missing.js", want: http.StatusNotFound},
	} {
		t.Run(tc.target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			req.Header.Set("Accept", "text/html")
			req.Header.Set("X-Test-Login", "1")
			w := httptest.NewRecorder()

			engine.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "spa shell") {
				t.Fatalf("excluded route was served by SPA fallback: %s", w.Body.String())
			}
		})
	}
}

func TestPanelCSRFTokenRemainsExplicit(t *testing.T) {
	engine := newSPAFallbackTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/admin-random/panel/csrf-token", nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("X-Test-Login", "1")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if strings.Contains(w.Body.String(), "spa shell") {
		t.Fatalf("csrf-token was served by SPA fallback: %s", w.Body.String())
	}
}

func TestPanelSPAFallbackPredicate(t *testing.T) {
	oldDistFS := distFS
	SetDistFS(fstest.MapFS{})
	t.Cleanup(func() { SetDistFS(oldDistFS) })

	cases := []struct {
		name   string
		method string
		path   string
		accept string
		want   bool
	}{
		{name: "panel root", method: http.MethodGet, path: "/admin-random/panel", accept: "text/html", want: true},
		{name: "panel descendant", method: http.MethodGet, path: "/admin-random/panel/hosts", accept: "*/*", want: true},
		{name: "empty accept", method: http.MethodGet, path: "/admin-random/panel/future", want: true},
		{name: "post excluded", method: http.MethodPost, path: "/admin-random/panel/hosts", accept: "text/html"},
		{name: "json accept excluded", method: http.MethodGet, path: "/admin-random/panel/hosts", accept: "application/json"},
		{name: "api root excluded", method: http.MethodGet, path: "/admin-random/panel/api", accept: "text/html"},
		{name: "api descendant excluded", method: http.MethodGet, path: "/admin-random/panel/api/unknown", accept: "text/html"},
		{name: "csrf excluded", method: http.MethodGet, path: "/admin-random/panel/csrf-token", accept: "text/html"},
		{name: "csrf descendant excluded", method: http.MethodGet, path: "/admin-random/panel/csrf-token/", accept: "text/html"},
		{name: "file excluded", method: http.MethodGet, path: "/admin-random/panel/missing.css", accept: "text/html"},
		{name: "outside panel excluded", method: http.MethodGet, path: "/admin-random/hosts", accept: "text/html"},
		{name: "dotted email route param served", method: http.MethodGet, path: "/admin-random/panel/clients/user@example.com", accept: "text/html", want: true},
		{name: "dotted version route param served", method: http.MethodGet, path: "/admin-random/panel/sub/1.2.3", accept: "text/html", want: true},
		{name: "uppercase asset extension excluded", method: http.MethodGet, path: "/admin-random/panel/app.JS", accept: "text/html"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("base_path", "/admin-random/")
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			c.Request = req

			if got := isPanelSPAFallbackRequest(c); got != tc.want {
				t.Fatalf("isPanelSPAFallbackRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}
