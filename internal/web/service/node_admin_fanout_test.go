package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// fanoutGate holds every node call open until released, so a test sees how many
// nodes an operation reaches at once.
type fanoutGate struct {
	entered atomic.Int32
	release chan struct{}
	once    sync.Once
}

func newFanoutGate() *fanoutGate { return &fanoutGate{release: make(chan struct{})} }

func (g *fanoutGate) hold(ctx context.Context) error {
	g.entered.Add(1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.release:
		return nil
	}
}

func (g *fanoutGate) open() { g.once.Do(func() { close(g.release) }) }

func (g *fanoutGate) waitAll(t *testing.T, want int32, op string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for g.entered.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("%s reached %d of %d hanging nodes, want all of them at once", op, g.entered.Load(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type gatedNodeRuntime struct {
	fakeNodeRuntime
	gate *fanoutGate
}

func (r *gatedNodeRuntime) ResetAllTraffics(ctx context.Context) error { return r.gate.hold(ctx) }

func (r *gatedNodeRuntime) DelInbound(ctx context.Context, _ *model.Inbound) error {
	return r.gate.hold(ctx)
}

func gatedNodes(t *testing.T, gate *fanoutGate, n int) []int {
	t.Helper()
	mgr := useTestRuntimeManager(t)
	ids := make([]int, 0, n)
	for i := range n {
		node := &model.Node{Name: fmt.Sprintf("fanout-%d", i), Address: "127.0.0.1", Port: 2100 + i, ApiToken: "tok", Enable: true, Status: "online"}
		if err := database.GetDB().Create(node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
		mgr.SetRuntimeOverride(node.Id, &gatedNodeRuntime{gate: gate})
		ids = append(ids, node.Id)
	}
	return ids
}

// Operations that touch every node walked them one at a time, so a few hanging
// nodes kept the request running for minutes past the panel's write timeout.
func TestResetAllTrafficsReachesNodesConcurrently(t *testing.T) {
	setupConflictDB(t)
	gate := newFanoutGate()
	gatedNodes(t, gate, 3)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = (&InboundService{}).ResetAllTraffics()
	}()
	t.Cleanup(func() { gate.open(); <-done })
	gate.waitAll(t, 3, "ResetAllTraffics")
}

func TestDelInboundsPushesNodeDeletesConcurrently(t *testing.T) {
	setupConflictDB(t)
	gate := newFanoutGate()
	var inboundIDs []int
	for i, nodeID := range gatedNodes(t, gate, 3) {
		inboundIDs = append(inboundIDs, nodeInbound(t, nodeID, 46400+i, nil).Id)
	}
	done := make(chan struct{})
	var result BulkDelInboundResult
	var err error
	go func() {
		defer close(done)
		result, _, err = (&InboundService{}).DelInbounds(inboundIDs)
	}()
	t.Cleanup(func() { gate.open(); <-done })
	gate.waitAll(t, 3, "DelInbounds")
	gate.open()
	<-done
	if err != nil || result.Deleted != 3 || len(result.Skipped) != 0 {
		t.Fatalf("DelInbounds = %+v, %v; want 3 deleted", result, err)
	}
}

func TestUpdatePanelsReachesNodesConcurrently(t *testing.T) {
	setupConflictDB(t)
	useTestRuntimeManager(t)
	gate := newFanoutGate()
	var ids []int
	for i := range 3 {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			if strings.HasSuffix(r.URL.Path, "server/updatePanel") {
				_ = gate.hold(r.Context())
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		t.Cleanup(srv.Close)
		host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
		portNum, _ := strconv.Atoi(port)
		node := &model.Node{
			Name: fmt.Sprintf("panel-%d", i), Scheme: "http", Address: host, Port: portNum, BasePath: "/",
			ApiToken: "tok", Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
		}
		if err := database.GetDB().Create(node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
		ids = append(ids, node.Id)
	}
	done := make(chan struct{})
	var results []NodeUpdateResult
	go func() {
		defer close(done)
		results, _ = (&NodeService{}).UpdatePanels(ids, false)
	}()
	t.Cleanup(func() { gate.open(); <-done })
	gate.waitAll(t, 3, "UpdatePanels")
	gate.open()
	<-done
	if len(results) != len(ids) {
		t.Fatalf("UpdatePanels returned %d results for %d nodes", len(results), len(ids))
	}
	for i, res := range results {
		if res.Id != ids[i] || !res.OK {
			t.Fatalf("UpdatePanels result %d = %+v, want node %d updated, in request order", i, res, ids[i])
		}
	}
}
