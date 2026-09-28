package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// The master replays a node's reset backlog through the node's bulk endpoint.
func TestRemoteResetClientTrafficsPostsEmailsToBulkEndpoint(t *testing.T) {
	var path string
	var body struct {
		Emails []string `json:"emails"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	if err := r.ResetClientTraffics(context.Background(), []string{"a@x", "b@x"}); err != nil {
		t.Fatalf("ResetClientTraffics: %v", err)
	}
	if path != "/panel/api/clients/bulkResetTraffic" || !slices.Equal(body.Emails, []string{"a@x", "b@x"}) {
		t.Fatalf("node got %s %v, want /panel/api/clients/bulkResetTraffic [a@x b@x]", path, body.Emails)
	}
}
