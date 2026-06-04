package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/util/netproxy"
)

func recordingProxy(t *testing.T, hits *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, minDatBytes+1))
	}))
}

func originServer(t *testing.T, hits *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, minDatBytes+1))
	}))
}

func TestPanelProxy_NetproxyHelperRoutesThroughProxy(t *testing.T) {
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

func TestPanelProxy_CustomGeoDownloadUsesProxy(t *testing.T) {
	disableSSRFCheck(t)

	var proxyHits, originHits int64
	proxy := recordingProxy(t, &proxyHits)
	defer proxy.Close()
	origin := originServer(t, &originHits)
	defer origin.Close()

	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	dest := filepath.Join(dir, "geosite_repro.dat")

	s := CustomGeoService{getPanelProxy: func() (string, error) { return proxy.URL, nil }}
	if _, _, err := s.downloadToPath(origin.URL, dest, ""); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}

	if got := atomic.LoadInt64(&proxyHits); got != 1 {
		t.Fatalf("custom geo download did not route through the Panel Network Proxy "+
			"(proxy hits=%d, origin hits=%d)", got, atomic.LoadInt64(&originHits))
	}
}

func TestPanelProxy_CustomGeoDownloadDirectWhenUnset(t *testing.T) {
	disableSSRFCheck(t)

	var proxyHits, originHits int64
	proxy := recordingProxy(t, &proxyHits)
	defer proxy.Close()
	origin := originServer(t, &originHits)
	defer origin.Close()

	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	dest := filepath.Join(dir, "geosite_direct.dat")

	s := CustomGeoService{}
	if _, _, err := s.downloadToPath(origin.URL, dest, ""); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if atomic.LoadInt64(&proxyHits) != 0 || atomic.LoadInt64(&originHits) != 1 {
		t.Fatalf("expected direct connection (proxy=0, origin=1), got proxy=%d origin=%d",
			atomic.LoadInt64(&proxyHits), atomic.LoadInt64(&originHits))
	}
}
