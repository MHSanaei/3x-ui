package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// linkCount returns how many client_inbounds links a client currently has,
// across every inbound — the value ReapSyncOrphans checks before deleting.
func linkCount(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var n int64
	if err := db.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ?", email).
		Count(&n).Error; err != nil {
		t.Fatalf("count links for %q: %v", email, err)
	}
	return n
}

// A degraded node reporting zero clients for an inbound the hub populates must
// keep its links and never orphan-mark, or SyncInbound/ReapSyncOrphans delete the row.
func TestSetRemoteTraffic_EmptySnapshotKeepsClients(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "svc@x")

	settings := `{"clients":[{"email":"svc@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", settings,
		xray.ClientTraffic{Email: "svc@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if n := linkCount(t, db, "svc@x"); n != 1 {
		t.Fatalf("setup: svc@x links=%d, want 1", n)
	}

	// The node returns an empty snapshot — the trigger that deleted real clients.
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false, false); err != nil {
		t.Fatalf("empty-snapshot sync: %v", err)
	}

	if rec, _ := countClientRows(t, db, "svc@x"); rec != 1 {
		t.Fatalf("empty snapshot deleted the client row: clients=%d, want 1", rec)
	}
	if n := linkCount(t, db, "svc@x"); n != 1 {
		t.Fatalf("empty snapshot stripped the client link: links=%d, want 1", n)
	}
	if at := readOrphanMark(t, db, "svc@x"); at != 0 {
		t.Fatalf("empty snapshot orphan-marked a live client: sync_orphaned_at=%d, want 0", at)
	}
	// The hub must keep the client in the inbound's settings, or reconcile re-pushes
	// an empty blob to the node and the clients never come back (#6734).
	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("read central inbound: %v", err)
	}
	if !strings.Contains(ib.Settings, "svc@x") {
		t.Fatalf("empty snapshot blanked the inbound settings: %q", ib.Settings)
	}
}

// The guard is narrow: a snapshot still carrying a client is authoritative, so a
// client the node really dropped is unlinked and orphan-marked; only all-empty is degraded.
func TestSetRemoteTraffic_PartialSnapshotStillPrunes(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "keep@x")

	bothSettings := `{"clients":[{"email":"keep@x","enable":true},{"email":"drop@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", bothSettings,
		xray.ClientTraffic{Email: "keep@x", Enable: true},
		xray.ClientTraffic{Email: "drop@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if n := linkCount(t, db, "drop@x"); n != 1 {
		t.Fatalf("setup: drop@x links=%d, want 1", n)
	}

	// Node now reports only keep@x — drop@x was genuinely removed there.
	keepOnlySettings := `{"clients":[{"email":"keep@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", keepOnlySettings,
		xray.ClientTraffic{Email: "keep@x", Enable: true}), false, false); err != nil {
		t.Fatalf("partial-snapshot sync: %v", err)
	}

	if n := linkCount(t, db, "keep@x"); n != 1 {
		t.Fatalf("partial snapshot dropped a reported client: keep@x links=%d, want 1", n)
	}
	if n := linkCount(t, db, "drop@x"); n != 0 {
		t.Fatalf("partial snapshot kept an unreported client linked: drop@x links=%d, want 0", n)
	}
	if at := readOrphanMark(t, db, "drop@x"); at <= 0 {
		t.Fatalf("partial snapshot did not orphan-mark the removed client: sync_orphaned_at=%d, want >0", at)
	}
}

// Keeping the hub's settings is not recovery: the node is only healed once the
// hub actually re-pushes them, which needs a dirty node and a stale fingerprint.
func TestSetRemoteTraffic_EmptySnapshotRepushesHubClients(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	var mu sync.Mutex
	var pushed []string
	writeOK := func(w http.ResponseWriter, obj any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "msg": "", "obj": obj})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/panel/api/inbounds/list", func(w http.ResponseWriter, _ *http.Request) {
		writeOK(w, []map[string]any{{"id": 7, "tag": "deg-in", "port": 41001, "protocol": "vless"}})
	})
	mux.HandleFunc("/panel/api/inbounds/update/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		pushed = append(pushed, r.PostForm.Get("settings"))
		mu.Unlock()
		writeOK(w, nil)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	node := reconcileTestNode(t, ts, "deg-node", "all", nil)
	settings := `{"clients":[{"email":"svc@x","enable":true,"id":"11111111-1111-1111-1111-111111111111"}]}`
	nid := node.Id
	if err := db.Create(&model.Inbound{UserId: 1, Tag: "deg-in", Enable: true, Port: 41001, Protocol: model.VLESS, NodeID: &nid, Settings: settings}).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rt := runtime.NewRemote(node, nil)
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetRuntimeOverride(node.Id, rt)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	if _, err := svc.setRemoteTrafficLocked(node.Id, snapshotWithClients(t, "deg-in", settings,
		xray.ClientTraffic{Email: "svc@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if err := svc.ReconcileNode(context.Background(), rt, node); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	mu.Lock()
	pushed = nil
	mu.Unlock()

	if _, err := svc.setRemoteTrafficLocked(node.Id, snapshotWithoutClients(t, "deg-in"), false, false); err != nil {
		t.Fatalf("empty-snapshot sync: %v", err)
	}
	var after model.Node
	if err := db.Where("id = ?", node.Id).First(&after).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if !after.ConfigDirty {
		t.Fatal("empty snapshot left the node clean: the job never reconciles it, so the node stays without its clients")
	}
	if err := svc.ReconcileNode(context.Background(), rt, &after); err != nil {
		t.Fatalf("reconcile after empty snapshot: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 1 || !strings.Contains(pushed[0], "svc@x") {
		t.Fatalf("reconcile after empty snapshot pushed %d settings payload(s) %q, want one carrying svc@x", len(pushed), pushed)
	}
}

// The traffic a client used while its node reported nothing must still count
// once the node reports it again.
func TestSetRemoteTraffic_EmptySnapshotKeepsTrafficBaseline(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "svc@x")
	settings := `{"clients":[{"email":"svc@x","enable":true}]}`
	for _, used := range []int64{100, 200} {
		syncNodeWithSettings(t, svc, 1, "n1-in", settings, xray.ClientTraffic{Email: "svc@x", Up: used, Down: used, Enable: true})
	}
	before := readTraffic(t, db, "svc@x")

	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false, false); err != nil {
		t.Fatalf("empty-snapshot sync: %v", err)
	}
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, xray.ClientTraffic{Email: "svc@x", Up: 250, Down: 250, Enable: true})

	assertUpDown(t, readTraffic(t, db, "svc@x"), before.Up+50, before.Down+50, "after the node recovered")
}
