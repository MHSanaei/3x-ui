package mtproto

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// serverPort extracts the loopback port a httptest server bound to, so
// scrapeStats can rebuild the same http://127.0.0.1:<port>/stats URL.
func serverPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestScrapeStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"started_at":"2026-01-01T00:00:00Z","total_connections":2,`+
			`"users":{`+
			`"alice":{"connections":2,"bytes_in":100,"bytes_out":200,"last_seen":"2026-01-01T00:01:00Z"},`+
			`"bob":{"connections":0,"bytes_in":5,"bytes_out":7,"last_seen":null}}}`)
	}))
	defer srv.Close()

	users, ok := scrapeStats(serverPort(t, srv))
	if !ok {
		t.Fatal("scrapeStats should succeed against a valid /stats endpoint")
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d: %+v", len(users), users)
	}
	if users["alice"].BytesIn != 100 || users["alice"].BytesOut != 200 || users["alice"].Connections != 2 {
		t.Fatalf("alice stats parsed wrong: %+v", users["alice"])
	}
	if users["bob"].Connections != 0 || users["bob"].BytesIn != 5 {
		t.Fatalf("bob stats parsed wrong: %+v", users["bob"])
	}
}

func TestScrapeStatsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	port := serverPort(t, srv)
	srv.Close()

	if _, ok := scrapeStats(port); ok {
		t.Fatal("scrapeStats must report ok=false when the endpoint is unreachable")
	}
}
