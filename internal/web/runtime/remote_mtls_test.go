package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// nodeForPlainServer builds an http (non-TLS) node so do()'s token handling can
// be exercised without TLS scaffolding.
func nodeForPlainServer(t *testing.T, srv *httptest.Server, mode, token string) *model.Node {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return &model.Node{
		Id: 1, Name: "n1", Scheme: "http", Address: u.Hostname(), Port: port,
		BasePath: "/", ApiToken: token, Enable: true, AllowPrivateAddress: true,
		TlsVerifyMode: mode,
	}
}

// TestRemoteDo_MTLSNodeNoBearer asserts that an mtls node with no API token
// sends its request with NO Authorization header and does not trip the
// empty-token precondition; while a non-mtls node with no token still errors.
func TestRemoteDo_MTLSNodeNoBearer(t *testing.T) {
	var reached bool
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	t.Run("mtls without token sends no Authorization header", func(t *testing.T) {
		reached, gotAuth = false, "sentinel"
		r := NewRemote(nodeForPlainServer(t, srv, "mtls", ""), nil)
		if _, err := r.do(context.Background(), http.MethodGet, "ping", nil); err != nil {
			t.Fatalf("mtls node with no token must not error on the token precondition: %v", err)
		}
		if !reached {
			t.Fatal("request did not reach the server")
		}
		if gotAuth != "" {
			t.Fatalf("Authorization header = %q, want empty for a tokenless mtls node", gotAuth)
		}
	})

	t.Run("non-mtls without token still errors", func(t *testing.T) {
		reached = false
		r := NewRemote(nodeForPlainServer(t, srv, "verify", ""), nil)
		if _, err := r.do(context.Background(), http.MethodGet, "ping", nil); err == nil {
			t.Fatal("non-mtls node with no token must still error")
		}
		if reached {
			t.Fatal("non-mtls tokenless request must not reach the server")
		}
	})
}
