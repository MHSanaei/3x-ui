package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestActiveProfilesEndpointScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeSync, model.ApiScopeMonitor} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("active-profile-test-session-secret"))))
	a := &APIController{}
	group := router.Group("/panel/api")
	group.Use(a.checkAPIAuth, a.enforceTokenScope)
	NewClientController(group.Group("/clients"))
	for _, tc := range []struct {
		token string
		want  int
	}{{model.ApiScopeAdmin, 200}, {model.ApiScopeNodeSync, 200}, {model.ApiScopeMonitor, 403}, {"invalid", 401}, {"", 404}} {
		t.Run(tc.token, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/panel/api/clients/activeProfiles", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
