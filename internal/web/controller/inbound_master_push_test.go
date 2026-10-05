package controller

import (
	"context"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// A node enrolled with an admin-scope token (the -getApiToken default) must
// still store the clients its master pushes; it used to keep its own list.
func TestMasterPushWithAdminTokenAppliesClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	prev := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(prev) })

	const token = "admin-node-token"
	if err := database.GetDB().Create(&model.ApiToken{
		Name: "node", Token: crypto.HashTokenSHA256(token), Enabled: true, Scope: model.ApiScopeAdmin,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	var owner model.User
	if err := database.GetDB().First(&owner).Error; err != nil {
		t.Fatalf("load panel user: %v", err)
	}
	const stream = `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`
	stored := &model.Inbound{
		UserId: owner.Id, Tag: "in-46001", Protocol: model.VLESS, Port: 46001, Enable: true,
		Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: stream, Sniffing: `{}`,
	}
	if err := database.GetDB().Create(stored).Error; err != nil {
		t.Fatalf("seed node inbound: %v", err)
	}

	engine := gin.New()
	a := &APIController{}
	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth, a.enforceTokenScope)
	NewInboundController(api.Group("/inbounds"))
	srv := httptest.NewServer(engine)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	master := runtime.NewRemote(&model.Node{
		Id: 1, Name: "n1", Scheme: "http", Address: u.Hostname(), Port: port,
		BasePath: "/", ApiToken: token, Enable: true, AllowPrivateAddress: true,
	}, nil)

	pushed := *stored
	pushed.Settings = `{"clients":[{"id":"7fa0b7d1-9b5f-47ad-bef2-6cb0c4a624be","email":"alice","enable":true,"subId":"s-alice"}],"decryption":"none"}`
	if err := master.UpdateInbound(context.Background(), &pushed, &pushed); err != nil {
		t.Fatalf("master push: %v", err)
	}

	var got model.Inbound
	if err := database.GetDB().First(&got, stored.Id).Error; err != nil {
		t.Fatalf("reload node inbound: %v", err)
	}
	if !strings.Contains(got.Settings, `"alice"`) {
		t.Fatalf("node kept its own client list after a master push: %s", got.Settings)
	}
}
