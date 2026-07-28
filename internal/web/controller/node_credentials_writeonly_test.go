package controller

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
)

func newNodeCredentialTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewNodeController(engine.Group("/panel/api/nodes"))
	return engine
}

func TestNodeControllerResponsesDoNotLeakApiToken(t *testing.T) {
	engine := newNodeCredentialTestEngine(t)
	if err := database.GetDB().Create(&model.Node{
		Name:     "stored-node",
		Scheme:   "https",
		Address:  "example.com",
		Port:     2053,
		BasePath: "/",
		ApiToken: "stored-secret-token",
		Enable:   true,
	}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	for _, path := range []string{"/panel/api/nodes/list", "/panel/api/nodes/get/1"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, w.Code, w.Body.String())
		}
		body := w.Body.String()
		if strings.Contains(body, "stored-secret-token") || strings.Contains(body, "apiToken") {
			t.Fatalf("%s leaked api token: %s", path, body)
		}
		if !strings.Contains(body, `"hasApiToken":true`) {
			t.Fatalf("%s did not expose credential presence: %s", path, body)
		}
	}
}

func TestNodeControllerAddAcceptsTokenButReturnsView(t *testing.T) {
	engine := newNodeCredentialTestEngine(t)

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/server/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer input-secret-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":{"cpu":1,"mem":{"current":1,"total":2},"xray":{"version":"1","state":"running"},"panelVersion":"v3.4.1","panelGuid":"guid","uptime":7,"netIO":{"up":3,"down":4}}}`))
	}))
	defer remote.Close()
	host, portString, err := net.SplitHostPort(strings.TrimPrefix(remote.URL, "http://"))
	if err != nil {
		t.Fatalf("split remote addr: %v", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("parse remote port: %v", err)
	}

	payload := map[string]any{
		"name":                "added-node",
		"scheme":              "http",
		"address":             host,
		"port":                port,
		"basePath":            "/",
		"apiToken":            "input-secret-token",
		"enable":              true,
		"allowPrivateAddress": true,
	}
	raw, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/nodes/add", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "input-secret-token") || strings.Contains(body, "apiToken") {
		t.Fatalf("add response leaked api token: %s", body)
	}
	if !strings.Contains(body, `"hasApiToken":true`) {
		t.Fatalf("add response did not expose credential presence: %s", body)
	}

	var stored model.Node
	if err := database.GetDB().Where("name = ?", "added-node").First(&stored).Error; err != nil {
		t.Fatalf("load stored node: %v", err)
	}
	if stored.ApiToken != "input-secret-token" {
		t.Fatalf("stored token = %q, want input-secret-token", stored.ApiToken)
	}
}

func TestNodeControllerUpdateBlankApiTokenKeepsStoredToken(t *testing.T) {
	engine := newNodeCredentialTestEngine(t)

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/server/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer stored-secret-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":{"cpu":1,"mem":{"current":1,"total":2},"xray":{"version":"1","state":"running"},"panelVersion":"v3.4.1","panelGuid":"guid","uptime":7,"netIO":{"up":3,"down":4}}}`))
	}))
	defer remote.Close()
	host, portString, err := net.SplitHostPort(strings.TrimPrefix(remote.URL, "http://"))
	if err != nil {
		t.Fatalf("split remote addr: %v", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("parse remote port: %v", err)
	}
	node := &model.Node{
		Name:                "stored-node",
		Scheme:              "http",
		Address:             host,
		Port:                port,
		BasePath:            "/",
		ApiToken:            "stored-secret-token",
		Enable:              true,
		AllowPrivateAddress: true,
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	payload := map[string]any{
		"name":                "stored-node-renamed",
		"scheme":              "http",
		"address":             host,
		"port":                port,
		"basePath":            "/",
		"apiToken":            "",
		"enable":              true,
		"allowPrivateAddress": true,
	}
	raw, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/nodes/update/"+strconv.Itoa(node.Id), strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", w.Code, w.Body.String())
	}

	var stored model.Node
	if err := database.GetDB().Where("id = ?", node.Id).First(&stored).Error; err != nil {
		t.Fatalf("load stored node: %v", err)
	}
	if stored.ApiToken != "stored-secret-token" {
		t.Fatalf("blank update changed token to %q", stored.ApiToken)
	}
	if stored.Name != "stored-node-renamed" {
		t.Fatalf("stored name = %q, want stored-node-renamed", stored.Name)
	}
}
