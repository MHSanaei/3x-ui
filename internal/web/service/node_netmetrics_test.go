package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestProbeParsesNetIO(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":{"cpu":5,"mem":{"current":1,"total":2},"netIO":{"up":1000,"down":2000},"panelGuid":"g","uptime":42}}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	n := &model.Node{Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/", ApiToken: "t", AllowPrivateAddress: true}

	patch, err := (&NodeService{}).probe(context.Background(), n, "")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if patch.NetUp != 1000 || patch.NetDown != 2000 {
		t.Fatalf("net throughput not parsed from status: up=%d down=%d", patch.NetUp, patch.NetDown)
	}
}

func TestUpdateHeartbeatStoresNetMetrics(t *testing.T) {
	_ = setupSettingMtlsDB(t)
	s := &NodeService{}

	n := &model.Node{Name: "netn", Address: "1.2.3.4", Port: 2053, Scheme: "https", ApiToken: "t"}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	patch := HeartbeatPatch{Status: "online", LastHeartbeat: time.Now().Unix(), NetUp: 111, NetDown: 222}
	if err := s.UpdateHeartbeat(n.Id, patch); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}

	var got model.Node
	if err := database.GetDB().First(&got, n.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if got.NetUp != 111 || got.NetDown != 222 {
		t.Fatalf("net columns not persisted: up=%d down=%d", got.NetUp, got.NetDown)
	}
	if len(s.AggregateNodeMetric(n.Id, "netUp", 2, 60)) == 0 {
		t.Fatal("expected netUp history points after an online heartbeat")
	}
}

func TestNodeMetricKeysIncludesNet(t *testing.T) {
	for _, k := range []string{"netUp", "netDown"} {
		if !slices.Contains(NodeMetricKeys, k) {
			t.Fatalf("NodeMetricKeys must include %q so the history endpoint accepts it", k)
		}
	}
}
