package ha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func patroniServer(t *testing.T, code int, body string) *PatroniClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewPatroniClient(srv.URL, nil)
}

func TestPatroniIsLeader(t *testing.T) {
	cases := []struct {
		name string
		code int
		body string
		want bool
	}{
		{"patroni 3 primary", 200, `{"state":"running","role":"primary"}`, true},
		{"patroni 2 master", 200, `{"state":"running","role":"master"}`, true},
		{"replica", 503, `{"state":"running","role":"replica"}`, false},
		{"standby cluster leader does not write", 503, `{"state":"running","role":"standby_leader"}`, false},
		{"primary still starting", 503, `{"state":"starting","role":"primary"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := patroniServer(t, c.code, c.body).IsLeader(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("IsLeader = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPatroniIsLeaderErrors(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"not json", "<html>bad gateway</html>", "not JSON"},
		{"no role", `{"state":"running"}`, "no role"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := patroniServer(t, 502, c.body).IsLeader(context.Background())
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestPatroniConfig(t *testing.T) {
	dc, err := patroniServer(t, 200, `{"ttl":30,"loop_wait":10,"retry_timeout":10}`).Config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dc != (PatroniTimings{TTL: 30, LoopWait: 10, RetryTimeout: 10}) {
		t.Fatalf("got %+v", dc)
	}
}

func TestPatroniCheckTimings(t *testing.T) {
	cases := []struct {
		name     string
		dc       PatroniTimings
		deadline time.Duration
		want     int
	}{
		{"defaults are safe", PatroniTimings{TTL: 30, LoopWait: 10, RetryTimeout: 10}, 14 * time.Second, 0},
		{"leader cannot demote in time", PatroniTimings{TTL: 20, LoopWait: 10, RetryTimeout: 10}, 14 * time.Second, 1},
		{"panel fences after promotion is allowed", PatroniTimings{TTL: 30, LoopWait: 10, RetryTimeout: 10}, 30 * time.Second, 1},
		{"ttl missing", PatroniTimings{}, time.Second, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.dc.Check(c.deadline); len(got) != c.want {
				t.Fatalf("Check = %q, want %d problems", got, c.want)
			}
		})
	}
}
