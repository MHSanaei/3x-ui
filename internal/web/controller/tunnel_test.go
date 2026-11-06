package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestTunnelRoutesScopeAndIdempotentSync(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	old := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{}))
	t.Cleanup(func() { runtime.SetManager(old); wattunnel.GetManager().StopAll() })
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeMonitor, model.ApiScopeNodeSync} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Scope: scope, Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	engine := gin.New()
	a := &APIController{}
	group := engine.Group("/panel/api", a.checkAPIAuth, a.enforceTokenScope)
	NewTunnelController(group.Group("/tunnels"))
	call := func(scope, path string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/panel/api/tunnels/"+path, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+scope)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"add", "export/1", "start/1", "stop/1", "del/1"} {
		w := call(model.ApiScopeNodeSync, path, []byte(`{}`))
		if w.Code != http.StatusForbidden {
			t.Fatalf("node-sync reached %s: %d", path, w.Code)
		}
	}
	spec := wattunnel.Spec{GUID: uuid.NewString(), Owner: uuid.NewString(), Revision: 1, Name: "remote", Secret: "secret-must-not-leak", Config: wattunnel.Config{Role: "server", Carrier: "tcp", Endpoint: "192.0.2.1", Port: 4443, Ports: []wattunnel.Port{{Listen: 443, Target: 443, Protocol: "tcp"}}}}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		w := call(model.ApiScopeNodeSync, "sync", raw)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"success":true`) {
			t.Fatalf("sync: %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), spec.Secret) {
			t.Fatal("sync response leaked secret")
		}
	}
	var count int64
	database.GetDB().Model(&model.ManagedTunnel{}).Count(&count)
	if count != 1 {
		t.Fatalf("retry duplicated row: %d", count)
	}
	w := call(model.ApiScopeMonitor, "sync", raw)
	if w.Code != http.StatusForbidden {
		t.Fatalf("monitor mutated tunnel: %d", w.Code)
	}
}
