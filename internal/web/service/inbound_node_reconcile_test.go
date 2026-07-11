package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// fakeNodePanel serves just enough of the node API for ReconcileNode: the
// inbound list plus update/del endpoints, recording which remote ids get
// deleted.
func fakeNodePanel(t *testing.T, tagToID map[string]int) (*httptest.Server, func() []int) {
	t.Helper()
	var mu sync.Mutex
	var deleted []int
	writeOK := func(w http.ResponseWriter, obj any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "msg": "", "obj": obj})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/panel/api/inbounds/list", func(w http.ResponseWriter, _ *http.Request) {
		type row struct {
			Id  int    `json:"id"`
			Tag string `json:"tag"`
		}
		rows := make([]row, 0, len(tagToID))
		for tag, id := range tagToID {
			rows = append(rows, row{Id: id, Tag: tag})
		}
		writeOK(w, rows)
	})
	mux.HandleFunc("/panel/api/inbounds/update/", func(w http.ResponseWriter, _ *http.Request) {
		writeOK(w, nil)
	})
	mux.HandleFunc("/panel/api/inbounds/del/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/panel/api/inbounds/del/"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		mu.Lock()
		deleted = append(deleted, id)
		mu.Unlock()
		writeOK(w, nil)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, func() []int {
		mu.Lock()
		defer mu.Unlock()
		out := append([]int(nil), deleted...)
		sort.Ints(out)
		return out
	}
}

func reconcileTestNode(t *testing.T, ts *httptest.Server, name, mode string, tags []string) *model.Node {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse test server port: %v", err)
	}
	n := &model.Node{
		Name:                name,
		Scheme:              "http",
		Address:             u.Hostname(),
		Port:                port,
		BasePath:            "/",
		ApiToken:            "tok",
		Enable:              true,
		AllowPrivateAddress: true,
		Status:              "online",
		InboundSyncMode:     mode,
		InboundTags:         tags,
		InboundsAdoptedAt:   1,
	}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return n
}

// In "selected" sync mode the panel never imports the unselected inbounds, so
// reconcile must not treat their absence from the local DB as a deletion: only
// a *selected* tag missing locally may be swept from the node.
func TestReconcileNode_SelectedModeLeavesUnselectedRemoteInbounds(t *testing.T) {
	setupConflictDB(t)

	ts, deletedIDs := fakeNodePanel(t, map[string]int{
		"keep":          1,
		"selected-gone": 2,
		"unmanaged":     3,
	})
	node := reconcileTestNode(t, ts, "sel-node", "selected", []string{"keep", "selected-gone"})
	seedInboundConflictNode(t, "keep", "", 443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`, &node.Id)

	svc := InboundService{}
	if err := svc.ReconcileNode(context.Background(), runtime.NewRemote(node, nil), node); err != nil {
		t.Fatalf("ReconcileNode: %v", err)
	}

	got := deletedIDs()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("deleted remote ids = %v, want [2] (unmanaged inbound 3 must survive)", got)
	}
}

// "all" mode keeps the original anti-entropy contract: every remote inbound
// missing from the local DB is deleted on the node.
func TestReconcileNode_AllModeDeletesUndesiredRemoteInbounds(t *testing.T) {
	setupConflictDB(t)

	ts, deletedIDs := fakeNodePanel(t, map[string]int{
		"keep":   1,
		"gone-a": 2,
		"gone-b": 3,
	})
	node := reconcileTestNode(t, ts, "all-node", "all", nil)
	seedInboundConflictNode(t, "keep", "", 443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`, &node.Id)

	svc := InboundService{}
	if err := svc.ReconcileNode(context.Background(), runtime.NewRemote(node, nil), node); err != nil {
		t.Fatalf("ReconcileNode: %v", err)
	}

	got := deletedIDs()
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("deleted remote ids = %v, want [2 3]", got)
	}
}

// A node whose pre-existing inbounds were never adopted into the central DB
// has zero local rows for legitimate reasons: reconcile before that first
// adoption must not sweep — it would delete every real inbound on the node
// right after onboarding (add node, save it again, watch it get wiped).
func TestReconcileNode_SkipsSweepBeforeFirstAdoption(t *testing.T) {
	setupConflictDB(t)

	ts, deletedIDs := fakeNodePanel(t, map[string]int{
		"real-a": 1,
		"real-b": 2,
		"real-c": 3,
	})
	node := reconcileTestNode(t, ts, "fresh-node", "all", nil)
	node.InboundsAdoptedAt = 0

	svc := InboundService{}
	if err := svc.ReconcileNode(context.Background(), runtime.NewRemote(node, nil), node); err != nil {
		t.Fatalf("ReconcileNode: %v", err)
	}

	if got := deletedIDs(); len(got) != 0 {
		t.Fatalf("deleted remote ids = %v, want none before first adoption", got)
	}
}

// One inbound the node rejects (e.g. a legacy protocol failing the node's
// request validation, #5685) must not abort the reconcile: the healthy inbound
// is still pushed, the delete sweep still runs, and the returned error names
// the failed tag so the caller keeps the dirty flag set for retry.
func TestReconcileNode_ContinuesPastFailedInbound(t *testing.T) {
	setupConflictDB(t)

	var mu sync.Mutex
	updated := map[int]int{}
	var deleted []int
	tagToID := map[string]int{"legacy": 1, "healthy": 2, "gone": 3}
	writeOK := func(w http.ResponseWriter, obj any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "msg": "", "obj": obj})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/panel/api/inbounds/list", func(w http.ResponseWriter, _ *http.Request) {
		type row struct {
			Id  int    `json:"id"`
			Tag string `json:"tag"`
		}
		rows := make([]row, 0, len(tagToID))
		for tag, id := range tagToID {
			rows = append(rows, row{Id: id, Tag: tag})
		}
		writeOK(w, rows)
	})
	mux.HandleFunc("/panel/api/inbounds/update/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/panel/api/inbounds/update/"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		if id == tagToID["legacy"] {
			http.Error(w, "request body failed validation", http.StatusBadRequest)
			return
		}
		mu.Lock()
		updated[id]++
		mu.Unlock()
		writeOK(w, nil)
	})
	mux.HandleFunc("/panel/api/inbounds/del/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/panel/api/inbounds/del/"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		mu.Lock()
		deleted = append(deleted, id)
		mu.Unlock()
		writeOK(w, nil)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	node := reconcileTestNode(t, ts, "half-broken-node", "all", nil)
	seedInboundConflictNode(t, "legacy", "", 1080, model.Protocol("socks"), ``, `{"auth":"noauth"}`, &node.Id)
	seedInboundConflictNode(t, "healthy", "", 443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`, &node.Id)

	svc := InboundService{}
	err := svc.ReconcileNode(context.Background(), runtime.NewRemote(node, nil), node)
	if err == nil {
		t.Fatal("ReconcileNode: want an error naming the rejected inbound, got nil")
	}
	if !strings.Contains(err.Error(), `reconcile inbound "legacy"`) {
		t.Fatalf("ReconcileNode error = %q, want it to name inbound \"legacy\"", err)
	}

	mu.Lock()
	healthyPushes := updated[tagToID["healthy"]]
	gotDeleted := append([]int(nil), deleted...)
	mu.Unlock()
	if healthyPushes != 1 {
		t.Fatalf("healthy inbound pushed %d times, want 1", healthyPushes)
	}
	sort.Ints(gotDeleted)
	if len(gotDeleted) != 1 || gotDeleted[0] != tagToID["gone"] {
		t.Fatalf("deleted remote ids = %v, want [%d] (sweep must still run past the failure)", gotDeleted, tagToID["gone"])
	}
}

func TestEnsureInboundTagAllowed(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	svc := NodeService{}

	selected := &model.Node{
		Name: "ensure-sel", Address: "127.0.0.1", Port: 2096, ApiToken: "tok",
		InboundSyncMode: "selected", InboundTags: []string{"a"},
	}
	if err := db.Create(selected).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := svc.EnsureInboundTagAllowed(selected.Id, "b"); err != nil {
		t.Fatalf("EnsureInboundTagAllowed add: %v", err)
	}
	var got model.Node
	if err := db.First(&got, selected.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if len(got.InboundTags) != 2 || got.InboundTags[0] != "a" || got.InboundTags[1] != "b" {
		t.Fatalf("InboundTags = %#v, want [a b]", got.InboundTags)
	}

	if err := svc.EnsureInboundTagAllowed(selected.Id, "a"); err != nil {
		t.Fatalf("EnsureInboundTagAllowed existing: %v", err)
	}
	if err := db.First(&got, selected.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if len(got.InboundTags) != 2 {
		t.Fatalf("existing tag must not duplicate, got %#v", got.InboundTags)
	}

	all := &model.Node{
		Name: "ensure-all", Address: "127.0.0.1", Port: 2097, ApiToken: "tok",
		InboundSyncMode: "all",
	}
	if err := db.Create(all).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := svc.EnsureInboundTagAllowed(all.Id, "x"); err != nil {
		t.Fatalf("EnsureInboundTagAllowed all-mode: %v", err)
	}
	var gotAll model.Node
	if err := db.First(&gotAll, all.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if len(gotAll.InboundTags) != 0 {
		t.Fatalf("all-mode node must stay without tags, got %#v", gotAll.InboundTags)
	}
}
