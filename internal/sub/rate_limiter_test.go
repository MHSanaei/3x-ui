package sub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func baseTime() time.Time {
	return time.Unix(1_000_000, 0)
}

func TestSubRateLimiterAllowsUnderLimit(t *testing.T) {
	limiter := newSubRateLimiter(3, time.Minute)
	now := baseTime()
	limiter.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if _, ok := limiter.allow("1.2.3.4"); !ok {
			t.Fatalf("request %d under the limit was rejected", i+1)
		}
	}
	retryAfter, ok := limiter.allow("1.2.3.4")
	if ok {
		t.Fatal("request over the limit was allowed")
	}
	if want := time.Minute; retryAfter > want || retryAfter <= want-time.Second {
		t.Fatalf("retryAfter = %v, want ~%v", retryAfter, want)
	}

	// A different client gets its own bucket.
	if _, ok := limiter.allow("5.6.7.8"); !ok {
		t.Fatal("another client was rejected for the first client's usage")
	}
}

func TestSubRateLimiterWindowSlides(t *testing.T) {
	limiter := newSubRateLimiter(2, time.Minute)
	now := baseTime()
	limiter.now = func() time.Time { return now }

	limiter.allow("c")
	limiter.allow("c")
	if _, ok := limiter.allow("c"); ok {
		t.Fatal("third request in the window was allowed")
	}

	// Half the window later the two hits are still counted.
	now = now.Add(30 * time.Second)
	if _, ok := limiter.allow("c"); ok {
		t.Fatal("request was allowed while the window is still full")
	}

	// One window after the first hit it ages out and a slot frees up.
	now = now.Add(31 * time.Second)
	if _, ok := limiter.allow("c"); !ok {
		t.Fatal("request was rejected after the window slid past the old hits")
	}
}

func TestSubRateLimiterRetryAfterPointsAtOldestHit(t *testing.T) {
	limiter := newSubRateLimiter(1, time.Minute)
	now := baseTime()
	limiter.now = func() time.Time { return now }

	limiter.allow("c")
	now = now.Add(20 * time.Second)
	retryAfter, ok := limiter.allow("c")
	if ok {
		t.Fatal("second request was allowed")
	}
	if want := 40 * time.Second; retryAfter != want {
		t.Fatalf("retryAfter = %v, want %v (oldest hit + window - now)", retryAfter, want)
	}
}

func TestSubRateLimiterEvictsIdleAndCapacity(t *testing.T) {
	limiter := newSubRateLimiter(2, time.Minute)
	limiter.capacity = 2
	now := baseTime()
	limiter.now = func() time.Time { return now }

	limiter.allow("a")
	limiter.allow("b")

	// A third key at capacity evicts the least recently used ("a").
	limiter.allow("c")
	if len(limiter.buckets) != 2 {
		t.Fatalf("buckets = %d, want 2", len(limiter.buckets))
	}
	if _, contains := limiter.buckets["a"]; contains {
		t.Fatal("least recently used bucket was kept at capacity")
	}

	// Touching "b" makes "c" the LRU front; the next insert evicts "c".
	limiter.allow("b")
	limiter.allow("d")
	if _, contains := limiter.buckets["c"]; contains {
		t.Fatal("eviction did not follow the LRU order")
	}
	if _, contains := limiter.buckets["b"]; !contains {
		t.Fatal("recently used bucket was evicted")
	}

	// Idle buckets age out on the next request after the TTL.
	now = now.Add(subRateLimitIdleTTL + time.Minute)
	limiter.allow("e")
	if _, contains := limiter.buckets["b"]; contains {
		t.Fatal("idle bucket survived the idle TTL")
	}
}

func TestSubRateLimiterConcurrentKeys(t *testing.T) {
	limiter := newSubRateLimiter(50, time.Minute)

	const clients = 8
	const perClient = 100
	var mu sync.Mutex
	allowed := make(map[string]int, clients)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("10.0.0.%d", i)
			for j := 0; j < perClient; j++ {
				if _, ok := limiter.allow(key); ok {
					mu.Lock()
					allowed[key]++
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	for key, count := range allowed {
		if count > 50 {
			t.Fatalf("%s got %d allowed requests, limit is 50", key, count)
		}
	}
	if len(allowed) == 0 {
		t.Fatal("no requests were allowed at all")
	}
}

func TestParseTrustedCIDRs(t *testing.T) {
	prefixes, err := parseTrustedCIDRs(" 127.0.0.1/32, ::1, 10.0.0.0/8 ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prefixes) != 3 {
		t.Fatalf("prefixes = %d, want 3", len(prefixes))
	}
	if _, err := parseTrustedCIDRs("*"); err == nil || !strings.Contains(err.Error(), "must not trust every address") {
		t.Fatalf("wildcard error = %v, want the exact trust-widening refusal", err)
	}
	if _, err := parseTrustedCIDRs("10.0.0.0/8,not-an-ip"); err == nil {
		t.Fatal("a malformed entry must be refused")
	}
}

func TestClientIPResolverDirectClientIgnoresHeaders(t *testing.T) {
	resolver := newLoopbackIPResolver()
	ip := resolver.resolve("203.0.113.7:4434", "9.9.9.9", "8.8.8.8")
	if ip != "203.0.113.7" {
		t.Fatalf("ip = %q, want the peer address; an untrusted peer must not set headers", ip)
	}
}

func TestClientIPResolverWalksForwardedForFromProxyEnd(t *testing.T) {
	resolver := newClientIPResolverWith("127.0.0.1/32,10.0.0.0/8")
	// Chain: client 203.0.113.9 -> 10.1.1.1 (trusted) -> panel on loopback.
	ip := resolver.resolve("127.0.0.1:5555", "203.0.113.9, 10.1.1.1", "")
	if ip != "203.0.113.9" {
		t.Fatalf("ip = %q, want the first untrusted hop 203.0.113.9", ip)
	}

	// A client-supplied leftmost entry is ignored: only the proxy-end walk decides.
	ip = resolver.resolve("127.0.0.1:5555", "6.6.6.6, 203.0.113.9, 10.1.1.1", "")
	if ip != "203.0.113.9" {
		t.Fatalf("ip = %q, want the proxy-end client 203.0.113.9, not the client-sent 6.6.6.6", ip)
	}
}

func TestClientIPResolverXRealIPRules(t *testing.T) {
	resolver := newClientIPResolverWith("127.0.0.1/32")

	// X-Real-IP agreeing with the forwarded chain is accepted.
	ip := resolver.resolve("127.0.0.1:5555", "198.51.100.4", "198.51.100.4")
	if ip != "198.51.100.4" {
		t.Fatalf("ip = %q, want the agreed client 198.51.100.4", ip)
	}
	// A spoofed X-Real-IP that disagrees falls back to the proxy address.
	ip = resolver.resolve("127.0.0.1:5555", "198.51.100.4", "6.6.6.6")
	if ip != "127.0.0.1" {
		t.Fatalf("ip = %q, want the proxy address on a header mismatch", ip)
	}
	// X-Real-IP alone from a trusted proxy is the only identity available.
	ip = resolver.resolve("127.0.0.1:5555", "", "198.51.100.4")
	if ip != "198.51.100.4" {
		t.Fatalf("ip = %q, want X-Real-IP when no X-Forwarded-For is present", ip)
	}
	// Malformed headers fall back to the proxy address.
	ip = resolver.resolve("127.0.0.1:5555", "garbage", "also-garbage")
	if ip != "127.0.0.1" {
		t.Fatalf("ip = %q, want the proxy address on malformed headers", ip)
	}
	// An unparseable peer address has no identity at all.
	if ip := resolver.resolve("not-an-address", "", ""); ip != "unknown" {
		t.Fatalf("ip = %q, want unknown for an unparseable peer", ip)
	}
}

func newClientIPResolverWith(cidrs string) *clientIPResolver {
	prefixes, err := parseTrustedCIDRs(cidrs)
	if err != nil {
		panic(err)
	}
	return &clientIPResolver{trusted: prefixes}
}

func TestSubRateLimitGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newSubRateLimiter(2, time.Minute)
	now := baseTime()
	limiter.now = func() time.Time { return now }

	router := gin.New()
	router.GET("/sub/:id", subRateLimitGuard(limiter, newLoopbackIPResolver()), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/abc", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: code = %d, want 200", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/abc", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: code = %d, want 429", w.Code)
	}
	// limit=1, first hit at t=0, reject at t=0: the window frees in exactly 60s.
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want \"60\"", got)
	}

	// A nil limiter keeps the historical behavior: everything passes.
	openRouter := gin.New()
	openRouter.GET("/sub/:id", subRateLimitGuard(nil, nil), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		openRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/abc", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("nil limiter request %d: code = %d, want 200", i+1, w.Code)
		}
	}
}

// initRouter's group-level Use must 429 every subscription format route when limited; unlimited,
// handlers answer 404 themselves — the exact code proves the guard did not interfere.
func TestSUBControllerRateLimitCoversAllRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.MkdirAll("internal/web/dist", 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile("internal/web/dist/subpage.html", []byte("<html><head></head><body></body></html>"), 0o644); err != nil {
		t.Fatalf("write subpage: %v", err)
	}
	t.Setenv("XUI_DB_FOLDER", tmp)
	dbtest.InitDB(t, filepath.Join(tmp, "x-ui.db"))

	limited := func(withLimit bool) *gin.Engine {
		router := gin.New()
		options := []SUBControllerOption{
			WithSUBPath("/sub/"),
			WithSUBJsonPath("/json/"),
			WithSUBJsonEnabled(true),
			WithSUBClashPath("/clash/"),
			WithSUBClashEnabled(true),
		}
		if withLimit {
			limiter := newSubRateLimiter(1, time.Minute)
			limiter.now = func() time.Time { return baseTime() }
			options = append(options, WithSUBRateLimit(limiter, newLoopbackIPResolver()))
		}
		NewSUBController(router.Group("/"), options...)
		return router
	}

	paths := []string{"/sub/abc", "/sub/abc/hwid-status", "/json/abc", "/clash/abc", "/clash-legacy/abc", "/mihomo/abc"}

	// With limit=1 the priming request consumes the slot (handler → 404); every later route rejects.
	router := limited(true)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/prime", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("priming request: code = %d, want 404 from the handler (limiter must allow it)", w.Code)
	}
	for _, path := range paths {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("limited %s: code = %d, want 429", path, w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Fatalf("limited %s: 429 without Retry-After", path)
		}
	}

	// Unlimited controller: the guard is a no-op, handlers answer 404 for the
	// unknown subId — never 429, never 5xx.
	open := limited(false)
	for _, path := range paths {
		w := httptest.NewRecorder()
		open.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unlimited %s: code = %d, want 404 from the handler", path, w.Code)
		}
	}
}
