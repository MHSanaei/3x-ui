package sub

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// External subscription fetching: a "subscription" external link is a remote
// URL whose body is a (often base64-encoded) newline list of share links. We
// fetch it on demand, cache the decoded links briefly, and bound the request
// with a short timeout so a slow/dead provider can't stall a client's sub.

const (
	subscriptionCacheTTL = 5 * time.Minute
	subscriptionMaxBytes = 2 << 20 // 2 MiB
)

var subscriptionHTTPClient = &http.Client{Timeout: 6 * time.Second}

type subscriptionCacheEntry struct {
	links     []string
	fetchedAt time.Time
}

var subscriptionCache = struct {
	sync.Mutex
	m map[string]subscriptionCacheEntry
}{m: make(map[string]subscriptionCacheEntry)}

// fetchSubscriptionLinks returns the share links contained in a remote
// subscription URL, using a short-lived cache. On any failure it returns the
// last cached value (if present) or nil — never an error, so the rest of the
// client's subscription still renders.
func fetchSubscriptionLinks(rawURL string) []string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}

	subscriptionCache.Lock()
	cached, ok := subscriptionCache.m[rawURL]
	subscriptionCache.Unlock()
	if ok && time.Since(cached.fetchedAt) < subscriptionCacheTTL {
		return cached.links
	}

	links, err := doFetchSubscriptionLinks(rawURL)
	if err != nil {
		// Serve stale on error rather than dropping the client's configs.
		if ok {
			return cached.links
		}
		return nil
	}

	subscriptionCache.Lock()
	subscriptionCache.m[rawURL] = subscriptionCacheEntry{links: links, fetchedAt: time.Now()}
	subscriptionCache.Unlock()
	return links
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
