package controller

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

// newAPIAuthTestEngine builds a gin engine that mirrors the production auth
// wiring: the sessions middleware, then checkAPIAuth guarding a sentinel
// handler that reports whether c.Next() was reached and whether api_authed was
// set. The APIController is the zero value, exactly as NewAPIController leaves
// its service fields (they query the global DB), so this exercises the real
// auth path. A fresh temp DB is initialised per test.
func newAPIAuthTestEngine(t *testing.T) (*gin.Engine, *APIController) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	engine := gin.New()
	store := cookie.NewStore([]byte("api-auth-test-secret"))
	engine.Use(sessions.Sessions("3x-ui", store))

	a := &APIController{}

	// Logs in as the first user so the session path can be exercised over a
	// cookie round-trip without reaching into checkAPIAuth's internals.
	engine.GET("/test-login", func(c *gin.Context) {
		u, err := a.userService.GetFirstUser()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		if err := session.SetLoginUser(c, u); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"api_authed": c.GetBool("api_authed")})
	})
	return engine, a
}

// TestCheckAPIAuth_BearerSuccess characterizes the bearer-token path: a valid
// token reaches the handler and sets api_authed (the contract the later
// client-cert branch must match).
func TestCheckAPIAuth_BearerSuccess(t *testing.T) {
	engine, _ := newAPIAuthTestEngine(t)

	const plaintext = "characterization-token-value"
	if err := database.GetDB().Create(&model.ApiToken{
		Name:    "t1",
		Token:   crypto.HashTokenSHA256(plaintext),
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/panel/api/ping", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != `{"api_authed":true}` {
		t.Fatalf("body = %s, want api_authed true", got)
	}
}

// TestCheckAPIAuth_AcceptsVerifiedClientCert asserts that a completed mTLS
// handshake (a non-empty verified client chain) authenticates the request even
// with no bearer token and no session — the equivalent of a valid token — and
// sets api_authed so the CSRF middleware lets mutations through.
func TestCheckAPIAuth_AcceptsVerifiedClientCert(t *testing.T) {
	engine, _ := newAPIAuthTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/panel/api/ping", nil)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{}}},
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != `{"api_authed":true}` {
		t.Fatalf("body = %s, want api_authed true", got)
	}
}

// TestCheckAPIAuth_EmptyVerifiedChainsFallsThrough asserts a TLS request with no
// verified client chain is NOT treated as authenticated (it falls through to the
// bearer/session paths) — so the cert branch can't accidentally authorize plain
// browser HTTPS.
func TestCheckAPIAuth_EmptyVerifiedChainsFallsThrough(t *testing.T) {
	engine, _ := newAPIAuthTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/panel/api/ping", nil)
	req.TLS = &tls.ConnectionState{} // handshake done, but no client cert verified
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (unauthenticated, no verified chain)", w.Code)
	}
}

// TestCheckAPIAuth_RejectsUnauthenticated characterizes the reject paths: no
// bearer token and no session yields 401 for XHR callers and 404 otherwise.
func TestCheckAPIAuth_RejectsUnauthenticated(t *testing.T) {
	engine, _ := newAPIAuthTestEngine(t)

	cases := []struct {
		name string
		xhr  bool
		want int
	}{
		{"xhr gets 401", true, http.StatusUnauthorized},
		{"non-xhr gets 404", false, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/panel/api/ping", nil)
			if c.xhr {
				req.Header.Set("X-Requested-With", "XMLHttpRequest")
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != c.want {
				t.Fatalf("status = %d, want %d", w.Code, c.want)
			}
		})
	}
}

// TestCheckAPIAuth_SessionLoginPasses characterizes the session path: a
// logged-in browser session (no bearer token) reaches the handler.
func TestCheckAPIAuth_SessionLoginPasses(t *testing.T) {
	engine, _ := newAPIAuthTestEngine(t)

	db := database.GetDB()
	var n int64
	if err := db.Model(&model.User{}).Count(&n).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n == 0 {
		if err := db.Create(&model.User{Username: "sess", Password: "x"}).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	ts := httptest.NewServer(engine)
	defer ts.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}

	loginResp, err := client.Get(ts.URL + "/test-login")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.StatusCode)
	}

	pingResp, err := client.Get(ts.URL + "/panel/api/ping")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	pingResp.Body.Close()
	if pingResp.StatusCode != http.StatusOK {
		t.Fatalf("session ping status = %d, want 200", pingResp.StatusCode)
	}
}
