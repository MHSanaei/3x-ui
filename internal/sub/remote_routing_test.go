package sub

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func mergeRemoteClashRulesYAML(base map[string]any, raw string) error {
	var remote map[string]any
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(raw)), &remote); err != nil {
		return err
	}
	return mergeRemoteClashRules(base, remote)
}

type remoteRoutingRoundTripper func(*http.Request) (*http.Response, error)

func (fn remoteRoutingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func remoteRoutingTestClient(fn remoteRoutingRoundTripper) *http.Client {
	return &http.Client{Transport: fn}
}

func remoteRoutingResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func waitRemoteRoutingIdle(t *testing.T, resolver *remoteRoutingResolver) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		resolver.mu.Lock()
		inflight := len(resolver.inflight)
		resolver.mu.Unlock()
		if inflight == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("remote routing refresh did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitRemoteRoutingLoadIdle(t *testing.T, resolver *remoteRoutingResolver) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		resolver.mu.Lock()
		loading := resolver.loadInFlight
		resolver.mu.Unlock()
		if !loading {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("persisted routing cache load did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func primeRemoteRouting(t *testing.T, resolver *remoteRoutingResolver, kind remoteRoutingKind, source string) string {
	t.Helper()
	if err := resolver.refreshSource(kind, source); err != nil {
		t.Fatalf("prime remote routing: %v", err)
	}
	value, remote, err := resolver.resolve(kind, source)
	if err != nil || !remote || value == "" {
		t.Fatalf("primed resolve got=%q remote=%v err=%v", value, remote, err)
	}
	return value
}

func TestNormalizeHappRoutingAcceptsJSONAndDeeplink(t *testing.T) {
	deeplink, err := normalizeHappRouting([]byte(`{"Name":"RoscomVPN","GlobalProxy":"true"}`))
	if err != nil {
		t.Fatalf("normalize JSON: %v", err)
	}
	const prefix = "happ://routing/onadd/"
	if !strings.HasPrefix(deeplink, prefix) {
		t.Fatalf("deeplink = %q", deeplink)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(deeplink, prefix))
	if err != nil || !strings.Contains(string(decoded), `"Name":"RoscomVPN"`) {
		t.Fatalf("decoded payload = %q, err=%v", decoded, err)
	}

	if got, err := normalizeHappRouting([]byte(deeplink + "\n")); err != nil || got != deeplink {
		t.Fatalf("ready deeplink got=%q err=%v", got, err)
	}
	if _, err := normalizeHappRouting([]byte("routing.help")); err == nil {
		t.Fatal("invalid Happ response was accepted")
	}
}

func TestRemoteRoutingResolverAcceptsHappRedirect(t *testing.T) {
	deeplink, err := normalizeHappRouting([]byte(`{"Name":"redirected"}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var requests atomic.Int32
	client := remoteRoutingTestClient(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		response := remoteRoutingResponse(http.StatusFound, "")
		response.Header.Set("Location", deeplink)
		response.Request = req
		return response, nil
	})
	client.CheckRedirect = checkRemoteRoutingRedirect
	resolver := newRemoteRoutingResolver(client, false)

	const source = "https://routing.example/"
	if err := resolver.refreshSource(remoteRoutingHapp, source); err != nil {
		t.Fatalf("refresh redirect: %v", err)
	}
	got, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || got != deeplink {
		t.Fatalf("redirect resolve got=%q remote=%v err=%v", got, remote, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("network requests = %d, want 1", requests.Load())
	}
}

func TestRemoteRoutingResolverHandlesHappNotModified(t *testing.T) {
	var requests atomic.Int32
	client := remoteRoutingTestClient(func(req *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			response := remoteRoutingResponse(http.StatusOK, `{"Name":"etagged"}`)
			response.Header.Set("ETag", `"v1"`)
			return response, nil
		}
		if req.Header.Get("If-None-Match") != `"v1"` {
			t.Errorf("If-None-Match = %q", req.Header.Get("If-None-Match"))
		}
		return remoteRoutingResponse(http.StatusNotModified, ""), nil
	})
	resolver := newRemoteRoutingResolver(client, false)
	now := time.Unix(1_800_000_000, 0)
	resolver.now = func() time.Time { return now }
	const source = "https://example.com/default.json"

	first := primeRemoteRouting(t, resolver, remoteRoutingHapp, source)
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	second, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || second != first {
		t.Fatalf("stale resolve got=%q err=%v", second, err)
	}
	waitRemoteRoutingIdle(t, resolver)
	now = now.Add(time.Minute)
	third, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || third != first {
		t.Fatalf("refreshed cache got=%q err=%v", third, err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestRemoteRoutingResolverDoesNotBlockAndCoalescesColdFetch(t *testing.T) {
	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	client := remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		return remoteRoutingResponse(http.StatusOK, `{"Name":"RoscomVPN"}`), nil
	})
	resolver := newRemoteRoutingResolver(client, false)
	const source = "https://example.com/default.json"

	results := make(chan error, 8)
	for range 8 {
		go func() {
			_, remote, err := resolver.resolve(remoteRoutingHapp, source)
			if !remote {
				results <- errors.New("source was not classified as remote")
				return
			}
			results <- err
		}()
	}
	<-started
	for range 8 {
		select {
		case err := <-results:
			if !errors.Is(err, errRemoteRoutingUnavailable) {
				t.Fatalf("cold resolve err=%v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("cold resolve blocked on the remote fetch")
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
	close(release)
	waitRemoteRoutingIdle(t, resolver)
	if got, _, err := resolver.resolve(remoteRoutingHapp, source); err != nil || !strings.HasPrefix(got, "happ://routing/onadd/") {
		t.Fatalf("cached resolve got=%q err=%v", got, err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cached request count = %d, want 1", got)
	}
}

func TestRemoteRoutingResolverServesStaleAfterFailedRefresh(t *testing.T) {
	var requests atomic.Int32
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var startOnce sync.Once
	fail := atomic.Bool{}
	client := remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		if fail.Load() {
			startOnce.Do(func() { close(refreshStarted) })
			<-releaseRefresh
			return remoteRoutingResponse(http.StatusBadGateway, "bad gateway"), nil
		}
		return remoteRoutingResponse(http.StatusOK, `{"Name":"last-good"}`), nil
	})
	resolver := newRemoteRoutingResolver(client, false)
	now := time.Unix(1_800_000_000, 0)
	resolver.now = func() time.Time { return now }
	const source = "https://example.com/default.json"

	first := primeRemoteRouting(t, resolver, remoteRoutingHapp, source)
	fail.Store(true)
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	startedAt := time.Now()
	stale, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || stale != first {
		t.Fatalf("stale resolve got=%q remote=%v err=%v", stale, remote, err)
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("stale resolve blocked for %v", elapsed)
	}
	select {
	case <-refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("refresh did not run")
	}
	close(releaseRefresh)

	waitRemoteRoutingIdle(t, resolver)

	if got, _, err := resolver.resolve(remoteRoutingHapp, source); err != nil || got != first {
		t.Fatalf("negative-cache resolve got=%q err=%v", got, err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRemoteRoutingResolverLoadsPersistedLastGood(t *testing.T) {
	initSubDB(t)

	deeplink, err := normalizeHappRouting([]byte(`{"Name":"persisted"}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	const source = "https://example.com/default.json"
	entry := remoteRoutingCacheEntry{
		Source: source, Content: deeplink, FetchedAt: time.Now().Add(-time.Hour).Unix(), ETag: `"v1"`,
	}
	newRemoteRoutingResolver(nil, false).persistEntry(remoteRoutingHapp, entry)

	resolver := newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusServiceUnavailable, "offline"), nil
	}), true)
	resolver.ensurePersistedLoaded()
	got, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || got != deeplink {
		t.Fatalf("persisted resolve got=%q remote=%v err=%v", got, remote, err)
	}
	waitRemoteRoutingIdle(t, resolver)
}

func TestRemoteRoutingResolverDoesNotBlockOnPersistedLoad(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	resolver := newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		startOnce.Do(func() { close(started) })
		<-release
		return remoteRoutingResponse(http.StatusServiceUnavailable, "offline"), nil
	}), true)

	resolver.loadMu.Lock()
	loadLocked := true
	t.Cleanup(func() {
		if loadLocked {
			resolver.loadMu.Unlock()
		}
	})

	startedAt := time.Now()
	_, remote, err := resolver.resolve(remoteRoutingHapp, "https://example.com/default.json")
	if !remote || !errors.Is(err, errRemoteRoutingUnavailable) {
		t.Fatalf("resolve remote=%v err=%v", remote, err)
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("resolve blocked on persisted cache load for %v", elapsed)
	}

	resolver.loadMu.Unlock()
	loadLocked = false
	close(release)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}
	waitRemoteRoutingIdle(t, resolver)
	waitRemoteRoutingLoadIdle(t, resolver)
}

func TestRemoteRoutingResolverRejectsOversizedPersistedHappValue(t *testing.T) {
	initSubDB(t)

	deeplink, err := normalizeHappRouting([]byte(`{"Name":"` + strings.Repeat("x", remoteRoutingHappMaxValue) + `"}`))
	if err != nil || len(deeplink) <= remoteRoutingHappMaxValue {
		t.Fatalf("oversized fixture length=%d err=%v", len(deeplink), err)
	}
	const source = "https://example.com/oversized.json"
	newRemoteRoutingResolver(nil, false).persistEntry(remoteRoutingHapp, remoteRoutingCacheEntry{
		Source: source, Content: deeplink, FetchedAt: time.Now().Unix(),
	})

	resolver := newRemoteRoutingResolver(nil, true)
	resolver.ensurePersistedLoaded()
	resolver.mu.Lock()
	_, exists := resolver.entries[remoteRoutingKey{kind: remoteRoutingHapp, source: source}]
	resolver.mu.Unlock()
	if exists {
		t.Fatal("oversized persisted Happ routing value was loaded")
	}
}

func TestRemoteRoutingResolverDoesNotReplaceClashCacheWithInvalidSchema(t *testing.T) {
	var requests atomic.Int32
	client := remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			return remoteRoutingResponse(http.StatusOK, "rules:\n  - MATCH,PROXY\n"), nil
		}
		return remoteRoutingResponse(http.StatusOK, "rules: not-a-list\n"), nil
	})
	resolver := newRemoteRoutingResolver(client, false)
	now := time.Unix(1_800_000_000, 0)
	resolver.now = func() time.Time { return now }
	const source = "https://example.com/routing.yaml"

	first := primeRemoteRouting(t, resolver, remoteRoutingClash, source)
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	second, _, err := resolver.resolve(remoteRoutingClash, source)
	if err != nil || second != first {
		t.Fatalf("invalid refresh replaced last-good: got=%q err=%v", second, err)
	}
	waitRemoteRoutingIdle(t, resolver)
	second, _, err = resolver.resolve(remoteRoutingClash, source)
	if err != nil || second != first {
		t.Fatalf("invalid refresh replaced last-good after completion: got=%q err=%v", second, err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestApplyCommonHeadersResolvesRemoteHappAndFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })

	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusOK, `{"Name":"RoscomVPN"}`), nil
	}), false)
	const source = "https://example.com/default.json"
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingHapp, source)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&SUBController{}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", true, source, false)
	if recorder.Header().Get("Routing-Enable") != "true" || !strings.HasPrefix(recorder.Header().Get("Routing"), "happ://routing/onadd/") {
		t.Fatalf("headers = %#v", recorder.Header())
	}

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	(&SUBController{}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", false, source, false)
	if recorder.Header().Get("Routing-Enable") != "" || !strings.HasPrefix(recorder.Header().Get("Routing"), "happ://routing/onadd/") {
		t.Fatalf("independent routing headers = %#v", recorder.Header())
	}

	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusOK, "routing.help"), nil
	}), false)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	(&SUBController{}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", true, "https://example.com/bad", false)
	if recorder.Header().Get("Routing-Enable") != "true" || recorder.Header().Get("Routing") != "" {
		t.Fatalf("invalid remote source leaked routing headers: %#v", recorder.Header())
	}
	waitRemoteRoutingIdle(t, routingSourceResolver)
}

func TestResolveIncyRemoteSourceUsesAutorouting(t *testing.T) {
	got, remote, err := resolveIncyRoutingSource("https://example.com/DEFAULT.JSON")
	if err != nil || !remote || got != "incy://autorouting/onadd/https://example.com/DEFAULT.JSON" {
		t.Fatalf("got=%q remote=%v err=%v", got, remote, err)
	}
	inline := "incy://routing/onadd/abc"
	if got, remote, err := resolveIncyRoutingSource(inline); err != nil || remote || got != inline {
		t.Fatalf("inline got=%q remote=%v err=%v", got, remote, err)
	}
}

func TestMergeRemoteClashRulesPreservesGeneratedProxies(t *testing.T) {
	originalProxy := map[string]any{"name": "vpn-node", "type": "vless"}
	base := map[string]any{
		"proxies": []map[string]any{originalProxy},
		"proxy-groups": []map[string]any{{
			"name": "PROXY", "type": "select", "proxies": []string{"vpn-node", "DIRECT"},
		}},
		"rules": []string{"MATCH,PROXY"},
	}
	remote := `
proxies:
  - name: attacker-controlled
proxy-providers:
  prov:
    url: <SUBSCRIPTION PLACEHOLDER>
external-controller: 0.0.0.0:9090
allow-lan: true
mixed-port: 7890
dns:
  enable: true
tun:
  enable: true
proxy-groups:
  - name: VPN
    type: select
    include-all: true
  - name: PROXY
    type: select
    proxies: [VPN]
rule-providers:
  roscom:
    type: http
    url: https://example.com/rules.mrs
rules:
  - RULE-SET,roscom,PROXY
  - MATCH,PROXY
`
	if err := mergeRemoteClashRulesYAML(base, remote); err != nil {
		t.Fatalf("merge: %v", err)
	}
	proxies, ok := base["proxies"].([]map[string]any)
	if !ok || len(proxies) != 1 || proxies[0]["name"] != "vpn-node" {
		t.Fatalf("generated proxies were replaced: %#v", base["proxies"])
	}
	if _, exists := base["proxy-providers"]; exists {
		t.Fatal("remote proxy-providers were imported")
	}
	if _, exists := base["external-controller"]; exists {
		t.Fatal("unsafe top-level key was imported")
	}
	for _, key := range []string{"allow-lan", "mixed-port", "dns", "tun"} {
		if _, exists := base[key]; exists {
			t.Fatalf("client-local key %q was imported", key)
		}
	}
	if _, exists := base["rule-providers"]; !exists {
		t.Fatal("rule-providers were not imported")
	}
	groups, ok := asAnySlice(base["proxy-groups"])
	if !ok || len(groups) != 2 || clashProxyGroupName(groups[0]) != "VPN" || clashProxyGroupName(groups[1]) != "PROXY" {
		t.Fatalf("proxy groups = %#v", base["proxy-groups"])
	}
	rules, ok := asAnySlice(base["rules"])
	if !ok || len(rules) != 2 || rules[1] != "MATCH,PROXY" {
		t.Fatalf("rules = %#v", base["rules"])
	}
}

func TestMergeRemoteClashRulesKeepsBaseProxyGroupWhenRemoteOmitsIt(t *testing.T) {
	base := map[string]any{
		"proxies": []map[string]any{{"name": "vpn-node", "type": "vless"}},
		"proxy-groups": []map[string]any{{
			"name": "PROXY", "type": "select", "proxies": []string{"vpn-node", "DIRECT"},
		}},
		"rules": []string{"MATCH,PROXY"},
	}
	if err := mergeRemoteClashRulesYAML(base, `proxy-groups:
  - name: Extra
    type: select
    proxies: [PROXY]
rules:
  - MATCH,PROXY
`); err != nil {
		t.Fatalf("merge: %v", err)
	}
	groups, ok := asAnySlice(base["proxy-groups"])
	if !ok || len(groups) != 2 || clashProxyGroupName(groups[0]) != "Extra" || clashProxyGroupName(groups[1]) != "PROXY" {
		t.Fatalf("proxy groups = %#v", base["proxy-groups"])
	}
}

func TestRemoteRoutingRejectsOversizedHappValues(t *testing.T) {
	largeJSON := `{"Name":"large","Rules":"` + strings.Repeat("a", remoteRoutingHappMaxValue) + `"}`
	largeDeeplink, err := normalizeHappRouting([]byte(largeJSON))
	if err != nil {
		t.Fatalf("prepare large deeplink: %v", err)
	}

	tests := []struct {
		name     string
		response func(*http.Request) *http.Response
		wantErr  string
	}{
		{
			name: "response body",
			response: func(*http.Request) *http.Response {
				return remoteRoutingResponse(http.StatusOK, strings.Repeat("x", remoteRoutingHappMaxBody+1))
			},
			wantErr: "response exceeds the size limit",
		},
		{
			name: "normalized header",
			response: func(*http.Request) *http.Response {
				return remoteRoutingResponse(http.StatusOK, largeJSON)
			},
			wantErr: "header exceeds the size limit",
		},
		{
			name: "redirect header",
			response: func(req *http.Request) *http.Response {
				response := remoteRoutingResponse(http.StatusFound, "")
				response.Header.Set("Location", largeDeeplink)
				response.Request = req
				return response
			},
			wantErr: "header exceeds the size limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := remoteRoutingTestClient(func(req *http.Request) (*http.Response, error) {
				return tt.response(req), nil
			})
			client.CheckRedirect = checkRemoteRoutingRedirect
			resolver := newRemoteRoutingResolver(client, false)
			err := resolver.refreshSource(remoteRoutingHapp, "https://example.com/rules")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestRemoteRoutingRefreshTurnsPanicsIntoErrors(t *testing.T) {
	client := remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		panic("transport exploded")
	})
	resolver := newRemoteRoutingResolver(client, false)
	err := resolver.refreshSource(remoteRoutingHapp, "https://example.com/rules")
	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("err=%v, want the panic converted into an error", err)
	}
	// The inflight slot must be released so later refreshes are not wedged.
	waitRemoteRoutingIdle(t, resolver)
}

func TestRemoteRoutingHTTPClientRejectsLoopback(t *testing.T) {
	resolver := newRemoteRoutingResolver(newRemoteRoutingHTTPClient(), false)
	startedAt := time.Now()
	err := resolver.refreshSource(remoteRoutingHapp, "https://127.0.0.1:1/rules")
	if err == nil {
		t.Fatal("loopback remote source was accepted")
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("loopback rejection took %v", elapsed)
	}
}

func TestRemoteRoutingPersistedLoadRetriesAfterDatabaseBecomesReady(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	deeplink, err := normalizeHappRouting([]byte(`{"Name":"persisted-after-ready"}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	const source = "https://example.com/default.json"
	newRemoteRoutingResolver(nil, false).persistEntry(remoteRoutingHapp, remoteRoutingCacheEntry{
		Source: source, Content: deeplink, FetchedAt: time.Now().Unix(),
	})
	if err := database.CloseDB(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	resolver := newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusServiceUnavailable, "offline"), nil
	}), true)
	if _, _, err := resolver.resolve(remoteRoutingHapp, source); !errors.Is(err, errRemoteRoutingUnavailable) {
		t.Fatalf("closed-db resolve err=%v", err)
	}
	waitRemoteRoutingIdle(t, resolver)
	waitRemoteRoutingLoadIdle(t, resolver)
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	resolver.triggerPersistedLoad()
	waitRemoteRoutingLoadIdle(t, resolver)
	got, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || got != deeplink {
		t.Fatalf("reloaded resolve got=%q remote=%v err=%v", got, remote, err)
	}
}

func TestRemoteClashRouteGraphValidation(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		wantErr string
	}{
		{
			name:    "missing group name",
			remote:  "proxy-groups:\n  - type: select\n    proxies: [vpn-node]\nrules:\n  - MATCH,PROXY\n",
			wantErr: "named group maps",
		},
		{
			name:    "duplicate group name",
			remote:  "proxy-groups:\n  - {name: A, type: select, proxies: [vpn-node]}\n  - {name: A, type: select, proxies: [vpn-node]}\nrules:\n  - MATCH,A\n",
			wantErr: "duplicated",
		},
		{
			name:    "unknown group reference",
			remote:  "proxy-groups:\n  - {name: A, type: select, proxies: [missing]}\nrules:\n  - MATCH,A\n",
			wantErr: "unknown proxy or group",
		},
		{
			name:    "remote proxy provider use",
			remote:  "proxy-groups:\n  - name: A\n    type: select\n    use: [manual-provider]\nrules:\n  - MATCH,A\n",
			wantErr: "cannot use proxy-providers",
		},
		{
			name:    "unknown rule provider",
			remote:  "proxy-groups:\n  - {name: A, type: select, proxies: [vpn-node]}\nrules:\n  - RULE-SET,missing,A\n  - MATCH,A\n",
			wantErr: "unknown rule-provider",
		},
		{
			name:    "unknown rule target",
			remote:  "proxy-groups:\n  - {name: A, type: select, proxies: [vpn-node]}\nrules:\n  - MATCH,missing\n",
			wantErr: "unknown proxy or group",
		},
		{
			name:    "unknown provider download proxy",
			remote:  "proxy-groups:\n  - {name: A, type: select, proxies: [vpn-node]}\nrule-providers:\n  p: {type: http, url: https://example.com/p.mrs, proxy: missing}\nrules:\n  - RULE-SET,p,A\n  - MATCH,A\n",
			wantErr: "rule-provider \"p\" references unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := map[string]any{
				"proxies": []map[string]any{{"name": "vpn-node", "type": "vless"}},
				"proxy-groups": []map[string]any{{
					"name": "PROXY", "type": "select", "proxies": []string{"vpn-node", "DIRECT"},
				}},
				"rules": []string{"MATCH,PROXY"},
			}
			err := mergeRemoteClashRulesYAML(base, tt.remote)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestRemoteClashRouteGraphAcceptsLogicalRulesAndCachedDocument(t *testing.T) {
	const remote = `
proxy-groups:
  - name: Auto
    type: url-test
    include-all: true
  - name: Video
    type: select
    proxies: [Auto, DIRECT]
rule-providers:
  video:
    type: http
    url: https://example.com/video.mrs
    proxy: Auto
rules:
  - RULE-SET,video,Video
  - AND,((NETWORK,TCP),(DST-PORT,443)),Video
  - GEOIP,private,DIRECT,no-resolve
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve,src
  - MATCH,Auto
`
	var requests atomic.Int32
	resolver := newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return remoteRoutingResponse(http.StatusOK, remote), nil
	}), false)
	const source = "https://example.com/routing.yaml"
	if err := resolver.refreshSource(remoteRoutingClash, source); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	entry, remoteSource, err := resolver.resolveEntry(remoteRoutingClash, source)
	if err != nil || !remoteSource || entry.Clash == nil {
		t.Fatalf("entry remote=%v parsed=%v err=%v", remoteSource, entry.Clash != nil, err)
	}
	base := map[string]any{
		"proxies": []map[string]any{{"name": "vpn-node", "type": "vless"}},
		"proxy-groups": []map[string]any{{
			"name": "PROXY", "type": "select", "proxies": []string{"vpn-node", "DIRECT"},
		}},
		"rules": []string{"MATCH,PROXY"},
	}
	if err := mergeRemoteClashRules(base, entry.Clash); err != nil {
		t.Fatalf("merge cached document: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, want 1", requests.Load())
	}
}
