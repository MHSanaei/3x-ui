package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestSetLoginUserStoresOnlyUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions(sessionCookieName, cookie.NewStore([]byte("01234567890123456789012345678901"))))
	router.GET("/", func(c *gin.Context) {
		if err := SetLoginUser(c, &model.User{Id: 7, Username: "admin", Password: "hash"}); err != nil {
			t.Fatal(err)
		}
		got := sessions.Default(c).Get(loginUserKey)
		if got != 7 {
			t.Fatalf("stored session payload = %#v, want user id only", got)
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestSessionUserIDSupportsLegacyUserPayload(t *testing.T) {
	id, ok := sessionUserID(model.User{Id: 11, Username: "admin", Password: "hash"})
	if !ok || id != 11 {
		t.Fatalf("legacy session payload resolved to (%d, %v), want (11, true)", id, ok)
	}
	id, ok = sessionUserID(&model.User{Id: 12, Username: "admin", Password: "hash"})
	if !ok || id != 12 {
		t.Fatalf("legacy pointer session payload resolved to (%d, %v), want (12, true)", id, ok)
	}
}

// Anyone can fetch an anonymous session with its CSRF token and plant that cookie in a
// browser from a sibling host; the token must not survive the login it was planted for.
func TestSetLoginUserReplacesThePreLoginCSRFToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions(sessionCookieName, cookie.NewStore([]byte("01234567890123456789012345678901"))))
	var planted, afterLogin string
	router.GET("/csrf", func(c *gin.Context) {
		planted, _ = EnsureCSRFToken(c)
		c.Status(http.StatusNoContent)
	})
	router.GET("/login", func(c *gin.Context) {
		if err := SetLoginUser(c, &model.User{Id: 7}); err != nil {
			t.Fatal(err)
		}
		afterLogin, _ = EnsureCSRFToken(c)
		c.Status(http.StatusNoContent)
	})
	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/csrf", nil))
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	for _, cookie := range first.Result().Cookies() {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(httptest.NewRecorder(), req)
	if planted == "" || afterLogin == "" || afterLogin == planted {
		t.Fatalf("CSRF token before login %q, after login %q; want a fresh token", planted, afterLogin)
	}
}
