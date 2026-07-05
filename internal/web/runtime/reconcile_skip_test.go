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

type nodeCallCounts struct {
	adds            atomic.Int32
	inboundUpdates  atomic.Int32
	clientMutations atomic.Int32
}

func newCountingNodeServer(t *testing.T, clientsResp string) (*httptest.Server, *nodeCallCounts) {
	t.Helper()
	counts := &nodeCallCounts{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/panel/api/inbounds/add"):
			counts.adds.Add(1)
		case strings.Contains(r.URL.Path, "/panel/api/inbounds/update/"):
			counts.inboundUpdates.Add(1)
		case strings.Contains(r.URL.Path, "/panel/api/clients/"):
			counts.clientMutations.Add(1)
			_, _ = w.Write([]byte(clientsResp))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, counts
}

func perClientMutationCases(client model.Client) []struct {
	name string
	run  func(*Remote, *model.Inbound) error
} {
	return []struct {
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
}

// TestPerClientMutationsAloneDoNotSeedReconcileFingerprint: a per-client RPC
// only proves one client's slice converged, so without an explicit advance the
// dirty-reconcile backup must still send the full inbound.
func TestPerClientMutationsAloneDoNotSeedReconcileFingerprint(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	client := model.Client{ID: "11111111-1111-1111-1111-111111111111", Email: "a@x", SubID: "s", Enable: true}
	for _, tt := range perClientMutationCases(client) {
		t.Run(tt.name, func(t *testing.T) {
			counts.inboundUpdates.Store(0)
			counts.clientMutations.Store(0)
			r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
			ib := &model.Inbound{Tag: "in-" + tt.name, Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"a@x","subId":"s","enable":true}]}`}
			r.cacheSet(ib.Tag, 7)

			if err := tt.run(r, ib); err != nil {
				t.Fatalf("%s client mutation: %v", tt.name, err)
			}
			if got := counts.clientMutations.Load(); got != 1 {
				t.Fatalf("%s client mutation requests=%d, want 1", tt.name, got)
			}
			if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || !pushed {
				t.Fatalf("%s reconcile after unadvanced mutation: pushed=%v err=%v, want full push", tt.name, pushed, err)
			}
			if got := counts.inboundUpdates.Load(); got != 1 {
				t.Fatalf("%s reconcile sent %d full inbound updates, want 1", tt.name, got)
			}
		})
	}
}

// TestAdvancePushedInboundEnablesReconcileSkip: when the node provably held the
// pre-edit payload and every per-client push succeeded, advancing the
// fingerprint lets the next reconcile skip the redundant full push.
func TestAdvancePushedInboundEnablesReconcileSkip(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	client := model.Client{ID: "11111111-1111-1111-1111-111111111111", Email: "a@x", SubID: "s", Enable: true}
	prevByOp := map[string]string{
		"add":    `{"clients":[]}`,
		"delete": `{"clients":[{"email":"a@x","enable":true}]}`,
		"update": `{"clients":[{"email":"a@x","enable":false}]}`,
	}
	newByOp := map[string]string{
		"add":    `{"clients":[{"email":"a@x","enable":true}]}`,
		"delete": `{"clients":[]}`,
		"update": `{"clients":[{"email":"a@x","enable":true}]}`,
	}
	for _, tt := range perClientMutationCases(client) {
		t.Run(tt.name, func(t *testing.T) {
			counts.inboundUpdates.Store(0)
			r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
			prevIb := &model.Inbound{Tag: "in-adv-" + tt.name, Protocol: model.VLESS, Port: 443, Settings: prevByOp[tt.name]}
			ib := &model.Inbound{Tag: prevIb.Tag, Protocol: model.VLESS, Port: 443, Settings: newByOp[tt.name]}
			r.cacheSet(ib.Tag, 7)
			r.recordPushedInbound(prevIb)

			if err := tt.run(r, ib); err != nil {
				t.Fatalf("%s client mutation: %v", tt.name, err)
			}
			r.AdvancePushedInbound(prevIb, ib)
			if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
				t.Fatalf("%s reconcile after advance: pushed=%v err=%v, want skip", tt.name, pushed, err)
			}
			if got := counts.inboundUpdates.Load(); got != 0 {
				t.Fatalf("%s reconcile sent %d full inbound updates, want 0", tt.name, got)
			}
		})
	}
}

// TestAdvancePushedInboundRequiresMatchingPreviousFingerprint: if changes were
// folded to dirty (or an earlier push failed), the recorded fingerprint no
// longer matches the pre-edit payload; a later successful client push must not
// mask the pending reconcile.
func TestAdvancePushedInboundRequiresMatchingPreviousFingerprint(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	staleIb := &model.Inbound{Tag: "in-stale", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"email":"folded@x"}]}`}
	prevIb := &model.Inbound{Tag: "in-stale", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
	ib := &model.Inbound{Tag: "in-stale", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"email":"a@x"}]}`}
	r.cacheSet(ib.Tag, 7)
	r.recordPushedInbound(staleIb)

	if err := r.UpdateUser(context.Background(), ib, "a@x", model.Client{Email: "a@x"}); err != nil {
		t.Fatalf("client mutation: %v", err)
	}
	r.AdvancePushedInbound(prevIb, ib)
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || !pushed {
		t.Fatalf("reconcile with unproven pre-state: pushed=%v err=%v, want full push", pushed, err)
	}
	if got := counts.inboundUpdates.Load(); got != 1 {
		t.Fatalf("reconcile sent %d full inbound updates, want 1", got)
	}
}

// TestAdoptedSerializationChainKeepsReconcileSkip: after a push the node
// re-serializes settings its own way and the master adopts that form back into
// its DB; stamping the adopted payload keeps edit->advance->skip alive instead
// of degrading every edit to a full reconcile push.
func TestAdoptedSerializationChainKeepsReconcileSkip(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	pushForm := &model.Inbound{Tag: "in-adopt", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"email":"a@x"}]}`}
	adoptedForm := &model.Inbound{Tag: "in-adopt", Protocol: model.VLESS, Port: 443, Settings: "{\n  \"clients\": [{\"email\": \"a@x\"}]\n}"}
	edited := &model.Inbound{Tag: "in-adopt", Protocol: model.VLESS, Port: 443, Settings: "{\n  \"clients\": [{\"comment\": \"c\", \"email\": \"a@x\"}]\n}"}
	r.cacheSet(pushForm.Tag, 7)

	r.recordPushedInbound(pushForm)
	r.RecordAdoptedInbound(adoptedForm)
	if err := r.UpdateUser(context.Background(), edited, "a@x", model.Client{Email: "a@x"}); err != nil {
		t.Fatalf("client mutation: %v", err)
	}
	r.AdvancePushedInbound(adoptedForm, edited)
	if pushed, err := r.ReconcileInbound(context.Background(), edited, true); err != nil || pushed {
		t.Fatalf("reconcile after adopted-form advance: pushed=%v err=%v, want skip", pushed, err)
	}
	if got := counts.inboundUpdates.Load(); got != 0 {
		t.Fatalf("full inbound updates=%d, want 0", got)
	}
}

func TestDeleteUserNotFoundHandling(t *testing.T) {
	t.Run("envelope not-found counts as already deleted", func(t *testing.T) {
		srv, counts := newCountingNodeServer(t, `{"success":false,"msg":"client not found"}`)
		r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
		ib := &model.Inbound{Tag: "in-delete-missing", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
		r.cacheSet(ib.Tag, 7)

		if err := r.DeleteUser(context.Background(), ib, "missing@x"); err != nil {
			t.Fatalf("DeleteUser missing client: %v", err)
		}
		if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || !pushed {
			t.Fatalf("reconcile after missing-client delete: pushed=%v err=%v, want full push", pushed, err)
		}
		if got := counts.inboundUpdates.Load(); got != 1 {
			t.Fatalf("reconcile sent %d full inbound updates, want 1", got)
		}
	})
	t.Run("http 404 from an old node stays an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/panel/api/clients/") {
				http.Error(w, "404 page not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		defer srv.Close()

		r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
		ib := &model.Inbound{Tag: "in-delete-404", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
		r.cacheSet(ib.Tag, 7)

		err := r.DeleteUser(context.Background(), ib, "missing@x")
		if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
			t.Fatalf("DeleteUser against old node = %v, want HTTP 404 error", err)
		}
	})
}

// TestDelInboundDropsReconcileFingerprint: deleting an inbound must forget its
// fingerprint so a later same-tag inbound with identical content is re-pushed.
func TestDelInboundDropsReconcileFingerprint(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Tag: "in-del", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
	r.cacheSet(ib.Tag, 7)

	if pushed, err := r.ReconcileInbound(context.Background(), ib, false); err != nil || !pushed {
		t.Fatalf("initial reconcile: pushed=%v err=%v, want push", pushed, err)
	}
	if err := r.DelInbound(context.Background(), ib); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	r.cacheSet(ib.Tag, 7)
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || !pushed {
		t.Fatalf("reconcile after DelInbound: pushed=%v err=%v, want full push", pushed, err)
	}
	if got := counts.inboundUpdates.Load(); got != 2 {
		t.Fatalf("full inbound updates=%d, want 2", got)
	}
}

func TestUpdateInboundFallbackAddSeedsReconcileFingerprint(t *testing.T) {
	srv, counts := newCountingNodeServer(t, `{"success":true}`)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Tag: "in-add-fallback", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}

	if err := r.UpdateInbound(context.Background(), ib, ib); err != nil {
		t.Fatalf("UpdateInbound fallback add: %v", err)
	}
	if got := counts.adds.Load(); got != 1 {
		t.Fatalf("fallback add requests=%d, want 1", got)
	}
	if pushed, err := r.ReconcileInbound(context.Background(), ib, true); err != nil || pushed {
		t.Fatalf("reconcile after fallback add: pushed=%v err=%v, want skip", pushed, err)
	}
	if got := counts.inboundUpdates.Load(); got != 0 {
		t.Fatalf("reconcile sent %d full inbound updates, want 0", got)
	}
}
