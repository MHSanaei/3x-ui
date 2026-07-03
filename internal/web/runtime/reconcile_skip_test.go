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

func TestRemotePerClientMutationsSeedReconcileFingerprint(t *testing.T) {
	var inboundUpdates atomic.Int32
	var clientMutations atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/panel/api/inbounds/update/"):
			inboundUpdates.Add(1)
		case strings.Contains(r.URL.Path, "/panel/api/clients/"):
			clientMutations.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	client := model.Client{ID: "11111111-1111-1111-1111-111111111111", Email: "a@x", SubID: "s", Enable: true}
	tests := []struct {
		name string
		run  func(*Remote, *model.Inbound) error
	}{
		{"add", func(r *Remote, ib *model.Inbound) error {
			return r.AddClient(context.Background(), ib, client)
		}},
		{"delete", func(r *Remote, ib *model.Inbound) error {
			return r.DeleteUser(context.Background(), ib, client.Email)
		}},
		{"update", func(r *Remote, ib *model.Inbound) error {
			return r.UpdateUser(context.Background(), ib, client.Email, client)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inboundUpdates.Store(0)
			clientMutations.Store(0)
			r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
			ib := &model.Inbound{Tag: "in-" + tt.name, Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"a@x","subId":"s","enable":true}]}`}
			r.cacheSet(ib.Tag, 7)

			if err := tt.run(r, ib); err != nil {
				t.Fatalf("%s client mutation: %v", tt.name, err)
			}
			if got := clientMutations.Load(); got != 1 {
				t.Fatalf("%s client mutation requests=%d, want 1", tt.name, got)
			}
			if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
				t.Fatalf("%s reconcile after client mutation: pushed=%v err=%v, want skip", tt.name, pushed, err)
			}
			if got := inboundUpdates.Load(); got != 0 {
				t.Fatalf("%s reconcile sent %d full inbound updates, want 0", tt.name, got)
			}
		})
	}
}

func TestDeleteUserNotFoundSeedsReconcileFingerprint(t *testing.T) {
	var inboundUpdates atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/panel/api/inbounds/update/") {
			inboundUpdates.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/panel/api/clients/") {
			_, _ = w.Write([]byte(`{"success":false,"msg":"client not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Tag: "in-delete-missing", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
	r.cacheSet(ib.Tag, 7)

	if err := r.DeleteUser(context.Background(), ib, "missing@x"); err != nil {
		t.Fatalf("DeleteUser missing client: %v", err)
	}
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
		t.Fatalf("reconcile after missing-client delete: pushed=%v err=%v, want skip", pushed, err)
	}
	if got := inboundUpdates.Load(); got != 0 {
		t.Fatalf("reconcile sent %d full inbound updates, want 0", got)
	}
}

func TestUpdateInboundFallbackAddSeedsReconcileFingerprint(t *testing.T) {
	var adds atomic.Int32
	var inboundUpdates atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/panel/api/inbounds/add"):
			adds.Add(1)
		case strings.Contains(r.URL.Path, "/panel/api/inbounds/update/"):
			inboundUpdates.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Tag: "in-add-fallback", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}

	if err := r.UpdateInbound(context.Background(), ib, ib); err != nil {
		t.Fatalf("UpdateInbound fallback add: %v", err)
	}
	if got := adds.Load(); got != 1 {
		t.Fatalf("fallback add requests=%d, want 1", got)
	}
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
		t.Fatalf("reconcile after fallback add: pushed=%v err=%v, want skip", pushed, err)
	}
	if got := inboundUpdates.Load(); got != 0 {
		t.Fatalf("reconcile sent %d full inbound updates, want 0", got)
	}
}
