package integration

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netproxy"
)

func recordingProxy(t *testing.T, hits *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
}

func originServer(t *testing.T, hits *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
}

func TestPanelEgress_NetproxyHelperRoutesThroughProxy(t *testing.T) {
	var proxyHits, originHits int64
	proxy := recordingProxy(t, &proxyHits)
	defer proxy.Close()
	origin := originServer(t, &originHits)
	defer origin.Close()

	client, err := netproxy.NewHTTPClient(proxy.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if atomic.LoadInt64(&proxyHits) != 1 {
		t.Fatalf("expected panel proxy to be hit once, got %d (origin hits=%d)", proxyHits, originHits)
	}
}
