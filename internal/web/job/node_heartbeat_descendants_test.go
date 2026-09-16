package job

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func transitiveGuids(t *testing.T) []string {
	t.Helper()
	tree, err := (&service.NodeService{}).GetNodeTree()
	if err != nil {
		t.Fatalf("GetNodeTree: %v", err)
	}
	var out []string
	for _, n := range tree {
		if n.Transitive {
			out = append(out, n.Guid)
		}
	}
	return out
}

// The heartbeat skips a disabled node and never sees a deleted one, so the
// sub-nodes it had learned from them stayed on the Nodes page for good.
func TestHeartbeatDropsSubNodesOfNodesItNoLongerProbes(t *testing.T) {
	cases := []struct {
		name   string
		retire func(t *testing.T, nodeID int)
	}{
		{"disabled", func(t *testing.T, nodeID int) {
			if err := (&service.NodeService{}).SetEnable(nodeID, false); err != nil {
				t.Fatalf("SetEnable: %v", err)
			}
		}},
		{"deleted", func(t *testing.T, nodeID int) {
			if err := (&service.NodeService{}).Delete(nodeID); err != nil {
				t.Fatalf("Delete: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			xuilogger.InitLogger(logging.ERROR)
			if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
				t.Fatalf("InitDB: %v", err)
			}
			t.Cleanup(func() { _ = database.CloseDB() })
			runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
			t.Cleanup(func() { runtime.SetManager(nil) })

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "server/status"):
					_, _ = w.Write([]byte(`{"success":true,"obj":{"panelGuid":"direct-guid","xray":{"state":"running"}}}`))
				case strings.HasSuffix(r.URL.Path, "server/descendants"):
					_, _ = w.Write([]byte(`{"success":true,"obj":[{"guid":"sub-guid","parentGuid":"direct-guid","name":"sub","status":"online"}]}`))
				default:
					_, _ = w.Write([]byte(`{"success":true}`))
				}
			}))
			t.Cleanup(srv.Close)
			host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
			portNum, _ := strconv.Atoi(port)
			node := &model.Node{
				Name: "direct", Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
				Enable: true, Status: "unknown", AllowPrivateAddress: true, TlsVerifyMode: "verify",
			}
			if err := database.GetDB().Create(node).Error; err != nil {
				t.Fatalf("create node: %v", err)
			}

			hb := NewNodeHeartbeatJob()
			hb.Run()
			if got := transitiveGuids(t); len(got) != 1 || got[0] != "sub-guid" {
				t.Fatalf("sub-nodes after first heartbeat = %v, want [sub-guid]", got)
			}

			tc.retire(t, node.Id)
			hb.Run()
			if got := transitiveGuids(t); len(got) != 0 {
				t.Fatalf("sub-nodes after the node was %s = %v, want none", tc.name, got)
			}
		})
	}
}
