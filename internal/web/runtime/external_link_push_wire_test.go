package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRemotePushExternalLinksSendsTheWireSnapshot pins the master's half of the
// node link push: the exact endpoint the node's token scope allows, sent as JSON,
// with both halves of the snapshot intact. A dropped field here is invisible
// until a node starts serving links the master never resolved.
func TestRemotePushExternalLinksSendsTheWireSnapshot(t *testing.T) {
	var (
		gotPath, gotMethod, gotType string
		gotBody                     []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath, gotMethod, gotType = req.URL.Path, req.Method, req.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(req.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"msg":"ok","obj":{"links":1,"clients":1}}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	payload := ExternalLinkSync{
		Links: []ExternalLinkSyncLink{{
			Kind: "link", Value: "trojan://push@wire.example:443", Remark: "from-master",
			Enable: true, SortIndex: 3, UserAgent: "master-ua",
			Headers: map[string]string{"X-Token": "abc"}, CacheTtl: 90, ExpiryTime: 1893456000000,
		}},
		Clients: []ExternalLinkSyncClient{{
			Email: "wire@node",
			Links: []ExternalLinkSyncAssignment{{
				Kind: "link", Value: "trojan://push@wire.example:443", Enable: true, Remark: "resolved",
			}},
		}},
	}
	if err := r.PushExternalLinks(context.Background(), payload); err != nil {
		t.Fatalf("PushExternalLinks: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	// The path is load-bearing twice: the node routes on it and its node-sync
	// token is only allowed this exact route.
	if gotPath != "/panel/api/links/sync" {
		t.Errorf("path = %s, want /panel/api/links/sync", gotPath)
	}
	if gotType != "application/json" {
		t.Errorf("content type = %s, want application/json", gotType)
	}

	var sent ExternalLinkSync
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("decode the pushed body: %v (body=%s)", err, gotBody)
	}
	if len(sent.Links) != 1 || len(sent.Clients) != 1 {
		t.Fatalf("pushed snapshot = %+v, want one link and one client", sent)
	}
	link := sent.Links[0]
	if link.Kind != payload.Links[0].Kind || link.Value != payload.Links[0].Value ||
		link.Remark != payload.Links[0].Remark || link.Enable != payload.Links[0].Enable ||
		link.SortIndex != payload.Links[0].SortIndex || link.UserAgent != payload.Links[0].UserAgent ||
		link.CacheTtl != payload.Links[0].CacheTtl || link.ExpiryTime != payload.Links[0].ExpiryTime ||
		link.Headers["X-Token"] != "abc" {
		t.Fatalf("pushed link = %+v, want the payload unchanged", link)
	}
	if sent.Clients[0].Email != "wire@node" || len(sent.Clients[0].Links) != 1 ||
		sent.Clients[0].Links[0].Value != "trojan://push@wire.example:443" {
		t.Fatalf("pushed client = %+v, want the resolved assignment", sent.Clients[0])
	}
}

// TestRemotePushExternalLinksSurfacesANodeThatRefusesIt: the node answers a
// refused push with a non-2xx status - an old build without the route, or a token
// without the scope. That must reach the caller as an error, because the only
// trace of a silently dropped push is links missing on the node.
func TestRemotePushExternalLinksSurfacesANodeThatRefusesIt(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"old node without the route", http.StatusNotFound, `{"success":false,"msg":"404 page not found"}`},
		{"token without the scope", http.StatusForbidden, `{"success":false,"msg":"this API token is not permitted to access this endpoint"}`},
		{"refused payload", http.StatusOK, `{"success":false,"msg":"unknown external link kind: bogus"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
			err := r.PushExternalLinks(context.Background(), ExternalLinkSync{
				Links: []ExternalLinkSyncLink{{Kind: "link", Value: "trojan://refused@node.example:443", Enable: true}},
			})
			if err == nil {
				t.Fatal("a refused push returned no error")
			}
		})
	}
}
