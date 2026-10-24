package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func reviewNodeFixture(t *testing.T, body string) (*NodeService, *model.Node, func(string)) {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "node.db"))
	var response atomic.Value
	response.Store(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response.Load().(string)))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{Name: "sample", Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/", Enable: true, AllowPrivateAddress: true, TlsVerifyMode: "skip"}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatal(err)
	}
	old := nodeMetrics
	nodeMetrics = newMetricHistory()
	t.Cleanup(func() { nodeMetrics = old })
	return &NodeService{}, n, func(s string) { response.Store(s) }
}

func reviewAvailability(t *testing.T, value any) []string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Unavailable []string `json:"unavailableMetrics"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out.Unavailable
}

const (
	reviewUnavailableStatus = `{"success":true,"obj":{"cpu":0,"uptime":0,"unavailableMetrics":["cpu","uptime"],"mem":{"current":1,"total":2},"xray":{"state":"running"}}}`
	reviewZeroStatus        = `{"success":true,"obj":{"cpu":0,"uptime":0,"mem":{"current":1,"total":2},"xray":{"state":"running"}}}`
)

func TestReviewNodeProbePreservesMetricAvailability(t *testing.T) {
	svc, n, _ := reviewNodeFixture(t, reviewUnavailableStatus)
	patch, err := svc.Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if got := reviewAvailability(t, patch.ToUI(true)); !reflect.DeepEqual(got, []string{"cpu", "uptime"}) {
		t.Fatalf("probe availability=%v, want cpu+uptime unavailable", got)
	}
}

func TestReviewNodeUnavailableCPUDoesNotBecomeHistoryZero(t *testing.T) {
	svc, n, setBody := reviewNodeFixture(t, reviewUnavailableStatus)
	patch, err := svc.Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	patch.Status = "online"
	if err := svc.UpdateHeartbeat(n.Id, patch); err != nil {
		t.Fatal(err)
	}
	if points := nodeMetrics.aggregate(nodeMetricKey(n.Id, "cpu"), 2, 60); len(points) != 0 {
		t.Fatalf("unavailable CPU became history: %v", points)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got := reviewAvailability(t, toNodeView(&stored)); !reflect.DeepEqual(got, []string{"cpu", "uptime"}) {
		t.Fatalf("stored availability=%v", got)
	}
	if points := nodeMetrics.aggregate(nodeMetricKey(n.Id, "mem"), 2, 60); len(points) != 1 || points[0]["v"] != float64(50) {
		t.Fatalf("memory history changed: %v", points)
	}
	setBody(reviewZeroStatus)
	patch, err = svc.Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	patch.Status = "online"
	if err := svc.UpdateHeartbeat(n.Id, patch); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got := reviewAvailability(t, toNodeView(&stored)); len(got) != 0 {
		t.Fatalf("valid zero did not recover: %v", got)
	}
	if points := nodeMetrics.aggregate(nodeMetricKey(n.Id, "cpu"), 2, 60); len(points) != 1 || points[0]["v"] != float64(0) {
		t.Fatalf("valid zero lost: %v", points)
	}
}

func TestReviewNodeLegacyAvailabilityMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	dbtest.InitDB(t, path)
	n := &model.Node{Name: "legacy", CpuPct: 0, UptimeSecs: 0}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatal(err)
	}
	// Recreate the pre-change schema, then let the normal startup migration run.
	if err := database.GetDB().Migrator().DropColumn(&model.Node{}, "UnavailableMetrics"); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(path); err != nil {
		t.Fatal(err)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "legacy" || stored.CpuPct != 0 || stored.UptimeSecs != 0 || len(stored.UnavailableMetrics) != 0 {
		t.Fatalf("legacy row changed: id=%d availability=%v", stored.Id, stored.UnavailableMetrics)
	}
}
