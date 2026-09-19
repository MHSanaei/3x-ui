package sub

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// External subscription fetching: a "subscription" link is a remote URL whose
// body lists share links. A dead provider must never stall a client's sub.

const (
	subscriptionCacheTTL       = 5 * time.Minute
	subscriptionMaxBytes       = 2 << 20 // 2 MiB
	subscriptionCacheCapacity  = 256
	subscriptionDefaultUA      = "v2rayNG/1.8.5"
	subscriptionHostConcurrent = 4
	subscriptionNegativeMin    = 30 * time.Second
	subscriptionNegativeMax    = 5 * time.Minute
)

var subscriptionHTTPClient = &http.Client{Timeout: 6 * time.Second}

// subscriptionRequest is one fetch: the URL plus the identity that decides both
// the cache key and the request headers, so two identities never share a slot.
type subscriptionRequest struct {
	URL       string
	UserAgent string
	Headers   map[string]string
	CacheTTL  time.Duration
}

func (r subscriptionRequest) ttl() time.Duration {
	if r.CacheTTL > 0 {
		return r.CacheTTL
	}
	return subscriptionCacheTTL
}

func (r subscriptionRequest) userAgent() string {
	if ua := strings.TrimSpace(r.UserAgent); ua != "" {
		return ua
	}
	return subscriptionDefaultUA
}

// cacheKey is the URL plus the request identity: same URL, different User-Agent
// or custom header, different entry.
func (r subscriptionRequest) cacheKey() string {
	if len(r.Headers) == 0 && strings.TrimSpace(r.UserAgent) == "" {
		return r.URL
	}
	keys := make([]string, 0, len(r.Headers))
	for key := range r.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var identity strings.Builder
	identity.WriteString(strings.ToLower(r.userAgent()))
	for _, key := range keys {
		identity.WriteString("\n")
		identity.WriteString(strings.ToLower(key))
		identity.WriteString(": ")
		identity.WriteString(strings.TrimSpace(r.Headers[key]))
	}
	return r.URL + "\x00" + identity.String()
}

// jitteredTTL spreads refreshes so several panels behind one provider do not
// revalidate in lockstep.
func (r subscriptionRequest) jitteredTTL() time.Duration {
	ttl := r.ttl()
	return ttl + time.Duration(rand.Int64N(int64(ttl/10)+1))
}

type subscriptionCacheEntry struct {
	links        []string
	fetchedAt    time.Time
	refreshAt    time.Time
	etag         string
	lastModified string
	failures     int
	retryAt      time.Time
}

// subscriptionEntryFresh reports whether the cached body is still servable
// without a refresh. An entry with no refreshAt falls back to the plain TTL.
func subscriptionEntryFresh(entry subscriptionCacheEntry, req subscriptionRequest, now time.Time) bool {
	if entry.refreshAt.IsZero() {
		return now.Sub(entry.fetchedAt) < req.ttl()
	}
	return now.Before(entry.refreshAt)
}

type subscriptionFetch struct {
	done  chan struct{}
	links []string
}

var subscriptionCache = struct {
	sync.Mutex
	m          map[string]subscriptionCacheEntry
	inflight   map[string]*subscriptionFetch
	refreshing map[string]struct{}
	hosts      map[string]chan struct{}
}{
	m:          make(map[string]subscriptionCacheEntry),
	inflight:   make(map[string]*subscriptionFetch),
	refreshing: make(map[string]struct{}),
	hosts:      make(map[string]chan struct{}),
}

// subscriptionFetchResult reports whether this caller performed the network
// fetch: cache hits stay read-only and never re-record the outcome.
type subscriptionFetchResult struct {
	links   []string
	fetched bool
	err     error
}

// fetchSubscriptionLinks returns the share links a remote subscription URL
// holds, falling back to the last good body — never failing the whole sub.
func fetchSubscriptionLinks(rawURL string) subscriptionFetchResult {
	return fetchSubscriptionLinksFor(subscriptionRequest{URL: rawURL})
}

func fetchSubscriptionLinksFor(req subscriptionRequest) subscriptionFetchResult {
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		return subscriptionFetchResult{}
	}
	key := req.cacheKey()
	now := time.Now()

	subscriptionCache.Lock()
	entry, cached := subscriptionCache.m[key]
	if cached && subscriptionEntryFresh(entry, req, now) {
		subscriptionCache.Unlock()
		return subscriptionFetchResult{links: entry.links}
	}
	if cached && !entry.retryAt.IsZero() && now.Before(entry.retryAt) {
		// Negative cache: a provider that just failed is not retried per client.
		subscriptionCache.Unlock()
		return subscriptionFetchResult{links: entry.links}
	}
	if fetch, waiting := subscriptionCache.inflight[key]; waiting {
		subscriptionCache.Unlock()
		<-fetch.done
		return subscriptionFetchResult{links: fetch.links}
	}
	if cached {
		_, refreshing := subscriptionCache.refreshing[key]
		if !refreshing {
			subscriptionCache.refreshing[key] = struct{}{}
		}
		subscriptionCache.Unlock()
		if !refreshing {
			// Stale-while-revalidate: answer from cache now, refresh behind it.
			go func() {
				defer func() {
					subscriptionCache.Lock()
					delete(subscriptionCache.refreshing, key)
					subscriptionCache.Unlock()
				}()
				// Only refreshes nobody waits on are host-gated: a client that
				// waits must not queue behind them.
				release := acquireSubscriptionHost(req.URL)
				_, _ = refreshSubscription(req, key)
				release()
			}()
		}
		return subscriptionFetchResult{links: entry.links}
	}

	fetch := &subscriptionFetch{done: make(chan struct{})}
	subscriptionCache.inflight[key] = fetch
	subscriptionCache.Unlock()

	links, err := refreshSubscription(req, key)
	subscriptionCache.Lock()
	fetch.links = links
	close(fetch.done)
	delete(subscriptionCache.inflight, key)
	subscriptionCache.Unlock()
	if err != nil {
		return subscriptionFetchResult{fetched: true, err: err}
	}
	return subscriptionFetchResult{links: links, fetched: true}
}

// RefreshExternalSubscription fetches one URL again on the operator's command:
// forgetting the entry first is the point, or the cache answers instead.
func RefreshExternalSubscription(rawURL, userAgent string, headers map[string]string, ttlSeconds int) (int, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return 0, nil
	}
	forgetSubscriptionURL(rawURL)
	req := subscriptionRequest{URL: rawURL, UserAgent: userAgent, Headers: headers}
	if ttlSeconds > 0 {
		req.CacheTTL = time.Duration(ttlSeconds) * time.Second
	}
	res := fetchSubscriptionLinksFor(req)
	return len(res.links), res.err
}

// forgetSubscriptionURL drops every cached identity of one URL; the key is the
// URL plus the request identity, so it is matched by prefix.
func forgetSubscriptionURL(rawURL string) {
	subscriptionCache.Lock()
	defer subscriptionCache.Unlock()
	for key := range subscriptionCache.m {
		if key == rawURL || strings.HasPrefix(key, rawURL+"\x00") {
			delete(subscriptionCache.m, key)
		}
	}
}

// refreshSubscription performs one network fetch and stores its outcome. The
// last good body survives an error, so a provider outage never empties a sub.
func refreshSubscription(req subscriptionRequest, key string) ([]string, error) {
	subscriptionCache.Lock()
	cached, ok := subscriptionCache.m[key]
	existing := subscriptionCacheEntry{}
	if ok {
		existing = cached
	}
	subscriptionCache.Unlock()

	links, meta, err := doFetchSubscriptionLinks(req, existing)

	now := time.Now()
	subscriptionCache.Lock()
	entry := existing
	switch {
	case err == nil:
		entry.links = links
		entry.fetchedAt = now
		entry.refreshAt = now.Add(req.jitteredTTL())
		entry.etag = meta.etag
		entry.lastModified = meta.lastModified
		entry.failures = 0
		entry.retryAt = time.Time{}
	case errors.Is(err, errNotModified):
		entry.fetchedAt = now
		entry.refreshAt = now.Add(req.jitteredTTL())
		entry.failures = 0
		entry.retryAt = time.Time{}
	default:
		entry.failures++
		entry.retryAt = now.Add(subscriptionNegativeBackoff(entry.failures, meta.retryAfter))
		if ok {
			// Keep the served body and push the next attempt out.
			entry.links = existing.links
			entry.fetchedAt = existing.fetchedAt
		}
	}
	subscriptionCache.m[key] = entry
	trimSubscriptionCacheLocked(key)
	subscriptionCache.Unlock()

	recordSubscriptionFetchStatus(req.URL, links, err)
	return entry.links, err
}

// recordSubscriptionFetchStatus stamps every library row holding this URL, so
// the operator sees the provider's health where the link is managed.
func recordSubscriptionFetchStatus(rawURL string, links []string, err error) {
	if err != nil && errors.Is(err, errNotModified) {
		if recordErr := service.RecordExternalLinkFetchByValue(rawURL, nil, nil); recordErr != nil {
			logFetchStatusError(rawURL, recordErr)
		}
		return
	}
	if recordErr := service.RecordExternalLinkFetchByValue(rawURL, links, err); recordErr != nil {
		logFetchStatusError(rawURL, recordErr)
	}
}

func acquireSubscriptionHost(rawURL string) func() {
	parsed, err := url.Parse(rawURL)
	host := "unknown"
	if err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	subscriptionCache.Lock()
	gate, ok := subscriptionCache.hosts[host]
	if !ok {
		gate = make(chan struct{}, subscriptionHostConcurrent)
		subscriptionCache.hosts[host] = gate
	}
	subscriptionCache.Unlock()
	gate <- struct{}{}
	return func() { <-gate }
}

// subscriptionNegativeBackoff widens the gap between retries of a failing
// provider, honouring its own Retry-After when it sends one.
func subscriptionNegativeBackoff(failures int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, 30*time.Minute)
	}
	wait := subscriptionNegativeMin
	for i := 1; i < failures && wait < subscriptionNegativeMax; i++ {
		wait *= 2
	}
	return min(wait, subscriptionNegativeMax)
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}

func trimSubscriptionCacheLocked(keep string) {
	for len(subscriptionCache.m) > subscriptionCacheCapacity {
		var oldestKey string
		var oldest time.Time
		for key, entry := range subscriptionCache.m {
			if key == keep {
				continue
			}
			if oldestKey == "" || entry.fetchedAt.Before(oldest) {
				oldestKey = key
				oldest = entry.fetchedAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(subscriptionCache.m, oldestKey)
	}
}

// applySubscriptionIdentityHeaders sends the device identity the XTLS standard
// defines, so a provider gating on it answers instead of returning a bare 404.
func applySubscriptionIdentityHeaders(header http.Header, custom map[string]string) {
	if hwid := serverHwid(); hwid != "" {
		header.Set("X-HWID", hwid)
	}
	header.Set("X-Device-Os", "Linux")
	for key, value := range custom {
		header.Set(key, value)
	}
}

func logFetchStatusError(rawURL string, err error) {
	logger.Warningf("sub: recording fetch status for external subscription %q: %v", rawURL, err)
}

func doFetchSubscriptionLinks(req subscriptionRequest, cached subscriptionCacheEntry) ([]string, subscriptionFetchMeta, error) {
	meta := subscriptionFetchMeta{}
	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, meta, err
	}
	httpReq.Header.Set("User-Agent", req.userAgent())
	applySubscriptionIdentityHeaders(httpReq.Header, req.Headers)
	if cached.etag != "" {
		httpReq.Header.Set("If-None-Match", cached.etag)
	}
	if cached.lastModified != "" {
		httpReq.Header.Set("If-Modified-Since", cached.lastModified)
	}
	resp, err := subscriptionHTTPClient.Do(httpReq)
	if err != nil {
		return nil, meta, err
	}
	defer resp.Body.Close()
	meta.etag = resp.Header.Get("ETag")
	meta.lastModified = resp.Header.Get("Last-Modified")
	meta.retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	if resp.StatusCode == http.StatusNotModified {
		return nil, meta, errNotModified
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, meta, errBadStatus
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionMaxBytes+1))
	if err != nil {
		return nil, meta, err
	}
	if len(body) > subscriptionMaxBytes {
		return nil, meta, errSubscriptionBodyTooLarge
	}
	return decodeSubscriptionBody(body), meta, nil
}

type subscriptionFetchMeta struct {
	etag         string
	lastModified string
	retryAfter   time.Duration
}

var (
	errBadStatus                = &subError{"non-2xx subscription response"}
	errNotModified              = &subError{"subscription not modified"}
	errSubscriptionBodyTooLarge = &subError{"subscription response body exceeds size limit"}
)

// serverHwidKey is the settings row holding this panel's stable identity
// for outbound external-subscription fetches.
const serverHwidKey = "externalSubHwid"

// serverHwidMu serializes first-time creation: without it, concurrent first
// fetches of different URLs each mint and persist their own UUID.
var serverHwidMu sync.Mutex

// serverHwid returns a stable per-installation id, creating and persisting
// it on first use. Empty means the DB is unreachable: send no header then.
func serverHwid() string {
	serverHwidMu.Lock()
	defer serverHwidMu.Unlock()
	db := database.GetDB()
	if db == nil {
		return ""
	}
	var row model.Setting
	if err := db.Where("key = ?", serverHwidKey).First(&row).Error; err == nil {
		if strings.TrimSpace(row.Value) != "" {
			return strings.TrimSpace(row.Value)
		}
	}
	hwid := "3x-ui-server-" + uuid.NewString()
	row = model.Setting{Key: serverHwidKey, Value: hwid}
	if err := db.Where(model.Setting{Key: serverHwidKey}).FirstOrCreate(&row).Error; err != nil {
		logger.Warningf("sub: persisting server hwid failed: %v", err)
		return ""
	}
	if strings.TrimSpace(row.Value) == "" {
		return hwid
	}
	return strings.TrimSpace(row.Value)
}

type subError struct{ msg string }

func (e *subError) Error() string { return e.msg }

// decodeSubscriptionBody handles the common base64-encoded newline list as well
// as a plain-text body, returning only the lines that look like share links.
func decodeSubscriptionBody(body []byte) []string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil
	}
	if decoded, ok := tryDecodeBase64Body(text); ok {
		text = strings.TrimSpace(decoded)
	}
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.Contains(ln, "://") {
			out = append(out, ln)
		}
	}
	return out
}

func tryDecodeBase64Body(s string) (string, bool) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		}
		return r
	}, s)
	if b, err := base64.StdEncoding.DecodeString(padBase64Sub(clean)); err == nil {
		return string(b), true
	}
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(clean, "=")); err == nil {
		return string(b), true
	}
	return "", false
}
