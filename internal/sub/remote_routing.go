package sub

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

// Remote sources reuse the existing settings fields (one HTTPS URL = remote,
// else inline) so no second mode toggle can disagree with the field contents.

type remoteRoutingKind string

const (
	remoteRoutingHapp  remoteRoutingKind = "happ"
	remoteRoutingClash remoteRoutingKind = "clash"

	remoteRoutingCacheTTL     = 10 * time.Minute
	remoteRoutingRetryDelay   = 30 * time.Second
	remoteRoutingHTTPTimeout  = 6 * time.Second
	remoteRoutingHappMaxBody  = 16 << 10 // 16 KiB; Happ emits the result in a response header
	remoteRoutingHappMaxValue = 8 << 10  // normalized Routing header value
	remoteRoutingClashMaxBody = 2 << 20  // 2 MiB
)

var errRemoteRoutingUnavailable = errors.New("remote routing source is temporarily unavailable")

type remoteRoutingKey struct {
	kind   remoteRoutingKind
	source string
}

type remoteRoutingCacheEntry struct {
	Source       string         `json:"source"`
	Content      string         `json:"content"`
	FetchedAt    int64          `json:"fetchedAt"`
	ETag         string         `json:"etag,omitempty"`
	LastModified string         `json:"lastModified,omitempty"`
	Clash        map[string]any `json:"-"`
}

func (e remoteRoutingCacheEntry) fetchedTime() time.Time {
	return time.Unix(e.FetchedAt, 0)
}

type remoteRoutingFetch struct {
	done chan struct{}
	err  error
}

type remoteRoutingResolver struct {
	mu           sync.Mutex
	loadMu       sync.Mutex
	loaded       bool
	loadInFlight bool
	entries      map[remoteRoutingKey]remoteRoutingCacheEntry
	inflight     map[remoteRoutingKey]*remoteRoutingFetch
	lastAttempt  map[remoteRoutingKey]time.Time
	client       *http.Client
	now          func() time.Time
	persist      bool
}

func newRemoteRoutingResolver(client *http.Client, persist bool) *remoteRoutingResolver {
	return &remoteRoutingResolver{
		entries:     make(map[remoteRoutingKey]remoteRoutingCacheEntry),
		inflight:    make(map[remoteRoutingKey]*remoteRoutingFetch),
		lastAttempt: make(map[remoteRoutingKey]time.Time),
		client:      client,
		now:         time.Now,
		persist:     persist,
	}
}

var routingSourceResolver = newRemoteRoutingResolver(newRemoteRoutingHTTPClient(), true)

// resolveRoutingSource serves a remote source from the validated cache without
// ever blocking on network; inline values pass through (bool reports remote).
func resolveRoutingSource(kind remoteRoutingKind, raw string) (string, bool, error) {
	return routingSourceResolver.resolve(kind, raw)
}

func (r *remoteRoutingResolver) resolve(kind remoteRoutingKind, raw string) (string, bool, error) {
	entry, remote, err := r.resolveEntry(kind, raw)
	if !remote {
		return raw, false, err
	}
	return entry.Content, true, err
}

func resolveClashRoutingSource(raw string) (string, map[string]any, bool, error) {
	entry, remote, err := routingSourceResolver.resolveEntry(remoteRoutingClash, raw)
	if !remote {
		return raw, nil, false, err
	}
	return entry.Content, entry.Clash, true, err
}

func (r *remoteRoutingResolver) resolveEntry(kind remoteRoutingKind, raw string) (remoteRoutingCacheEntry, bool, error) {
	source, remote, err := common.ParseRemoteRoutingURL(raw)
	if err != nil {
		return remoteRoutingCacheEntry{}, true, err
	}
	if !remote {
		return remoteRoutingCacheEntry{}, false, nil
	}

	r.triggerPersistedLoad()

	key := remoteRoutingKey{kind: kind, source: source}
	now := r.now()

	r.mu.Lock()
	cached, hasCached := r.entries[key]
	if hasCached && now.Sub(cached.fetchedTime()) < remoteRoutingCacheTTL {
		r.mu.Unlock()
		return cached, true, nil
	}

	if _, ok := r.inflight[key]; ok {
		r.mu.Unlock()
		if hasCached {
			return cached, true, nil
		}
		return remoteRoutingCacheEntry{}, true, errRemoteRoutingUnavailable
	}

	if attemptedAt, attempted := r.lastAttempt[key]; attempted && now.Sub(attemptedAt) < remoteRoutingRetryDelay {
		r.mu.Unlock()
		if hasCached {
			return cached, true, nil
		}
		return remoteRoutingCacheEntry{}, true, errRemoteRoutingUnavailable
	}

	fetch := &remoteRoutingFetch{done: make(chan struct{})}
	r.inflight[key] = fetch
	r.mu.Unlock()

	common.GoRecover("remote-routing-refresh", func() { r.refresh(key, cached, hasCached, fetch) })
	if hasCached {
		return cached, true, nil
	}
	return remoteRoutingCacheEntry{}, true, errRemoteRoutingUnavailable
}

// RefreshRemoteRoutingSources warms and refreshes configured remote sources
// from the cron job. Concurrent resolver reads are safe; fetches coalesce.
func RefreshRemoteRoutingSources(happ, clash string) {
	for kind, raw := range map[remoteRoutingKind]string{
		remoteRoutingHapp:  happ,
		remoteRoutingClash: clash,
	} {
		_, remote, parseErr := common.ParseRemoteRoutingURL(raw)
		if parseErr != nil {
			logger.Warningf("Remote %s routing source is invalid", kind)
			continue
		}
		if remote {
			_ = routingSourceResolver.refreshSource(kind, raw)
		}
	}
}

func (r *remoteRoutingResolver) refreshSource(kind remoteRoutingKind, raw string) error {
	source, remote, err := common.ParseRemoteRoutingURL(raw)
	if err != nil || !remote {
		return err
	}
	r.ensurePersistedLoaded()

	key := remoteRoutingKey{kind: kind, source: source}
	now := r.now()
	r.mu.Lock()
	previous, hasPrevious := r.entries[key]
	if hasPrevious && now.Sub(previous.fetchedTime()) < remoteRoutingCacheTTL {
		r.mu.Unlock()
		return nil
	}
	if fetch, ok := r.inflight[key]; ok {
		done := fetch.done
		r.mu.Unlock()
		<-done
		return fetch.err
	}
	if attemptedAt, attempted := r.lastAttempt[key]; attempted && now.Sub(attemptedAt) < remoteRoutingRetryDelay {
		r.mu.Unlock()
		return errRemoteRoutingUnavailable
	}
	fetch := &remoteRoutingFetch{done: make(chan struct{})}
	r.inflight[key] = fetch
	r.mu.Unlock()

	r.refresh(key, previous, hasPrevious, fetch)
	return fetch.err
}

func (r *remoteRoutingResolver) refresh(key remoteRoutingKey, previous remoteRoutingCacheEntry, hasPrevious bool, fetch *remoteRoutingFetch) {
	entry, err := r.fetch(key, previous, hasPrevious)
	now := r.now()

	r.mu.Lock()
	r.lastAttempt[key] = now
	if err == nil {
		r.entries[key] = entry
	}
	fetch.err = err
	delete(r.inflight, key)
	close(fetch.done)
	r.mu.Unlock()

	if err != nil {
		if hasPrevious {
			logger.Warningf("Remote %s routing refresh from %s failed; keeping the last valid value", key.kind, remoteRoutingHost(key.source))
		} else {
			logger.Warningf("Remote %s routing refresh from %s failed; no validated value is cached", key.kind, remoteRoutingHost(key.source))
		}
		return
	}
	if r.persist {
		r.persistEntry(key.kind, entry)
	}
}

func (r *remoteRoutingResolver) fetch(key remoteRoutingKey, previous remoteRoutingCacheEntry, hasPrevious bool) (entry remoteRoutingCacheEntry, err error) {
	// Remote bytes reach the YAML/JSON parsers below; a parser panic must
	// degrade to a failed refresh (keeping last-good), not crash the panel.
	defer func() {
		if panicValue := recover(); panicValue != nil {
			entry, err = remoteRoutingCacheEntry{}, fmt.Errorf("remote routing fetch panicked: %v", panicValue)
		}
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, key.source, nil)
	if err != nil {
		return remoteRoutingCacheEntry{}, err
	}
	req.Header.Set("User-Agent", "3x-ui-remote-routing/1.0")
	if hasPrevious {
		if previous.ETag != "" {
			req.Header.Set("If-None-Match", previous.ETag)
		}
		if previous.LastModified != "" {
			req.Header.Set("If-Modified-Since", previous.LastModified)
		}
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return remoteRoutingCacheEntry{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if !hasPrevious {
			return remoteRoutingCacheEntry{}, errors.New("remote source returned 304 without a cached value")
		}
		previous.FetchedAt = r.now().Unix()
		if etag := strings.TrimSpace(resp.Header.Get("ETag")); etag != "" {
			previous.ETag = etag
		}
		if modified := strings.TrimSpace(resp.Header.Get("Last-Modified")); modified != "" {
			previous.LastModified = modified
		}
		return previous, nil
	}
	if key.kind == remoteRoutingHapp && isRemoteHappRedirect(resp.StatusCode) {
		location := strings.TrimSpace(resp.Header.Get("Location"))
		content, locationErr := normalizeHappRouting([]byte(location))
		if locationErr != nil {
			return remoteRoutingCacheEntry{}, fmt.Errorf("invalid Happ redirect target: %w", locationErr)
		}
		if len(content) > remoteRoutingHappMaxValue {
			return remoteRoutingCacheEntry{}, errors.New("Happ routing header exceeds the size limit")
		}
		return remoteRoutingCacheEntry{
			Source:    key.source,
			Content:   content,
			FetchedAt: r.now().Unix(),
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return remoteRoutingCacheEntry{}, fmt.Errorf("remote source returned HTTP %d", resp.StatusCode)
	}

	limit := int64(remoteRoutingHappMaxBody)
	if key.kind == remoteRoutingClash {
		limit = remoteRoutingClashMaxBody
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return remoteRoutingCacheEntry{}, err
	}
	if int64(len(body)) > limit {
		return remoteRoutingCacheEntry{}, errors.New("remote routing response exceeds the size limit")
	}

	content, clash, err := normalizeRemoteRoutingContent(key.kind, body)
	if err != nil {
		return remoteRoutingCacheEntry{}, err
	}
	if key.kind == remoteRoutingHapp && len(content) > remoteRoutingHappMaxValue {
		return remoteRoutingCacheEntry{}, errors.New("Happ routing header exceeds the size limit")
	}
	return remoteRoutingCacheEntry{
		Source:       key.source,
		Content:      content,
		FetchedAt:    r.now().Unix(),
		ETag:         strings.TrimSpace(resp.Header.Get("ETag")),
		LastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
		Clash:        clash,
	}, nil
}

func isRemoteHappRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func normalizeRemoteRoutingContent(kind remoteRoutingKind, body []byte) (string, map[string]any, error) {
	switch kind {
	case remoteRoutingHapp:
		content, err := normalizeHappRouting(body)
		return content, nil, err
	case remoteRoutingClash:
		return normalizeClashRouting(body)
	default:
		return "", nil, fmt.Errorf("unsupported remote routing kind %q", kind)
	}
}

func normalizeHappRouting(body []byte) (string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", errors.New("empty Happ routing response")
	}

	if strings.HasPrefix(text, "{") {
		compact, err := validateAndCompactJSONObject([]byte(text))
		if err != nil {
			return "", fmt.Errorf("invalid Happ routing JSON: %w", err)
		}
		return "happ://routing/onadd/" + base64.StdEncoding.EncodeToString(compact), nil
	}
	if strings.ContainsAny(text, "\r\n") {
		return "", errors.New("Happ deeplink must be a single line")
	}

	payload := ""
	for _, prefix := range []string{"happ://routing/onadd/", "happ://routing/add/"} {
		if strings.HasPrefix(text, prefix) {
			payload = strings.TrimPrefix(text, prefix)
			break
		}
	}
	if payload == "" {
		return "", errors.New("Happ response is neither routing JSON nor a routing deeplink")
	}
	decoded, err := decodeRoutingBase64(payload)
	if err != nil {
		return "", fmt.Errorf("invalid Happ routing payload: %w", err)
	}
	if _, err := validateAndCompactJSONObject(decoded); err != nil {
		return "", fmt.Errorf("invalid Happ routing payload JSON: %w", err)
	}
	return text, nil
}

func validateAndCompactJSONObject(raw []byte) ([]byte, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, errors.New("expected a JSON object")
	}
	return json.Marshal(object)
}

func decodeRoutingBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func normalizeClashRouting(body []byte) (string, map[string]any, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", nil, errors.New("empty Clash routing response")
	}
	var document map[string]any
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		return "", nil, fmt.Errorf("invalid Clash routing YAML: %w", err)
	}
	if len(document) == 0 {
		return "", nil, errors.New("Clash routing response must be a YAML map")
	}
	hasSupportedKey := false
	for key := range document {
		if remoteClashAllowedKey(key) {
			hasSupportedKey = true
			break
		}
	}
	if !hasSupportedKey {
		return "", nil, errors.New("Clash routing response has no supported routing keys")
	}
	base := map[string]any{
		"proxies": []map[string]any{{"name": "validation-node", "type": "vless"}},
		"proxy-groups": []map[string]any{{
			"name": "PROXY", "type": "select", "proxies": []string{"validation-node", "DIRECT"},
		}},
		"rules": []string{"MATCH,PROXY"},
	}
	if err := mergeRemoteClashRules(base, document); err != nil {
		return "", nil, fmt.Errorf("invalid remote Clash routing schema: %w", err)
	}
	return text, document, nil
}

func resolveIncyRoutingSource(raw string) (string, bool, error) {
	source, remote, err := common.ParseRemoteRoutingURL(raw)
	if err != nil || !remote {
		return raw, remote, err
	}
	return "incy://autorouting/onadd/" + source, true, nil
}

func newRemoteRoutingHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           netsafe.SSRFGuardedDialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   4 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Timeout:       remoteRoutingHTTPTimeout,
		Transport:     transport,
		CheckRedirect: checkRemoteRoutingRedirect,
	}
}

func checkRemoteRoutingRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("stopped after 5 redirects")
	}
	if strings.EqualFold(req.URL.Scheme, "happ") {
		// routing.help-style services publish the deeplink as the final Location;
		// hand the 3xx back to fetch(), which validates it without a request.
		return http.ErrUseLastResponse
	}
	if !strings.EqualFold(req.URL.Scheme, "https") || req.URL.Hostname() == "" || req.URL.User != nil {
		return errors.New("remote routing redirect must stay on an absolute HTTPS URL")
	}
	// The guarded dialer re-resolves, validates and connects to the same public
	// address, including on every HTTPS redirect hop.
	return nil
}

func remoteRoutingHost(source string) string {
	u, err := url.Parse(source)
	if err != nil || u.Hostname() == "" {
		return "unknown host"
	}
	return u.Hostname()
}

func remoteRoutingSettingKey(kind remoteRoutingKind) string {
	return "_subRemoteRoutingCache_" + string(kind)
}

func (r *remoteRoutingResolver) ensurePersistedLoaded() {
	if !r.persist {
		return
	}
	r.mu.Lock()
	loaded := r.loaded
	r.mu.Unlock()
	if loaded {
		return
	}

	r.loadMu.Lock()
	defer r.loadMu.Unlock()
	r.mu.Lock()
	loaded = r.loaded
	r.mu.Unlock()
	if loaded {
		return
	}
	db := database.GetDB()
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		return
	}
	r.loadPersisted()
	r.mu.Lock()
	r.loaded = true
	r.mu.Unlock()
}

// triggerPersistedLoad keeps SQLite off the subscription request path: requests
// schedule at most one background load; the startup job loads synchronously.
func (r *remoteRoutingResolver) triggerPersistedLoad() {
	if !r.persist {
		return
	}
	r.mu.Lock()
	if r.loaded || r.loadInFlight {
		r.mu.Unlock()
		return
	}
	r.loadInFlight = true
	r.mu.Unlock()

	common.GoRecover("remote-routing-cache-load", func() {
		defer func() {
			r.mu.Lock()
			r.loadInFlight = false
			r.mu.Unlock()
		}()
		r.ensurePersistedLoaded()
	})
}

func (r *remoteRoutingResolver) loadPersisted() {
	loaded := make(map[remoteRoutingKey]remoteRoutingCacheEntry, 2)
	for _, kind := range []remoteRoutingKind{remoteRoutingHapp, remoteRoutingClash} {
		var setting model.Setting
		err := database.GetDB().Where("key = ?", remoteRoutingSettingKey(kind)).First(&setting).Error
		if err != nil {
			continue
		}
		var entry remoteRoutingCacheEntry
		if json.Unmarshal([]byte(setting.Value), &entry) != nil || entry.Source == "" || entry.Content == "" || entry.FetchedAt <= 0 {
			continue
		}
		if _, remote, err := common.ParseRemoteRoutingURL(entry.Source); err != nil || !remote {
			continue
		}
		normalized, clash, err := normalizeRemoteRoutingContent(kind, []byte(entry.Content))
		if err != nil {
			continue
		}
		if kind == remoteRoutingHapp && len(normalized) > remoteRoutingHappMaxValue {
			continue
		}
		entry.Content = normalized
		entry.Clash = clash
		loaded[remoteRoutingKey{kind: kind, source: entry.Source}] = entry
	}
	r.mu.Lock()
	for key, entry := range loaded {
		current, exists := r.entries[key]
		if !exists || entry.FetchedAt > current.FetchedAt {
			r.entries[key] = entry
		}
	}
	r.mu.Unlock()
}

func (r *remoteRoutingResolver) persistEntry(kind remoteRoutingKind, entry remoteRoutingCacheEntry) {
	db := database.GetDB()
	if db == nil {
		return
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	key := remoteRoutingSettingKey(kind)
	var setting model.Setting
	err = db.Where("key = ?", key).First(&setting).Error
	if database.IsNotFound(err) {
		err = db.Create(&model.Setting{Key: key, Value: string(encoded)}).Error
	} else if err == nil {
		setting.Value = string(encoded)
		err = db.Save(&setting).Error
	}
	if err != nil {
		logger.Warningf("Could not persist the last valid %s remote routing value", kind)
	}
}
