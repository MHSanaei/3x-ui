package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestReconcileInbound_SkipsUnchanged proves the delta-skip: a second reconcile
// of an unchanged inbound that the node still reports sends no push, while a
// content change or an absent-on-node inbound forces a fresh push.
func TestReconcileInbound_SkipsUnchanged(t *testing.T) {
	var pushes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/panel/api/inbounds/update/") {
			pushes.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Tag: "in-1", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
	// Pre-seed the tag→id cache so resolveRemoteID needs no network round-trip.
	r.cacheSet(ib.Tag, 7)

	// First reconcile: node doesn't report it yet → must push and record the fp.
	if pushed, err := r.ReconcileInbound(context.Background(), ib, false); err != nil || !pushed {
		t.Fatalf("first reconcile: pushed=%v err=%v, want push", pushed, err)
	}
	if got := pushes.Load(); got != 1 {
		t.Fatalf("after first reconcile pushes=%d, want 1", got)
	}

	// Second reconcile: unchanged and present on node → skip.
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
		t.Fatalf("second reconcile: pushed=%v err=%v, want skip", pushed, err)
	}
	if got := pushes.Load(); got != 1 {
		t.Fatalf("unchanged reconcile pushed again: pushes=%d, want 1", got)
	}

	// Content change → push again even though it's present on node.
	ib.Settings = `{"clients":[{"email":"a@x"}]}`
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || !pushed {
		t.Fatalf("changed reconcile: pushed=%v err=%v, want push", pushed, err)
	}
	if got := pushes.Load(); got != 2 {
		t.Fatalf("changed reconcile pushes=%d, want 2", got)
	}

	// Absent on node (e.g. node restarted/lost it) → re-push even if fp matches.
	if pushed, err := r.ReconcileInbound(context.Background(), ib, false); err != nil || !pushed {
		t.Fatalf("absent-on-node reconcile: pushed=%v err=%v, want push", pushed, err)
	}
	if got := pushes.Load(); got != 3 {
		t.Fatalf("absent-on-node reconcile pushes=%d, want 3", got)
	}
}
