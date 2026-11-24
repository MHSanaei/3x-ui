package controller

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestVlessFallbackHTTPAndImportValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	folder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", folder)
	dbtest.InitDB(t, filepath.Join(folder, "x-ui.db"))
	previous := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(previous) })
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeSync} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	auth := &APIController{}
	group := router.Group("/panel/api")
	group.Use(auth.checkAPIAuth, auth.enforceTokenScope)
	NewInboundController(group.Group("/inbounds"))
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeSync} {
		for _, route := range []string{"add", "update", "import"} {
			if scope == model.ApiScopeNodeSync && route == "import" {
				continue
			}
			for _, invalid := range []bool{false, true} {
				t.Run(scope+"/"+route+"/"+strconv.FormatBool(invalid), func(t *testing.T) {
					raw := `[]`
					if invalid {
						raw = `[{"dest":8080}]`
					}
					inbound := &model.Inbound{Protocol: model.VLESS, Port: 32110, Settings: `{"clients":[],"decryption":"mlkem768x25519plus.native.600s.` + base64.RawURLEncoding.EncodeToString(make([]byte, 32)) + `","fallbacks":` + raw + `}`, StreamSettings: `{"network":"tcp","security":"none"}`}
					db := database.GetDB()
					if err := db.Where("1 = 1").Delete(&model.Inbound{}).Error; err != nil {
						t.Fatal(err)
					}
					path := "/panel/api/inbounds/" + route
					if route == "update" {
						stored := *inbound
						stored.Settings = `{"clients":[],"decryption":"none"}`
						if err := db.Create(&stored).Error; err != nil {
							t.Fatal(err)
						}
						inbound.Id = stored.Id
						path += "/" + strconv.Itoa(stored.Id)
					}
					form := url.Values{"protocol": {"vless"}, "port": {"32110"}, "settings": {inbound.Settings}, "streamSettings": {inbound.StreamSettings}}
					if route == "import" {
						data, _ := json.Marshal(inbound)
						form = url.Values{"data": {string(data)}}
					}
					request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
					request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					request.Header.Set("Authorization", "Bearer "+scope)
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					var result struct {
						Success bool   `json:"success"`
						Msg     string `json:"msg"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.Success == invalid {
						t.Fatalf("unexpected response: %s", response.Body)
					}
					var rows []model.Inbound
					if err := db.Find(&rows).Error; err != nil {
						t.Fatal(err)
					}
					if invalid {
						if !strings.Contains(strings.ToLower(result.Msg), "fallback") {
							t.Fatalf("missing error: %s", response.Body)
						}
						if route == "update" {
							if len(rows) != 1 || rows[0].Settings != `{"clients":[],"decryption":"none"}` {
								t.Fatalf("failed update persisted: %+v", rows)
							}
						} else if len(rows) != 0 {
							t.Fatal("failed add/import persisted")
						}
						return
					}
					if len(rows) != 1 {
						t.Fatalf("saved rows: %d", len(rows))
					}
					var core conf.VLessInboundConfig
					if err := json.Unmarshal([]byte(rows[0].Settings), &core); err != nil {
						t.Fatal(err)
					}
					if _, err := core.Build(); err != nil {
						t.Fatalf("saved API config rejected by Xray: %v", err)
					}
				})
			}
		}
	}
}
