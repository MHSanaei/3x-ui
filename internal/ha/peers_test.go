package ha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParsePeers(t *testing.T) {
	got, err := ParsePeers(" nl = https://nl:2097/ , pl=https://pl:2097")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["nl"] != "https://nl:2097" || got["pl"] != "https://pl:2097" {
		t.Fatalf("got %v", got)
	}
	for _, bad := range []string{"nl", "=https://x", "nl=", "nl=https://a,nl=https://b"} {
		if _, err := ParsePeers(bad); err == nil {
			t.Fatalf("ParsePeers(%q) must fail", bad)
		}
	}
}

func peerServer(t *testing.T, secret string, ping Ping) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != PeerPath || r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(ping)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestProberReportsRoleAndURLs(t *testing.T) {
	nl := peerServer(t, "s3cret", Ping{Code: "nl", Role: Master, PanelURL: "https://nl/panel", SubURL: "https://nl/sub"})
	p := &Prober{Peers: map[string]string{"nl": nl}, Secret: "s3cret"}
	got := p.Probe(context.Background())
	want := PeerStatus{Code: "nl", OK: true, Role: Master, PanelURL: "https://nl/panel", SubURL: "https://nl/sub"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestProberRejectsBadPeers(t *testing.T) {
	cases := []struct {
		name string
		url  func(t *testing.T) string
	}{
		{"wrong secret", func(t *testing.T) string { return peerServer(t, "other", Ping{Code: "nl"}) }},
		{"another machine answers", func(t *testing.T) string { return peerServer(t, "s3cret", Ping{Code: "pl"}) }},
		{"not json", func(t *testing.T) string {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("ok"))
			}))
			t.Cleanup(srv.Close)
			return srv.URL
		}},
		{"unreachable", func(*testing.T) string { return "http://127.0.0.1:1" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &Prober{Peers: map[string]string{"nl": c.url(t)}, Secret: "s3cret"}
			if got := p.Probe(context.Background()); got[0].OK {
				t.Fatalf("peer must not count towards quorum: %+v", got[0])
			}
		})
	}
}

func TestProberHangingPeerDoesNotBlockRound(t *testing.T) {
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(func() { close(release); hang.Close() })
	nl := peerServer(t, "s3cret", Ping{Code: "nl"})

	p := &Prober{
		Peers:  map[string]string{"nl": nl, "pl": hang.URL},
		Secret: "s3cret",
		HTTP:   &http.Client{Timeout: 200 * time.Millisecond},
	}
	start := time.Now()
	got := p.Probe(context.Background())
	if time.Since(start) > 2*time.Second {
		t.Fatal("a hanging peer must time out instead of stalling the round")
	}
	if !got[0].OK || got[1].OK || !strings.EqualFold(got[1].Code, "pl") {
		t.Fatalf("want nl ok and pl down, got %+v", got)
	}
}
