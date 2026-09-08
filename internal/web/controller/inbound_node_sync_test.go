package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// A sub-node stores whatever the master pushes. A master row whose certificate
// predates the TLS guard must still land, or the node silently falls out of sync.
func TestNodeSyncPushSkipsOperatorTLSGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	prev := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(prev) })

	for name, scope := range map[string]string{"node-sync": model.ApiScopeNodeSync, "admin": model.ApiScopeAdmin} {
		row := &model.ApiToken{Name: name, Token: crypto.HashTokenSHA256(name + "-token"), Enabled: true, Scope: scope}
		if err := database.GetDB().Create(row).Error; err != nil {
			t.Fatalf("seed %s token: %v", name, err)
		}
	}

	engine := gin.New()
	a := &APIController{}
	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth, a.enforceTokenScope)
	NewInboundController(api.Group("/inbounds"))

	const legacyStream = `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"","keyFile":"","certificate":[],"key":[]}]}}`
	add := func(t *testing.T, token string, port int) string {
		t.Helper()
		form := url.Values{
			"protocol":       {"vless"},
			"port":           {strconv.Itoa(port)},
			"tag":            {"tls-legacy-" + strconv.Itoa(port)},
			"enable":         {"true"},
			"settings":       {`{"clients":[]}`},
			"streamSettings": {legacyStream},
		}
		req := httptest.NewRequest(http.MethodPost, "/panel/api/inbounds/add", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w.Body.String()
	}
	rows := func(t *testing.T, tag string) int64 {
		t.Helper()
		var n int64
		if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", tag).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", tag, err)
		}
		return n
	}

	t.Run("a master push lands on the node", func(t *testing.T) {
		body := add(t, "node-sync-token", 45001)
		if !strings.Contains(body, `"success":true`) {
			t.Fatalf("node-sync add rejected: %s", body)
		}
		if got := rows(t, "tls-legacy-45001"); got != 1 {
			t.Fatalf("stored rows = %d, want 1", got)
		}
	})

	t.Run("an operator token is still held to the guard", func(t *testing.T) {
		body := add(t, "admin-token", 45002)
		if !strings.Contains(body, `"success":false`) || !strings.Contains(body, "TLS") {
			t.Fatalf("admin add should fail on TLS, got: %s", body)
		}
		if got := rows(t, "tls-legacy-45002"); got != 0 {
			t.Fatalf("stored rows = %d, want 0", got)
		}
	})
}
