package sub

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// resetRoscomVPNState clears the shared cache/lock maps and swaps the URL
// maps to point at a test server, restoring everything on cleanup. Package-
// level state, so every test using it must run serially (no t.Parallel).
func resetRoscomVPNState(t *testing.T, srv *httptest.Server) {
	t.Helper()

	origHapp := roscomvpnHappURLs
	origIncy := roscomvpnIncyURLs
	roscomvpnMu.Lock()
	roscomvpnCache = map[string]roscomvpnCacheEntry{}
	roscomvpnMu.Unlock()
	roscomvpnFetchLocks = sync.Map{}

	if srv != nil {
		roscomvpnHappURLs = map[string]string{
			RoscomVPNSourceDefault:   srv.URL + "/HAPP/DEFAULT.DEEPLINK",
			RoscomVPNSourceJsonSub:   srv.URL + "/HAPP/JSONSUB.DEEPLINK",
			RoscomVPNSourceWhitelist: srv.URL + "/HAPP/WHITELIST.DEEPLINK",
		}
		roscomvpnIncyURLs = map[string]string{
			RoscomVPNSourceDefault:   srv.URL + "/INCY/DEFAULT.DEEPLINK",
			RoscomVPNSourceJsonSub:   srv.URL + "/INCY/JSONSUB.DEEPLINK",
			RoscomVPNSourceWhitelist: srv.URL + "/INCY/WHITELIST.DEEPLINK",
		}
	}

	t.Cleanup(func() {
		roscomvpnHappURLs = origHapp
		roscomvpnIncyURLs = origIncy
		roscomvpnMu.Lock()
		roscomvpnCache = map[string]roscomvpnCacheEntry{}
		roscomvpnMu.Unlock()
		roscomvpnFetchLocks = sync.Map{}
	})
}

func TestResolveRoscomVPNRouting_CustomAndUnknownPassThrough(t *testing.T) {
	resetRoscomVPNState(t, nil)

	cases := []struct {
		name   string
		source string
	}{
		{"explicit custom", "custom"},
		{"empty source defaults to custom", ""},
		{"unrecognized source", "not-a-real-source"},
		{"whitespace and case are normalized but still unknown", "  Not-Real  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveHappRoutingRules(tc.source, "my custom happ://routing/onadd/xyz")
			if got != "my custom happ://routing/onadd/xyz" {
				t.Errorf("got %q, want the custom value unchanged", got)
			}
		})
	}
}

func TestResolveHappRoutingRules_FetchesAndCaches(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte("happ://routing/onadd/deadbeef\n"))
	}))
	defer srv.Close()
	resetRoscomVPNState(t, srv)

	got := ResolveHappRoutingRules(RoscomVPNSourceDefault, "custom-fallback")
	if got != "happ://routing/onadd/deadbeef" {
		t.Fatalf("got %q, want the fetched deeplink trimmed", got)
	}
	if hits != 1 {
		t.Fatalf("expected exactly 1 fetch, got %d", hits)
	}

	// Second call within the TTL must be served from cache, not refetch.
	got2 := ResolveHappRoutingRules(RoscomVPNSourceDefault, "custom-fallback")
	if got2 != got {
		t.Errorf("cached call returned %q, want %q", got2, got)
	}
	if hits != 1 {
		t.Fatalf("expected the cache to serve the second call without a new fetch, got %d total fetches", hits)
	}
}

func TestResolveIncyRoutingRules_DoesNotShareCacheWithHapp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/HAPP/DEFAULT.DEEPLINK":
			w.Write([]byte("happ://routing/onadd/happvalue"))
		case "/INCY/DEFAULT.DEEPLINK":
			w.Write([]byte("incy://routing/onadd/incyvalue"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	resetRoscomVPNState(t, srv)

	happ := ResolveHappRoutingRules(RoscomVPNSourceDefault, "fallback")
	incy := ResolveIncyRoutingRules(RoscomVPNSourceDefault, "fallback")

	if happ != "happ://routing/onadd/happvalue" {
		t.Errorf("happ = %q, want the HAPP deeplink", happ)
	}
	if incy != "incy://routing/onadd/incyvalue" {
		t.Errorf("incy = %q, want the INCY deeplink", incy)
	}
}

func TestResolveRoscomVPNRouting_FallsBackToCustomOnColdFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	resetRoscomVPNState(t, srv)

	got := ResolveHappRoutingRules(RoscomVPNSourceWhitelist, "custom-fallback")
	if got != "custom-fallback" {
		t.Errorf("got %q, want the custom fallback since the cache was cold and the fetch failed", got)
	}
}

func TestResolveRoscomVPNRouting_ServesLastKnownGoodOnSubsequentFailure(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("happ://routing/onadd/good"))
	}))
	defer srv.Close()
	resetRoscomVPNState(t, srv)

	got := ResolveHappRoutingRules(RoscomVPNSourceJsonSub, "custom-fallback")
	if got != "happ://routing/onadd/good" {
		t.Fatalf("got %q, want the first successful fetch", got)
	}

	// Force the cache entry to look expired, then make the next fetch fail.
	roscomvpnMu.Lock()
	entry := roscomvpnCache["happ:"+RoscomVPNSourceJsonSub]
	entry.fetchedAt = time.Now().Add(-roscomvpnCacheTTL - time.Second)
	roscomvpnCache["happ:"+RoscomVPNSourceJsonSub] = entry
	roscomvpnMu.Unlock()
	fail = true

	got2 := ResolveHappRoutingRules(RoscomVPNSourceJsonSub, "custom-fallback")
	if got2 != "happ://routing/onadd/good" {
		t.Errorf("got %q, want the last known-good value preserved across a failed refresh", got2)
	}
}

func TestResolveRoscomVPNRouting_NegativeCacheAvoidsHammeringADeadUpstream(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	resetRoscomVPNState(t, srv)

	ResolveHappRoutingRules(RoscomVPNSourceDefault, "fallback")
	ResolveHappRoutingRules(RoscomVPNSourceDefault, "fallback")
	if hits != 1 {
		t.Errorf("expected the negative cache to suppress the second attempt, got %d fetch attempts", hits)
	}
}
