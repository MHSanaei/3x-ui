package job

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func switchableNode(t *testing.T, status string) *atomic.Bool {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	var up atomic.Bool
	up.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "server/status") {
			_, _ = w.Write([]byte(`{"success":true,"obj":{"panelGuid":"flaky-guid","xray":{"state":"running"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"obj":[]}`))
	}))
	t.Cleanup(srv.Close)
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	portNum, _ := strconv.Atoi(port)
	node := &model.Node{
		Name: "flaky", Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
		Enable: true, Status: status, AllowPrivateAddress: true, TlsVerifyMode: "verify",
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return &up
}

func TestHeartbeatHoldsNodeDownUntilThreshold(t *testing.T) {
	cases := []struct {
		name   string
		status string
		probes []bool
		want   []string
	}{
		{
			name:   "flapping below threshold",
			status: "online",
			probes: []bool{false, true, false, false, true, false, true},
		},
		{
			name:   "outage reaching threshold",
			status: "online",
			probes: []bool{false, false, false, false, true},
			want:   []string{"probe 3: node.down", "probe 5: node.up"},
		},
		{
			name:   "offline before restart",
			status: "offline",
			probes: []bool{true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := switchableNode(t, tc.status)
			if err := (&service.SettingService{}).SetNodeDownThreshold(3); err != nil {
				t.Fatalf("SetNodeDownThreshold: %v", err)
			}
			events := collectNodeEvents(t)
			job := NewNodeHeartbeatJob()

			var got []string
			for i, probeOK := range tc.probes {
				up.Store(probeOK)
				job.Run()
				for _, e := range events()[len(got):] {
					got = append(got, fmt.Sprintf("probe %d: %s", i+1, e.Type))
				}
			}

			if strings.Join(got, ", ") != strings.Join(tc.want, ", ") {
				t.Fatalf("notifications = %q, want %q", got, tc.want)
			}
		})
	}
}
