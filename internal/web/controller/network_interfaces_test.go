package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestNetworkInterfacesRequireAdminAndUseExistingEnvelope(t *testing.T) {
	engine, api := newAPIAuthTestEngine(t)
	controller := &ServerController{}
	group := engine.Group("/panel/api/server", api.checkAPIAuth, api.enforceTokenScope)
	group.GET("/networkInterfaces", controller.getNetworkInterfaces)
	for _, tc := range []struct {
		name, scope string
		want        int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"monitor", model.ApiScopeMonitor, http.StatusForbidden},
		{"node sync", model.ApiScopeNodeSync, http.StatusForbidden},
		{"admin", model.ApiScopeAdmin, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/panel/api/server/networkInterfaces", nil)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			if tc.scope != "" {
				token := "network-test-" + tc.name
				if err := database.GetDB().Create(&model.ApiToken{Name: tc.name, Token: crypto.HashTokenSHA256(token), Scope: tc.scope, Enabled: true}).Error; err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
			if tc.want == http.StatusOK {
				var response struct {
					Success bool `json:"success"`
					Obj     struct {
						Interfaces     []map[string]any `json:"interfaces"`
						Automatic      string           `json:"automatic"`
						AutomaticError string           `json:"automaticError"`
					} `json:"obj"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if !response.Success || response.Obj.Interfaces == nil {
					t.Fatalf("response=%s", w.Body.String())
				}
				for _, iface := range response.Obj.Interfaces {
					if len(iface) != 2 || iface["name"] == "" {
						t.Fatalf("unexpected interface metadata: %v", iface)
					}
				}
			}
		})
	}
}
