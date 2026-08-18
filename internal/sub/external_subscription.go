package sub

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// External subscription fetching: a "subscription" external link is a remote
// URL whose body is a (often base64-encoded) newline list of share links. We
// fetch it on demand, cache the decoded links briefly, and bound the request
// with a short timeout so a slow/dead provider can't stall a client's sub.

const (
	subscriptionCacheTTL      = 5 * time.Minute
	subscriptionMaxBytes      = 2 << 20 // 2 MiB
	subscriptionCacheCapacity = 256
)

var subscriptionHTTPClient = &http.Client{Timeout: 6 * time.Second}

type subscriptionCacheEntry struct {
	links     []string
	fetchedAt time.Time
}

type subscriptionFetch struct {
	done  chan struct{}
	links []string
}

var subscriptionCache = struct {
	sync.Mutex
	m        map[string]subscriptionCacheEntry
	inflight map[string]*subscriptionFetch
}{
	m:        make(map[string]subscriptionCacheEntry),
	inflight: make(map[string]*subscriptionFetch),
}

// subscriptionFetchResult reports whether this caller performed the network
// fetch, so only it records status and cache hits stay read-only.
type subscriptionFetchResult struct {
	links   []string
	fetched bool
	err     error
}

// fetchSubscriptionLinks returns the share links contained in a remote
// subscription URL, using a short-lived cache. On any failure it returns the
// last cached value (if present) or nil — never an error, so the rest of the
// client's subscription still renders.
func fetchSubscriptionLinks(rawURL string) subscriptionFetchResult {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return subscriptionFetchResult{}
	}

	subscriptionCache.Lock()
	cached, ok := subscriptionCache.m[rawURL]
	if ok && time.Since(cached.fetchedAt) < subscriptionCacheTTL {
		subscriptionCache.Unlock()
		return subscriptionFetchResult{links: cached.links}
	}
	if fetch, waiting := subscriptionCache.inflight[rawURL]; waiting {
		subscriptionCache.Unlock()
		<-fetch.done
		return subscriptionFetchResult{links: fetch.links}
	}
	fetch := &subscriptionFetch{done: make(chan struct{})}
	subscriptionCache.inflight[rawURL] = fetch
	subscriptionCache.Unlock()
	defer func() {
		subscriptionCache.Lock()
		close(fetch.done)
		delete(subscriptionCache.inflight, rawURL)
		subscriptionCache.Unlock()
	}()

	links, err := doFetchSubscriptionLinks(rawURL)
	if err != nil {
		if ok {
			fetch.links = cached.links
		}
		return subscriptionFetchResult{links: fetch.links, fetched: true, err: err}
	}

	subscriptionCache.Lock()
	subscriptionCache.m[rawURL] = subscriptionCacheEntry{links: links, fetchedAt: time.Now()}
	trimSubscriptionCacheLocked(rawURL)
	subscriptionCache.Unlock()
	fetch.links = links
	return subscriptionFetchResult{links: links, fetched: true}
}

func trimSubscriptionCacheLocked(keep string) {
	for len(subscriptionCache.m) > subscriptionCacheCapacity {
		var oldestURL string
		var oldest time.Time
		for rawURL, entry := range subscriptionCache.m {
			if rawURL == keep {
				continue
			}
			if oldestURL == "" || entry.fetchedAt.Before(oldest) {
				oldestURL = rawURL
				oldest = entry.fetchedAt
			}
		}
		if oldestURL == "" {
			return
		}
		delete(subscriptionCache.m, oldestURL)
	}
}

// recordExternalSubscriptionFetch stamps status on every row holding this URL,
// keyed by value because row ids churn on save and the cache is per URL.
func recordExternalSubscriptionFetch(rawURL string, fetchErr error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}
	lastFetchError := ""
	if fetchErr != nil {
		lastFetchError = fetchErr.Error()
	}
	if err := database.GetDB().
		Model(&model.ClientExternalLink{}).
		Where("kind = ? AND value = ?", model.ExternalLinkKindSubscription, rawURL).
		Updates(map[string]any{
			"last_fetch_at":    time.Now().UnixMilli(),
			"last_fetch_error": lastFetchError,
		}).Error; err != nil {
		logger.Warningf("sub: recording fetch status for external subscription %q: %v", rawURL, err)
	}
}

func doFetchSubscriptionLinks(rawURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Some providers gate the link body on a known client User-Agent.
	req.Header.Set("User-Agent", "v2rayNG/1.8.5")
	resp, err := subscriptionHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errBadStatus
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > subscriptionMaxBytes {
		return nil, errSubscriptionBodyTooLarge
	}
	return decodeSubscriptionBody(body), nil
}

var (
	errBadStatus                = &subError{"non-2xx subscription response"}
	errSubscriptionBodyTooLarge = &subError{"subscription response body exceeds size limit"}
)

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
