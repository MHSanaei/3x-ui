package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/oauth"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

func TestResolveRole(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		cfg    config.OAuthConfig
		want   string
	}{
		{"admin match", []string{"admins"}, config.OAuthConfig{AdminGroup: "admins", UserGroups: []string{"vpn"}}, session.RoleAdmin},
		{"user match", []string{"vpn"}, config.OAuthConfig{AdminGroup: "admins", UserGroups: []string{"vpn"}}, session.RoleUser},
		{"admin wins over user", []string{"admins", "vpn"}, config.OAuthConfig{AdminGroup: "admins", UserGroups: []string{"vpn"}}, session.RoleAdmin},
		{"no permitted group", []string{"other"}, config.OAuthConfig{AdminGroup: "admins", UserGroups: []string{"vpn"}}, ""},
		{"empty admin group ignored", []string{"admins"}, config.OAuthConfig{AdminGroup: "", UserGroups: []string{"vpn"}}, ""},
		{"user tier without user groups", []string{"vpn"}, config.OAuthConfig{AdminGroup: "admins"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := &oauth.Identity{Groups: tt.groups}
			if got := resolveRole(id, tt.cfg); got != tt.want {
				t.Fatalf("resolveRole = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		basePath string
		xfProto  string
		want     string
	}{
		{"plain http root", "http://panel.example/x", "/", "", "http://panel.example/oauth/callback"},
		{"forwarded https", "http://panel.example/x", "/", "https", "https://panel.example/oauth/callback"},
		{"base path", "http://panel.example/x", "/xui/", "", "http://panel.example/xui/oauth/callback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			if tt.xfProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.xfProto)
			}
			c.Request = req
			c.Set("base_path", tt.basePath)
			if got := deriveRedirectURL(c); got != tt.want {
				t.Fatalf("deriveRedirectURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func newOAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	r.Use(func(c *gin.Context) { c.Set("base_path", "/") })
	a := &IndexController{}
	r.GET("/getOAuthEnable", a.getOAuthEnable)
	r.GET("/oauth/callback", a.oauthCallback)
	return r
}

func TestGetOAuthEnable(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		t.Setenv("XUI_OAUTH_ISSUER", "https://idp.example")
		t.Setenv("XUI_OAUTH_CLIENT_ID", "cid")
		rec := httptest.NewRecorder()
		newOAuthRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/getOAuthEnable", nil))
		if !strings.Contains(rec.Body.String(), `"obj":true`) {
			t.Fatalf("body = %s, want obj true", rec.Body.String())
		}
	})
	t.Run("disabled", func(t *testing.T) {
		t.Setenv("XUI_OAUTH_ISSUER", "")
		t.Setenv("XUI_OAUTH_CLIENT_ID", "")
		rec := httptest.NewRecorder()
		newOAuthRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/getOAuthEnable", nil))
		if !strings.Contains(rec.Body.String(), `"obj":false`) {
			t.Fatalf("body = %s, want obj false", rec.Body.String())
		}
	})
}

func TestCheckLoginGating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := &BaseController{}
	r := gin.New()
	r.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	r.Use(func(c *gin.Context) { c.Set("base_path", "/") })
	r.GET("/seed-user", func(c *gin.Context) {
		if err := session.SetLoginClientSubID(c, "sub-abc"); err != nil {
			t.Fatal(err)
		}
		c.Status(http.StatusNoContent)
	})
	r.GET("/protected", base.checkLogin, func(c *gin.Context) { c.Status(http.StatusOK) })

	t.Run("anonymous redirects to login", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
		}
		if loc := rec.Header().Get("Location"); loc != "/" {
			t.Fatalf("Location = %q, want /", loc)
		}
	})

	t.Run("user tier redirects to cabinet", func(t *testing.T) {
		seed := httptest.NewRecorder()
		r.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed-user", nil))
		cookie := seed.Header().Get("Set-Cookie")
		if cookie == "" {
			t.Fatal("no session cookie from seed")
		}
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Cookie", cookie)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
		}
		if loc := rec.Header().Get("Location"); loc != "/cabinet/" {
			t.Fatalf("Location = %q, want /cabinet/", loc)
		}
	})
}

func TestCabinetDataRequiresUserSession(t *testing.T) {
	a := &IndexController{}
	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	r.Use(func(c *gin.Context) { c.Set("base_path", "/") })
	r.GET("/cabinet/data", a.cabinetData)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cabinet/data", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d for anonymous cabinet access", rec.Code, http.StatusUnauthorized)
	}
}

func TestCabinetData_LinksUsePortStrippedHost(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	subID := "cab-sub-1"
	uuid := "a5796ed0-abde-4bc4-bd0e-ed4a88a7be0b"
	email := "u@e"
	settings := fmt.Sprintf(`{"clients":[{"id":%q,"email":%q,"subId":%q,"enable":true}],"decryption":"none"}`, uuid, email, subID)
	stream := `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["r.example.com"],"shortIds":["ab"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}}`
	ib := &model.Inbound{UserId: 1, Tag: "VLESS-443", Enable: true, Port: 443, Protocol: model.VLESS, Remark: "VLESS-443", Settings: settings, StreamSettings: stream}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	cr := &model.ClientRecord{Email: email, SubID: subID, UUID: uuid, Enable: true}
	if err := db.Create(cr).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: cr.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	a := &IndexController{}
	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	r.Use(func(c *gin.Context) { c.Set("base_path", "/") })
	r.GET("/seed", func(c *gin.Context) {
		if err := session.SetLoginClientSubID(c, subID); err != nil {
			t.Fatal(err)
		}
		c.Status(http.StatusNoContent)
	})
	r.GET("/cabinet/data", a.cabinetData)

	seed := httptest.NewRecorder()
	r.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed", nil))
	cookieHdr := seed.Header().Get("Set-Cookie")

	req := httptest.NewRequest(http.MethodGet, "/cabinet/data", nil)
	req.Host = "localhost:2053"
	req.Header.Set("Cookie", cookieHdr)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "[localhost:2053]") {
		t.Fatalf("cabinet link carries the malformed bracketed host:port: %s", body)
	}
	if !strings.Contains(body, "type=tcp") {
		t.Fatalf("cabinet link is missing stream params (bad host resolution?): %s", body)
	}
	if !strings.Contains(body, "localhost:443") {
		t.Fatalf("cabinet link should fall back to the port-stripped host: %s", body)
	}
}

func TestOAuthCallbackDenies(t *testing.T) {
	t.Setenv("XUI_OAUTH_ISSUER", "https://idp.example")
	t.Setenv("XUI_OAUTH_CLIENT_ID", "cid")

	tests := []struct {
		name  string
		query string
	}{
		{"provider error", "error=access_denied&state=x"},
		{"no stored state", "state=whatever&code=abc"},
		{"missing state param", "code=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newOAuthRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/callback?"+tt.query, nil))
			if rec.Code != http.StatusTemporaryRedirect {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
			}
			if loc := rec.Header().Get("Location"); loc != "/?oauth_error=1" {
				t.Fatalf("Location = %q, want /?oauth_error=1", loc)
			}
		})
	}
}
