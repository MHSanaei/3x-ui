package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientGetBySubIDHTTP(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()
	clients := []model.ClientRecord{
		{Email: "alice@example.com", SubID: "shared_%", UUID: "alice-uuid", Enable: true},
		{Email: "bob@example.com", SubID: "shared_%", UUID: "bob-uuid", ExpiryTime: 1},
		{Email: "carol@example.com", SubID: "solo", Enable: true},
		{Email: "shared_%", SubID: "shared_extra", Enable: true},
	}
	for i := range clients {
		if err := db.Create(&clients[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&clients[1]).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{Tag: "lookup", Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: clients[0].Id, InboundId: inbound.Id, FlowOverride: "xtls-rprx-vision"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: clients[0].Email, Up: 111, Down: 222}).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewClientController(router.Group("/panel/api/clients"))

	for _, tt := range []struct {
		name   string
		subID  string
		emails []string
	}{
		{"shared subscription includes disabled orphan", "shared_%", []string{"alice@example.com", "bob@example.com"}},
		{"single match", "solo", []string{"carol@example.com"}},
		{"missing subscription", "missing", nil},
		{"partial subscription is not a match", "shared", nil},
		{"wildcard is not a match", "%", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panel/api/clients/get/subId/"+url.PathEscape(tt.subID), nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			var response struct {
				Success bool   `json:"success"`
				Msg     string `json:"msg"`
				Obj     []struct {
					Client      model.ClientRecord `json:"client"`
					InboundIDs  []int              `json:"inboundIds"`
					UsedTraffic int64              `json:"usedTraffic"`
				} `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if tt.emails == nil {
				if response.Success || !strings.Contains(response.Msg, gorm.ErrRecordNotFound.Error()) || response.Obj != nil {
					t.Fatalf("want record-not-found envelope, got %s", w.Body.String())
				}
				return
			}
			if !response.Success || len(response.Obj) != len(tt.emails) {
				t.Fatalf("want %d client payloads, got %s", len(tt.emails), w.Body.String())
			}
			for i, payload := range response.Obj {
				if payload.Client.Email != tt.emails[i] || payload.Client.SubID != tt.subID {
					t.Fatalf("client %d = %+v, want email %q and subId %q", i, payload.Client, tt.emails[i], tt.subID)
				}
				switch payload.Client.Email {
				case "alice@example.com":
					if payload.Client.UUID != "alice-uuid" || payload.Client.Flow != "xtls-rprx-vision" || payload.UsedTraffic != 333 || !reflect.DeepEqual(payload.InboundIDs, []int{inbound.Id}) {
						t.Fatalf("attached client payload = %+v", payload)
					}
				case "bob@example.com":
					if payload.Client.Enable || payload.Client.ExpiryTime != 1 || payload.Client.UUID != "bob-uuid" || len(payload.InboundIDs) != 0 {
						t.Fatalf("disabled orphan payload = %+v", payload)
					}
				}
			}
		})
	}
}

func TestClientGetBySubIDRequiresAdmin(t *testing.T) {
	router, api := newAPIAuthTestEngine(t)
	NewClientController(router.Group("/base/panel/api/clients", api.checkAPIAuth, api.enforceTokenScope))
	if err := database.GetDB().Create(&model.ClientRecord{Email: "private@example.com", SubID: "private-sub", UUID: "private-uuid"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeMonitor, model.ApiScopeNodeSync, model.ApiScopeNodeAdmin} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		scope string
		code  int
	}{
		{"", http.StatusUnauthorized},
		{model.ApiScopeMonitor, http.StatusForbidden},
		{model.ApiScopeNodeSync, http.StatusForbidden},
		{model.ApiScopeNodeAdmin, http.StatusForbidden},
		{model.ApiScopeAdmin, http.StatusOK},
	} {
		t.Run("scope="+tt.scope, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/base/panel/api/clients/get/subId/private-sub", nil)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			if tt.scope != "" {
				req.Header.Set("Authorization", "Bearer "+tt.scope)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.code {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.code, w.Body.String())
			}
			hasCredentials := strings.Contains(w.Body.String(), "private-uuid")
			if hasCredentials != (tt.code == http.StatusOK) {
				t.Fatalf("unexpected credential access: %s", w.Body.String())
			}
		})
	}
}
