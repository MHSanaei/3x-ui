package job

import (
	"encoding/json"
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
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// resetGate holds every node reset open until released, counting how many nodes
// the job reaches at once.
type resetGate struct {
	entered atomic.Int32
	release chan struct{}
	once    sync.Once
}

func (g *resetGate) open() { g.once.Do(func() { close(g.release) }) }

func (g *resetGate) waitAll(t *testing.T, want int32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for g.entered.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("periodic reset reached %d of %d hanging nodes, want all of them at once", g.entered.Load(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// resetNode is a node whose every traffic reset hangs until the gate opens.
func resetNode(t *testing.T, gate *resetGate, name string) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if strings.Contains(r.URL.Path, "resetTraffic") {
			gate.entered.Add(1)
			select {
			case <-r.Context().Done():
			case <-gate.release:
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	portNum, _ := strconv.Atoi(port)
	node := &model.Node{
		Name: name, Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
		Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.Id
}

func runResetJobAgainstGate(t *testing.T, gate *resetGate, want int32) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewPeriodicTrafficResetJob("daily", time.UTC).Run()
	}()
	t.Cleanup(func() { gate.open(); <-done })
	gate.waitAll(t, want)
}

func newResetFleet(t *testing.T) *resetGate {
	t.Helper()
	initResetJobDB(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })
	return &resetGate{release: make(chan struct{})}
}

// The job reset due clients and inbounds one by one, each waiting on its node,
// so a few hanging nodes stretched one run across hours.
func TestPeriodicResetReachesClientNodesConcurrently(t *testing.T) {
	gate := newResetFleet(t)
	db := database.GetDB()
	for i := range 3 {
		nodeID := resetNode(t, gate, fmt.Sprintf("client-node-%d", i))
		email := fmt.Sprintf("cycle-%d@node", i)
		client := model.Client{Email: email, ID: fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", i), Enable: true, TrafficReset: "daily"}
		settings, _ := json.Marshal(map[string]any{"clients": []model.Client{client}})
		ib := model.Inbound{
			UserId: 1, Enable: true, Port: 47000 + i, Protocol: model.VLESS, NodeID: &nodeID,
			Tag: "reset-client-" + strconv.Itoa(i), TrafficReset: "never", Settings: string(settings),
		}
		if err := db.Create(&ib).Error; err != nil {
			t.Fatalf("create inbound: %v", err)
		}
		rec := model.ClientRecord{Email: email, UUID: client.ID, Enable: true, TrafficReset: "daily"}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("create client record: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("link client: %v", err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: email, Enable: true, Up: 500, Down: 700}).Error; err != nil {
			t.Fatalf("create traffic: %v", err)
		}
	}
	runResetJobAgainstGate(t, gate, 3)
}

func TestPeriodicResetReachesInboundNodesConcurrently(t *testing.T) {
	gate := newResetFleet(t)
	for i := range 3 {
		nodeID := resetNode(t, gate, fmt.Sprintf("inbound-node-%d", i))
		ib := model.Inbound{
			UserId: 1, Enable: true, Port: 47100 + i, Protocol: model.VLESS, NodeID: &nodeID,
			Tag: "reset-inbound-" + strconv.Itoa(i), TrafficReset: "daily", Settings: `{"clients":[]}`,
		}
		if err := database.GetDB().Create(&ib).Error; err != nil {
			t.Fatalf("create inbound: %v", err)
		}
	}
	runResetJobAgainstGate(t, gate, 3)
}
