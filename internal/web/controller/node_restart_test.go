package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestNodeRestartTargetsOnlySelectedNode(t *testing.T) {
	for _, action := range []struct{ name, remotePath string }{
		{"restartXray", "/custom/panel/api/server/restartXrayService"},
		{"restartPanel", "/custom/panel/api/setting/restartPanel"},
	} {
		t.Run(action.name, func(t *testing.T) {
			engine, auth := newAPIAuthTestEngine(t)
			group := engine.Group("/panel/api/nodes", auth.checkAPIAuth, auth.enforceTokenScope)
			NewNodeController(group)
			if err := database.GetDB().Create(&model.ApiToken{Name: "master", Token: crypto.HashTokenSHA256("master"), Enabled: true, Scope: model.ApiScopeAdmin}).Error; err != nil {
				t.Fatal(err)
			}
			previous := runtime.GetManager()
			runtime.SetManager(runtime.NewManager(runtime.LocalDeps{}))
			t.Cleanup(func() { runtime.SetManager(previous) })
			var calls atomic.Int32
			var response atomic.Value
			response.Store(`{"success":true,"msg":"accepted","obj":null}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != action.remotePath || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer node-secret" {
					t.Errorf("remote request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
				}
				fmt.Fprint(w, response.Load().(string))
			}))
			t.Cleanup(server.Close)
			u, _ := url.Parse(server.URL)
			port, _ := strconv.Atoi(u.Port())
			node := model.Node{Name: "selected", Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/custom/", ApiToken: "node-secret", Enable: true, Status: "offline", AllowPrivateAddress: true}
			if err := database.GetDB().Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			other := node
			other.Id = 0
			other.Name = "untouched"
			other.Port = 1
			if err := database.GetDB().Create(&other).Error; err != nil {
				t.Fatal(err)
			}
			request := func(id string, ctx context.Context) (bool, string) {
				t.Helper()
				r := httptest.NewRequest(http.MethodPost, "/panel/api/nodes/"+action.name+"/"+id, nil).WithContext(ctx)
				r.Header.Set("Authorization", "Bearer master")
				w := httptest.NewRecorder()
				engine.ServeHTTP(w, r)
				var result struct {
					Success bool   `json:"success"`
					Msg     string `json:"msg"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode response: status=%d body=%q: %v", w.Code, w.Body.String(), err)
				}
				return result.Success, result.Msg
			}
			id := strconv.Itoa(node.Id)
			if ok, msg := request(id, context.Background()); !ok || calls.Load() != 1 {
				t.Fatalf("restart = %v %q, remote calls=%d, want success and one call", ok, msg, calls.Load())
			}
			response.Store(`{"success":false,"msg":"maintenance lock","obj":null}`)
			if ok, msg := request(id, context.Background()); ok || !strings.Contains(msg, "maintenance lock") {
				t.Fatalf("node rejection = %v %q", ok, msg)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if ok, msg := request(id, ctx); ok || !strings.Contains(msg, "context canceled") {
				t.Fatalf("canceled restart = %v %q", ok, msg)
			}
			deadlineCtx, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer deadlineCancel()
			if ok, msg := request(id, deadlineCtx); ok || !strings.Contains(msg, "context deadline exceeded") {
				t.Fatalf("expired restart = %v %q", ok, msg)
			}
			before := calls.Load()
			if err := database.GetDB().Model(&node).Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
			for _, bad := range []struct{ id, message string }{{id, "node is disabled"}, {"0", "invalid node ID"}, {"-1", "invalid node ID"}, {"abc", "invalid node ID"}, {"999999", "node not found"}} {
				if ok, msg := request(bad.id, context.Background()); ok || !strings.Contains(msg, bad.message) {
					t.Fatalf("restart %s = %v %q, want %q", bad.id, ok, msg, bad.message)
				}
			}
			if calls.Load() != before {
				t.Fatalf("invalid requests reached node: calls %d -> %d", before, calls.Load())
			}
		})
	}
}
