package sub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RoscomVPN routing presets: ready-made routing-rule deeplinks for Happ and
// Incy, published at hydraponique/roscomvpn-routing and regenerated
// automatically whenever that project's upstream geoip/geosite lists
// change. Each *.DEEPLINK file is a single line -- happ://routing/onadd/<b64>
// or incy://routing/onadd/<b64> -- ready to drop straight into the header
// (Happ) or subscription body (Incy) this fork already emits for a
// hand-pasted custom value. "custom" (or an unrecognized/empty source)
// always returns the admin's own free-text setting untouched, so nothing
// changes for anyone who never opts into a preset.
const (
	RoscomVPNSourceDefault   = "default"
	RoscomVPNSourceJsonSub   = "jsonsub"
	RoscomVPNSourceWhitelist = "whitelist"
	RoscomVPNSourceCustom    = "custom"

	roscomvpnCacheTTL      = 10 * time.Minute
	roscomvpnNegativeCache = 30 * time.Second // back off after a fetch failure so a dead upstream can't slow every subscription response
	roscomvpnHTTPTimeout   = 4 * time.Second
	roscomvpnMaxBodyBytes  = 1 << 20 // 1 MiB cap on a .DEEPLINK response
)

var roscomvpnHappURLs = map[string]string{
	RoscomVPNSourceDefault:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/DEFAULT.DEEPLINK",
	RoscomVPNSourceJsonSub:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/JSONSUB.DEEPLINK",
	RoscomVPNSourceWhitelist: "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/WHITELIST.DEEPLINK",
}

var roscomvpnIncyURLs = map[string]string{
	RoscomVPNSourceDefault:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/INCY/DEFAULT.DEEPLINK",
	RoscomVPNSourceJsonSub:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/INCY/JSONSUB.DEEPLINK",
	RoscomVPNSourceWhitelist: "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/INCY/WHITELIST.DEEPLINK",
}

type roscomvpnCacheEntry struct {
	value     string
	fetchedAt time.Time
	lastFail  time.Time // zero if the last attempt succeeded
}

var (
	roscomvpnMu     sync.RWMutex
	roscomvpnCache  = map[string]roscomvpnCacheEntry{}
	roscomvpnClient = &http.Client{Timeout: roscomvpnHTTPTimeout}

	// Per-cache-key fetch lock so concurrent subscription requests for the
	// same (app, source) coalesce into a single outbound HTTP call instead
	// of each racing to refresh an expired cache entry.
	roscomvpnFetchLocks sync.Map // map[string]*sync.Mutex
)

func roscomvpnLockFor(key string) *sync.Mutex {
	if m, ok := roscomvpnFetchLocks.Load(key); ok {
		return m.(*sync.Mutex)
	}
	m, _ := roscomvpnFetchLocks.LoadOrStore(key, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// ResolveHappRoutingRules returns the value for Happ's "Routing" response
// header: a known RoscomVPN source name is fetched (with caching) from
// GitHub; "custom" or anything unrecognized returns custom unchanged.
func ResolveHappRoutingRules(source, custom string) string {
	return resolveRoscomVPNRouting("happ", roscomvpnHappURLs, source, custom)
}

// ResolveIncyRoutingRules returns the line appended to an Incy subscription
// body for routing rules, with the same source/custom semantics as
// ResolveHappRoutingRules.
func ResolveIncyRoutingRules(source, custom string) string {
	return resolveRoscomVPNRouting("incy", roscomvpnIncyURLs, source, custom)
}

func resolveRoscomVPNRouting(app string, urls map[string]string, source, custom string) string {
	src := strings.ToLower(strings.TrimSpace(source))
	if src == "" {
		src = RoscomVPNSourceCustom
	}
	if src == RoscomVPNSourceCustom {
		return custom
	}
	url, ok := urls[src]
	if !ok {
		return custom
	}
	cacheKey := app + ":" + src

	roscomvpnMu.RLock()
	entry, hit := roscomvpnCache[cacheKey]
	roscomvpnMu.RUnlock()
	if hit && time.Since(entry.fetchedAt) < roscomvpnCacheTTL {
		return entry.value
	}
	if hit && !entry.lastFail.IsZero() && time.Since(entry.lastFail) < roscomvpnNegativeCache {
		if entry.value != "" {
			return entry.value
		}
		return custom
	}

	mu := roscomvpnLockFor(cacheKey)
	mu.Lock()
	defer mu.Unlock()

	// Re-check after acquiring the lock: another goroutine may have just
	// refreshed this exact key while we were waiting.
	roscomvpnMu.RLock()
	entry, hit = roscomvpnCache[cacheKey]
	roscomvpnMu.RUnlock()
	if hit && time.Since(entry.fetchedAt) < roscomvpnCacheTTL {
		return entry.value
	}

	if v, err := fetchRoscomVPNDeepLink(url); err == nil {
		roscomvpnMu.Lock()
		roscomvpnCache[cacheKey] = roscomvpnCacheEntry{value: v, fetchedAt: time.Now()}
		roscomvpnMu.Unlock()
		return v
	}

	roscomvpnMu.Lock()
	prev := roscomvpnCache[cacheKey]
	prev.lastFail = time.Now()
	roscomvpnCache[cacheKey] = prev
	roscomvpnMu.Unlock()

	// Serve the last known-good value across a transient upstream outage
	// rather than dropping routing rules from every subscription response.
	if hit && entry.value != "" {
		return entry.value
	}
	return custom
}

func fetchRoscomVPNDeepLink(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), roscomvpnHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := roscomvpnClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("roscomvpn deeplink fetch failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, roscomvpnMaxBodyBytes))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
