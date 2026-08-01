package sub

import (
	"encoding/base64"
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

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

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

func TestParseRemoteRoutingURLKeepsInlineCompatibility(t *testing.T) {
	inline := "happ://routing/onadd/abc"
	if got, remote, err := parseRemoteRoutingURL(inline); err != nil || remote || got != "" {
		t.Fatalf("inline classified as got=%q remote=%v err=%v", got, remote, err)
	}

	got, remote, err := parseRemoteRoutingURL("  https://example.com/rules#ignored  ")
	if err != nil || !remote || got != "https://example.com/rules" {
		t.Fatalf("HTTPS classified as got=%q remote=%v err=%v", got, remote, err)
	}

	if _, remote, err := parseRemoteRoutingURL("http://example.com/rules"); err == nil || !remote {
		t.Fatalf("plain HTTP should be recognized and rejected: remote=%v err=%v", remote, err)
	}

	multiline := "https://example.com/rules\nMATCH,PROXY"
	if got, remote, err := parseRemoteRoutingURL(multiline); err != nil || remote || got != "" {
		t.Fatalf("mixed inline text classified as got=%q remote=%v err=%v", got, remote, err)
	}
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

	got, remote, err := resolver.resolve(remoteRoutingHapp, "https://routing.example/")
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

	first, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	second, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || second != first {
		t.Fatalf("304 resolve got=%q err=%v", second, err)
	}
	now = now.Add(time.Minute)
	third, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || third != first {
		t.Fatalf("refreshed cache got=%q err=%v", third, err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestRemoteRoutingResolverCachesAndCoalescesColdFetch(t *testing.T) {
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

	results := make(chan string, 8)
	for range 8 {
		go func() {
			value, remote, err := resolver.resolve(remoteRoutingHapp, source)
			if err != nil || !remote {
				results <- "error"
				return
			}
			results <- value
		}()
	}
	<-started
	close(release)

	for range 8 {
		if value := <-results; !strings.HasPrefix(value, "happ://routing/onadd/") {
			t.Fatalf("unexpected result %q", value)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
	if _, _, err := resolver.resolve(remoteRoutingHapp, source); err != nil {
		t.Fatalf("cached resolve: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cached request count = %d, want 1", got)
	}
}

func TestRemoteRoutingResolverServesStaleAfterFailedRefresh(t *testing.T) {
	var requests atomic.Int32
	refreshDone := make(chan struct{}, 1)
	fail := atomic.Bool{}
	client := remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		if fail.Load() {
			refreshDone <- struct{}{}
			return remoteRoutingResponse(http.StatusBadGateway, "bad gateway"), nil
		}
		return remoteRoutingResponse(http.StatusOK, `{"Name":"last-good"}`), nil
	})
	resolver := newRemoteRoutingResolver(client, false)
	now := time.Unix(1_800_000_000, 0)
	resolver.now = func() time.Time { return now }
	const source = "https://example.com/default.json"

	first, _, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	fail.Store(true)
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	stale, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || stale != first {
		t.Fatalf("stale resolve got=%q remote=%v err=%v", stale, remote, err)
	}
	select {
	case <-refreshDone:
	default:
		t.Fatal("refresh did not run")
	}

	deadline := time.Now().Add(time.Second)
	for {
		resolver.mu.Lock()
		inflight := len(resolver.inflight)
		resolver.mu.Unlock()
		if inflight == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not finish")
		}
		time.Sleep(time.Millisecond)
	}

	if got, _, err := resolver.resolve(remoteRoutingHapp, source); err != nil || got != first {
		t.Fatalf("negative-cache resolve got=%q err=%v", got, err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRemoteRoutingResolverLoadsPersistedLastGood(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

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
	got, remote, err := resolver.resolve(remoteRoutingHapp, source)
	if err != nil || !remote || got != deeplink {
		t.Fatalf("persisted resolve got=%q remote=%v err=%v", got, remote, err)
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

	first, _, err := resolver.resolve(remoteRoutingClash, source)
	if err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	now = now.Add(remoteRoutingCacheTTL + time.Second)
	second, _, err := resolver.resolve(remoteRoutingClash, source)
	if err != nil || second != first {
		t.Fatalf("invalid refresh replaced last-good: got=%q err=%v", second, err)
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
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&SUBController{}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", true, "https://example.com/default.json", false)
	if recorder.Header().Get("Routing-Enable") != "true" || !strings.HasPrefix(recorder.Header().Get("Routing"), "happ://routing/onadd/") {
		t.Fatalf("headers = %#v", recorder.Header())
	}

	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusOK, "routing.help"), nil
	}), false)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	(&SUBController{}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", true, "https://example.com/bad", false)
	if recorder.Header().Get("Routing-Enable") != "" || recorder.Header().Get("Routing") != "" {
		t.Fatalf("invalid remote source leaked routing headers: %#v", recorder.Header())
	}
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
