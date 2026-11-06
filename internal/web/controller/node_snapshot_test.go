package controller

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestNodeTrafficSnapshotScopeAndRevision(t *testing.T) {
	router, api := newAPIAuthTestEngine(t)
	NewInboundController(router.Group("/base/panel/api/inbounds", api.checkAPIAuth, api.enforceTokenScope))
	mtls := httptest.NewRequest(http.MethodGet, "/base/panel/api/inbounds/trafficSnapshot", nil)
	mtls.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}
	mtlsResult := httptest.NewRecorder()
	router.ServeHTTP(mtlsResult, mtls)
	if mtlsResult.Code != http.StatusOK {
		t.Fatalf("verified mTLS snapshot rejected: %d %s", mtlsResult.Code, mtlsResult.Body)
	}
	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/base/panel/api/inbounds/trafficSnapshot", nil))
	if unauthenticated.Code != http.StatusNotFound {
		t.Fatalf("unauthenticated snapshot exposed: %d", unauthenticated.Code)
	}

	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeSync, model.ApiScopeNodeAdmin, model.ApiScopeMonitor} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/base/panel/api/inbounds/trafficSnapshot", nil)
		request.Header.Set("Authorization", "Bearer "+scope)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if scope == model.ApiScopeMonitor {
			if response.Code != http.StatusForbidden {
				t.Fatalf("monitor status=%d", response.Code)
			}
			continue
		}
		if response.Code != http.StatusOK {
			t.Fatalf("scope %s rejected: %d %s", scope, response.Code, response.Body)
		}
		var env struct {
			Success bool `json:"success"`
			Obj     struct {
				Version   int             `json:"version"`
				Revision  string          `json:"revision"`
				Structure json.RawMessage `json:"structure"`
			} `json:"obj"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Success || env.Obj.Version != 1 || len(env.Obj.Revision) != 64 || len(env.Obj.Structure) == 0 {
			t.Fatalf("invalid first snapshot: %s", response.Body)
		}
	}
}
