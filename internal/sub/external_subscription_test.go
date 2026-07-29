package sub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resetSubscriptionCache(t *testing.T) {
	t.Helper()
	subscriptionCache.Lock()
	previousEntries := subscriptionCache.m
	previousInflight := subscriptionCache.inflight
	subscriptionCache.m = make(map[string]subscriptionCacheEntry)
	subscriptionCache.inflight = make(map[string]*subscriptionFetch)
	subscriptionCache.Unlock()
	t.Cleanup(func() {
		subscriptionCache.Lock()
		subscriptionCache.m = previousEntries
		subscriptionCache.inflight = previousInflight
		subscriptionCache.Unlock()
	})
}

func TestFetchSubscriptionLinksSharesConcurrentRefresh(t *testing.T) {
	resetSubscriptionCache(t)
	var requests atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-release
		_, _ = w.Write([]byte("vless://uuid@example.com:443"))
	}))
	defer srv.Close()

	const callers = 16
	results := make(chan []string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- fetchSubscriptionLinks(srv.URL)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)

	for links := range results {
		if len(links) != 1 || links[0] != "vless://uuid@example.com:443" {
			t.Fatalf("links = %#v", links)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestFetchSubscriptionLinksBoundsCacheSize(t *testing.T) {
	resetSubscriptionCache(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("vless://uuid@example.com:443"))
	}))
	defer srv.Close()

	for i := range subscriptionCacheCapacity + 1 {
		links := fetchSubscriptionLinks(srv.URL + "?id=" + strconv.Itoa(i))
		if len(links) != 1 {
			t.Fatalf("links at %d = %#v", i, links)
		}
	}

	subscriptionCache.Lock()
	entries := len(subscriptionCache.m)
	subscriptionCache.Unlock()
	if entries != subscriptionCacheCapacity {
		t.Fatalf("cache entries = %d, want %d", entries, subscriptionCacheCapacity)
	}
	if got := requests.Load(); got != subscriptionCacheCapacity+1 {
		t.Fatalf("requests = %d, want %d", got, subscriptionCacheCapacity+1)
	}
}

func TestFetchSubscriptionLinksSharesStaleResultAfterRefreshFailure(t *testing.T) {
	resetSubscriptionCache(t)
	stale := []string{"vless://stale@example.com:443"}
	release := make(chan struct{})
	var staleRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stale" {
			staleRequests.Add(1)
			<-release
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("vless://fresh@example.com:443"))
	}))
	defer srv.Close()
	staleURL := srv.URL + "/stale"

	subscriptionCache.Lock()
	subscriptionCache.m[staleURL] = subscriptionCacheEntry{
		links:     stale,
		fetchedAt: time.Now().Add(-subscriptionCacheTTL),
	}
	for i := range subscriptionCacheCapacity - 1 {
		subscriptionCache.m["cached-"+strconv.Itoa(i)] = subscriptionCacheEntry{fetchedAt: time.Now()}
	}
	subscriptionCache.Unlock()

	const callers = 16
	results := make(chan []string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- fetchSubscriptionLinks(staleURL)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	if links := fetchSubscriptionLinks(srv.URL + "/fresh"); len(links) != 1 || links[0] != "vless://fresh@example.com:443" {
		t.Fatalf("fresh links = %#v", links)
	}
	close(release)
	wg.Wait()
	close(results)

	for links := range results {
		if len(links) != 1 || links[0] != stale[0] {
			t.Fatalf("links = %#v, want %#v", links, stale)
		}
	}
	if got := staleRequests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestDoFetchSubscriptionLinks_RejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", subscriptionMaxBytes+1)))
	}))
	defer srv.Close()

	links, err := doFetchSubscriptionLinks(srv.URL)
	if !errors.Is(err, errSubscriptionBodyTooLarge) {
		t.Fatalf("err = %v, want errSubscriptionBodyTooLarge", err)
	}
	if links != nil {
		t.Fatalf("links = %v, want nil", links)
	}
}

func TestDoFetchSubscriptionLinks_AcceptsBodyAtLimit(t *testing.T) {
	link := "vless://example"
	body := link + "\n" + strings.Repeat("#", subscriptionMaxBytes-len(link)-1)
	if len(body) != subscriptionMaxBytes {
		t.Fatalf("fixture size = %d, want %d", len(body), subscriptionMaxBytes)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	links, err := doFetchSubscriptionLinks(srv.URL)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(links) != 1 || links[0] != link {
		t.Fatalf("links = %v, want [%q]", links, link)
	}
}
