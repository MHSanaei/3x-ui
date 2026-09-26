package sub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// subscriptionCacheEntryFor reads one cache entry so a test can pin the window a
// fetch scheduled, which is the only observable of the negative cache.
func subscriptionCacheEntryFor(t *testing.T, url string) subscriptionCacheEntry {
	t.Helper()
	key := subscriptionRequest{URL: url}.cacheKey()
	subscriptionCache.Lock()
	defer subscriptionCache.Unlock()
	entry, ok := subscriptionCache.m[key]
	if !ok {
		t.Fatalf("no cache entry for %s", url)
	}
	return entry
}

func ageSubscriptionEntry(t *testing.T, key string) {
	t.Helper()
	subscriptionCache.Lock()
	defer subscriptionCache.Unlock()
	entry, ok := subscriptionCache.m[key]
	if !ok {
		t.Fatalf("no cache entry for %s", key)
	}
	entry.refreshAt = time.Now().Add(-time.Second)
	subscriptionCache.m[key] = entry
}

// TestExternalSubscriptionHonoursTheRowsOwnCacheTTL pins the cacheTtl a library
// row carries: it is how long that row may be served without asking again, so a
// short one has to expire while the shared default still holds.
func TestExternalSubscriptionHonoursTheRowsOwnCacheTTL(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("vless://88888888-8888-8888-8888-888888888888@ttl.example:443"))
	}))
	defer srv.Close()

	short := subscriptionRequest{URL: srv.URL + "/short", CacheTTL: 40 * time.Millisecond}
	if got := fetchSubscriptionLinksFor(short); len(got.links) != 1 {
		t.Fatalf("first fetch links = %#v", got.links)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("requests after the first fetch = %d, want 1", got)
	}
	fetchSubscriptionLinksFor(short)
	if got := hits.Load(); got != 1 {
		t.Fatalf("requests inside the row's own cacheTtl = %d, want the cache to answer", got)
	}

	// Past that window the stale copy is served and refreshed behind it, so the
	// second request is the fetch, not the client's wait for one.
	deadline := time.Now().Add(4 * time.Second)
	for hits.Load() < 2 && time.Now().Before(deadline) {
		fetchSubscriptionLinksFor(short)
		time.Sleep(10 * time.Millisecond)
	}
	if got := hits.Load(); got < 2 {
		t.Fatalf("requests = %d, want the row's own cacheTtl to expire its entry", got)
	}

	// A row without a cacheTtl keeps the shared default, so the same wait must
	// not produce a second request - otherwise every client request would fetch.
	plain := subscriptionRequest{URL: srv.URL + "/plain"}
	if got := fetchSubscriptionLinksFor(plain); len(got.links) != 1 {
		t.Fatalf("plain row fetch links = %#v", got.links)
	}
	before := hits.Load()
	time.Sleep(120 * time.Millisecond)
	fetchSubscriptionLinksFor(plain)
	if got := hits.Load(); got != before {
		t.Fatalf("a row with no cacheTtl refetched after 120ms (%d -> %d): the default TTL must hold", before, got)
	}
}

// TestExternalSubscriptionNegativeBackoffHonoursRetryAfter: a provider that asks
// for a delay decides when it is retried, and one that just fails is retried
// later each time instead of at a fixed interval.
func TestExternalSubscriptionNegativeBackoffHonoursRetryAfter(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)
	var honouring atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		honouring.Add(1)
		w.Header().Set("Retry-After", "150")
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	url := srv.URL + "/retry-after"
	if got := fetchSubscriptionLinks(url).links; len(got) != 0 {
		t.Fatalf("a failing provider returned links: %#v", got)
	}
	if got := honouring.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
	entry := subscriptionCacheEntryFor(t, url)
	wait := time.Until(entry.retryAt)
	if wait < 100*time.Second || wait > 160*time.Second {
		t.Fatalf("retryAt is %v ahead, want the provider's own 150s", wait)
	}
	fetchSubscriptionLinks(url)
	if got := honouring.Load(); got != 1 {
		t.Fatalf("requests = %d, want the provider's Retry-After honoured", got)
	}

	// Without one, the wait starts at the floor and doubles per failure.
	var plain atomic.Int32
	plainSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plain.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer plainSrv.Close()

	plainURL := plainSrv.URL + "/plain"
	fetchSubscriptionLinks(plainURL)
	first := time.Until(subscriptionCacheEntryFor(t, plainURL).retryAt)
	if first < 20*time.Second || first > 40*time.Second {
		t.Fatalf("first backoff = %v, want the 30s floor", first)
	}

	// Force the retry window open so the second failure is observed now instead
	// of after the first wait. The entry is stale rather than cold, so the next
	// request answers from cache and fails again behind it.
	key := subscriptionRequest{URL: plainURL}.cacheKey()
	subscriptionCache.Lock()
	entry = subscriptionCache.m[key]
	entry.retryAt = time.Time{}
	subscriptionCache.m[key] = entry
	subscriptionCache.Unlock()

	deadline := time.Now().Add(4 * time.Second)
	for plain.Load() < 2 && time.Now().Before(deadline) {
		fetchSubscriptionLinks(plainURL)
		time.Sleep(10 * time.Millisecond)
	}
	if got := plain.Load(); got != 2 {
		t.Fatalf("requests after reopening the window = %d, want 2", got)
	}
	second := time.Until(subscriptionCacheEntryFor(t, plainURL).retryAt)
	if second < first+15*time.Second {
		t.Fatalf("second backoff = %v, want it widened past the first (%v)", second, first)
	}
}

// TestExternalSubscriptionRevalidatesWithLastModified: a provider that sends no
// ETag is still asked whether anything changed, and a 304 keeps the cached body
// rather than emptying the subscription.
func TestExternalSubscriptionRevalidatesWithLastModified(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)
	stamp := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	var conditional atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-Modified-Since"); got != "" {
			if got != stamp {
				t.Errorf("If-Modified-Since = %q, want the provider's own stamp %q", got, stamp)
			}
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", stamp)
		_, _ = w.Write([]byte("vless://99999999-9999-9999-9999-999999999999@lm.example:443"))
	}))
	defer srv.Close()

	req := subscriptionRequest{URL: srv.URL + "/last-modified"}
	first := fetchSubscriptionLinksFor(req)
	if len(first.links) != 1 || !strings.Contains(first.links[0], "lm.example") {
		t.Fatalf("first fetch links = %#v", first.links)
	}

	ageSubscriptionEntry(t, req.cacheKey())
	refreshed, err := refreshSubscription(req, req.cacheKey())
	if !errors.Is(err, errNotModified) {
		t.Fatalf("refreshSubscription after 304 err = %v, want errNotModified", err)
	}
	if len(refreshed) != 1 || !strings.Contains(refreshed[0], "lm.example") {
		t.Fatalf("revalidated links = %#v, want the cached body kept", refreshed)
	}
	if got := conditional.Load(); got != 1 {
		t.Fatalf("conditional requests = %d, want 1", got)
	}
}
