package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestValidateRegex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSettingController(router.Group("/panel/api"))

	tests := []struct {
		name    string
		body    string
		success bool
	}{
		{name: "Go RE2 inline flag", body: `{"regex":"(?m)^general-purpose$"}`, success: true},
		{name: "invalid expression", body: `{"regex":"["}`, success: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/panel/api/setting/validateRegex", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			needle := `"success":true`
			if !tt.success {
				needle = `"success":false`
			}
			if !strings.Contains(resp.Body.String(), needle) {
				t.Fatalf("body = %s, want %s", resp.Body.String(), needle)
			}
		})
	}
}

func TestAPITokenMutationRoutesEnforceExpectedScope(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	row := &model.ApiToken{Name: "route-scope", Token: crypto.HashTokenSHA256("token"), Enabled: true, Scope: model.ApiScopeNodeSync}
	if err := database.GetDB().Create(row).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSettingController(router.Group("/panel/api"))
	for _, path := range []string{
		"/panel/api/setting/apiTokens/delete/" + strconv.Itoa(row.Id),
		"/panel/api/setting/apiTokens/setEnabled/" + strconv.Itoa(row.Id),
	} {
		body := `{"expectedScope":"admin","enabled":false}`
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if !strings.Contains(resp.Body.String(), `"success":false`) {
			t.Fatalf("%s accepted wrong expected scope: %s", path, resp.Body.String())
		}
	}
	var stored model.ApiToken
	if err := database.GetDB().First(&stored, row.Id).Error; err != nil {
		t.Fatalf("token was deleted by wrong scope: %v", err)
	}
	if !stored.Enabled {
		t.Fatal("token was disabled by wrong scope")
	}
}
