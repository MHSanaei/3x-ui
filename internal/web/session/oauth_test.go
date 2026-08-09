package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newSessionRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions(sessionCookieName, cookie.NewStore([]byte("01234567890123456789012345678901"))))
	return r
}

func TestGetLoginRoleDefaultsToAdmin(t *testing.T) {
	r := newSessionRouter()
	r.GET("/", func(c *gin.Context) {
		if got := GetLoginRole(c); got != RoleAdmin {
			t.Fatalf("GetLoginRole with nothing set = %q, want %q", got, RoleAdmin)
		}
		c.Status(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestSetGetLoginRole(t *testing.T) {
	r := newSessionRouter()
	r.GET("/", func(c *gin.Context) {
		if err := SetLoginRole(c, RoleUser); err != nil {
			t.Fatal(err)
		}
		if got := GetLoginRole(c); got != RoleUser {
			t.Fatalf("GetLoginRole = %q, want %q", got, RoleUser)
		}
		c.Status(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestOAuthFlowRoundTripAndClear(t *testing.T) {
	r := newSessionRouter()
	r.GET("/", func(c *gin.Context) {
		if err := SetOAuthFlow(c, "st", "no", "vf"); err != nil {
			t.Fatal(err)
		}
		s, n, v := GetOAuthFlow(c)
		if s != "st" || n != "no" || v != "vf" {
			t.Fatalf("GetOAuthFlow = %q/%q/%q, want st/no/vf", s, n, v)
		}
		if err := ClearOAuthFlow(c); err != nil {
			t.Fatal(err)
		}
		s, n, v = GetOAuthFlow(c)
		if s != "" || n != "" || v != "" {
			t.Fatalf("after clear GetOAuthFlow = %q/%q/%q, want empty", s, n, v)
		}
		c.Status(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}
