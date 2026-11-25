package controller

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
)

func TestPanelRestartScopeAndDelayedHook(t *testing.T) {
	engine, auth := newAPIAuthTestEngine(t)
	NewSettingController(engine.Group("/panel/api", auth.checkAPIAuth, auth.enforceTokenScope))
	restarts := make(chan time.Time, 1)
	global.SetRestartHook(func() { restarts <- time.Now() })
	t.Cleanup(func() { global.SetRestartHook(func() {}) })
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeAdmin, model.ApiScopeNodeSync, model.ApiScopeMonitor} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
		for _, mtls := range []bool{false, true} {
			t.Run(scope+map[bool]string{false: "/bearer", true: "/mtls"}[mtls], func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/panel/api/setting/restartPanel", nil)
				req.Header.Set("Authorization", "Bearer "+scope)
				if mtls {
					req.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}
				}
				w := httptest.NewRecorder()
				start := time.Now()
				engine.ServeHTTP(w, req)
				allowed := scope == model.ApiScopeNodeAdmin || (scope == model.ApiScopeAdmin && !mtls)
				if !allowed {
					want := `{"msg":"this API token is not permitted to access this endpoint","success":false}`
					if w.Code != http.StatusForbidden || w.Body.String() != want {
						t.Fatalf("denied = %d %s", w.Code, w.Body.String())
					}
					return
				}
				var result struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Success || w.Code != http.StatusOK {
					t.Fatalf("accepted = %d %s: %v", w.Code, w.Body.String(), err)
				}
				select {
				case <-restarts:
					t.Fatal("panel restarted before response was delivered")
				default:
				}
				select {
				case at := <-restarts:
					if at.Sub(start) < 3*time.Second {
						t.Fatalf("restart delay = %s, want >=3s", at.Sub(start))
					}
				case <-time.After(6 * time.Second):
					t.Fatal("accepted request did not restart panel")
				}
			})
		}
	}
}
