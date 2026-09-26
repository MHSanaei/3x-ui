package job

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// goingDownNodes seeds n online nodes whose address refuses connections, so the
// next heartbeat flips every one of them to offline in the same tick.
func goingDownNodes(t *testing.T, n int) {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })
	srv := httptest.NewServer(nil)
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	portNum, _ := strconv.Atoi(port)
	srv.Close()
	for i := range n {
		node := &model.Node{
			Name: fmt.Sprintf("node-%02d", i), Scheme: "http", Address: host, Port: portNum, BasePath: "/",
			ApiToken: "tok", Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
		}
		if err := database.GetDB().Create(node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}
}

func collectNodeEvents(t *testing.T) func() []eventbus.Event {
	t.Helper()
	bus := eventbus.New(eventbus.DefaultBufferSize)
	var mu sync.Mutex
	var got []eventbus.Event
	bus.Subscribe("test", func(e eventbus.Event) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})
	prev := EventBus
	EventBus = bus
	t.Cleanup(func() {
		EventBus = prev
		bus.Stop()
	})
	return func() []eventbus.Event {
		time.Sleep(300 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return append([]eventbus.Event(nil), got...)
	}
}

// A master-side blip flipped every node in one tick and published one event per
// node, overflowing the notifier queues and every chat's rate limit.
func TestHeartbeatSummarizesNodeDownBurst(t *testing.T) {
	goingDownNodes(t, 12)
	events := collectNodeEvents(t)

	NewNodeHeartbeatJob().Run()

	got := events()
	if len(got) != 1 {
		t.Fatalf("heartbeat published %d events for 12 nodes going down, want 1 summary", len(got))
	}
	want := "node-00, node-01, node-02, node-03, node-04, node-05, node-06, node-07, node-08, node-09 (+2)"
	if got[0].Type != eventbus.EventNodeDown || got[0].Source != want {
		t.Fatalf("summary event = %s %q, want %s %q", got[0].Type, got[0].Source, eventbus.EventNodeDown, want)
	}
}

func TestHeartbeatKeepsSingleNodeDownEvent(t *testing.T) {
	goingDownNodes(t, 1)
	events := collectNodeEvents(t)

	NewNodeHeartbeatJob().Run()

	got := events()
	if len(got) != 1 || got[0].Source != "node-00" {
		t.Fatalf("events = %+v, want the node's own node.down", got)
	}
	if _, ok := got[0].Data.(*eventbus.NodeHealthData); !ok {
		t.Fatalf("single node.down lost its health data: %#v", got[0].Data)
	}
}
