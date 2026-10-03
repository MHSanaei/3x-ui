package job

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// A reset the node missed is replayed by the next sync, ahead of the snapshot
// fetch so the merge already sees the zeroed counters.
func TestNodeTrafficSyncReplaysOwedResetBeforeSnapshot(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	service.StartTrafficWriter()
	t.Cleanup(service.StopTrafficWriter)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		switch {
		case strings.Contains(r.URL.Path, "clients/resetTraffic/"):
			calls = append(calls, "reset:"+r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
		case strings.HasSuffix(r.URL.Path, "inbounds/list"):
			calls = append(calls, "snapshot")
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "inbounds/list") {
			_, _ = w.Write([]byte(`{"success":true,"obj":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	portNum, _ := strconv.Atoi(port)
	node := &model.Node{
		Name: "owes-reset", Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
		Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := database.GetDB().Create(&model.NodePendingReset{NodeId: node.Id, Email: "owed@node", QueuedAt: 1}).Error; err != nil {
		t.Fatalf("seed pending reset: %v", err)
	}

	NewNodeTrafficSyncJob().Run()

	mu.Lock()
	got := slices.Clone(calls)
	mu.Unlock()
	if len(got) < 2 || got[0] != "reset:owed@node" || !slices.Contains(got, "snapshot") {
		t.Fatalf("node calls %v, want the owed reset first, then the snapshot", got)
	}
	var left int64
	if err := database.GetDB().Model(&model.NodePendingReset{}).Count(&left).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if left != 0 {
		t.Fatalf("replayed reset still queued (%d rows)", left)
	}
}
