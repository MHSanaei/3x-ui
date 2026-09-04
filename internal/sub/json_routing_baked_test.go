package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const bakedRoutingPayload = `{
	"DomainStrategy": "IPIfNonMatch",
	"RemoteDNSDomain": "https://8.8.8.8/dns-query",
	"RemoteDNSIP": "8.8.8.8",
	"DomesticDNSDomain": "https://77.88.8.8/dns-query",
	"DomesticDNSIP": "77.88.8.8",
	"DnsHosts": {"lknpd.nalog.ru": "213.24.64.181"},
	"RouteOrder": "block-proxy-direct",
	"DirectSites": ["geosite:category-ru"],
	"DirectIp": ["geoip:private"],
	"ProxySites": ["geosite:youtube"],
	"BlockSites": ["geosite:category-ads"]
}`

func ruleSignatures(t *testing.T, doc map[string]any) []string {
	t.Helper()
	routing, _ := doc["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	signatures := make([]string, 0, len(rules))
	for _, rule := range rules {
		m, _ := rule.(map[string]any)
		target, _ := m["outboundTag"].(string)
		if target == "" {
			target = "balancer:" + m["balancerTag"].(string)
		}
		kind := "ip"
		if _, has := m["domain"]; has {
			kind = "domain"
		}
		if _, has := m["network"]; has {
			kind = "network"
		}
		signatures = append(signatures, kind+"->"+target)
	}
	return signatures
}

func assertBakedRouting(t *testing.T, doc map[string]any, wantRules []string, proxyTag string) {
	t.Helper()
	dns, _ := doc["dns"].(map[string]any)
	if dns == nil {
		t.Fatalf("doc has no dns:\n%v", doc)
	}
	if dns["tag"] != "dns_out" || dns["queryStrategy"] != "UseIP" {
		t.Fatalf("dns header = %v", dns)
	}
	servers, _ := dns["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("dns servers = %d, want 2 (domestic + remote): %v", len(servers), servers)
	}
	first, _ := servers[0].(map[string]any)
	if first["address"] != "https://77.88.8.8/dns-query" {
		t.Fatalf("domestic dns = %v", first)
	}
	if domains, _ := first["domains"].([]any); strings.Join(stringify(domains), ",") != "geosite:category-ru" {
		t.Fatalf("domestic dns domains = %v", first["domains"])
	}
	second, _ := servers[1].(map[string]any)
	if second["address"] != "https://8.8.8.8/dns-query" {
		t.Fatalf("remote dns = %v", second)
	}
	hosts, _ := dns["hosts"].(map[string]any)
	if hosts["lknpd.nalog.ru"] != "213.24.64.181" {
		t.Fatalf("dns hosts = %v", dns["hosts"])
	}

	routing, _ := doc["routing"].(map[string]any)
	if routing["domainStrategy"] != "IPIfNonMatch" {
		t.Fatalf("domainStrategy = %v", routing["domainStrategy"])
	}
	want := make([]string, 0, len(wantRules))
	for _, rule := range wantRules {
		want = append(want, strings.Replace(rule, "PROXY", proxyTag, 1))
	}
	got := ruleSignatures(t, doc)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rules = %v\nwant %v", got, want)
	}
}

func TestSubJson_BakedRoutingInEveryDocument(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4801, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	js := NewSubJsonService("", "", "", bakedRoutingPayload, NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1:\n%s", len(docs), out)
	}
	want := []string{"domain->block", "domain->PROXY", "domain->direct", "ip->direct", "network->PROXY"}
	assertBakedRouting(t, docs[0], want, "proxy")
}

func TestSubJson_BakedRoutingReplacesLegacyRules(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4802, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	legacy := `[{"type":"field","domain":["geosite:example"],"outboundTag":"proxy"}]`
	js := NewSubJsonService("", legacy, "", bakedRoutingPayload, NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	routing, _ := docs[0]["routing"].(map[string]any)
	ruleJSON, _ := json.Marshal(routing["rules"])
	if strings.Contains(string(ruleJSON), "geosite:example") {
		t.Fatalf("legacy subJsonRules must not leak into baked docs: %s", ruleJSON)
	}
}

func TestSubJson_BakedRoutingWithBalancer(t *testing.T) {
	seedSubDB(t)
	tcp := seedSubInbound(t, "s1", "tcpin", 4803, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "auto", Strategy: "leastLoad", InboundIds: []int{tcp.Id}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", bakedRoutingPayload, NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 2 {
		t.Fatalf("docs = %d, want 2 (inbound + balancer):\n%s", len(docs), out)
	}

	// Manual doc keeps the plain proxy tag.
	assertBakedRouting(t, findDocByRemarks(docs, "tcpin-tcpin@e"), []string{
		"domain->block", "domain->PROXY", "domain->direct", "ip->direct", "network->PROXY",
	}, "proxy")

	// Balancer doc routes proxy groups into the balancer.
	balancerDoc := findDocByRemarks(docs, "auto")
	want := []string{"domain->block", "domain->balancer:balancer", "domain->direct", "ip->direct", "network->balancer:balancer"}
	assertBakedRouting(t, balancerDoc, want, "balancer:balancer")
}

func TestSubJson_BakedRoutingInvalidFallsBackToDefault(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4804, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	js := NewSubJsonService("", "", "", "not json at all", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson must survive a bad routing payload: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	routing, _ := docs[0]["routing"].(map[string]any)
	rules, _ := json.Marshal(routing["rules"])
	if !strings.Contains(string(rules), `"outboundTag":"proxy"`) {
		t.Fatalf("default routing missing: %s", rules)
	}
}

func TestSubJson_LegacyRulesStillWorkWithoutBakedRouting(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4805, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	legacy := `[{"type":"field","domain":["geosite:example"],"outboundTag":"proxy"}]`
	js := NewSubJsonService("", legacy, "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	routing, _ := docs[0]["routing"].(map[string]any)
	ruleJSON, _ := json.Marshal(routing["rules"])
	if !strings.Contains(string(ruleJSON), "geosite:example") {
		t.Fatalf("legacy rules missing: %s", ruleJSON)
	}
}

func TestSubJson_BakedRoutingRemoteWarmsAfterColdStart(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4806, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })
	const source = "https://example.com/DEFAULT.JSON"
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(200, mustMarshal(t, fullRoutingPayload())), nil
	}), false)

	js := NewSubJsonService("", "", "", source, NewSubService(""))
	// Cold: no request has primed the resolver cache yet.
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	routing, _ := docs[0]["routing"].(map[string]any)
	if routing["domainStrategy"] != "AsIs" {
		t.Fatalf("cold doc must keep default routing: %v", routing["domainStrategy"])
	}

	// The cron job warms the cache; the next request must bake the profile.
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingJson, source)
	out, _, err = js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs = parseSubJsonDocs(t, out)
	routing, _ = docs[0]["routing"].(map[string]any)
	if routing["domainStrategy"] != "IPIfNonMatch" {
		t.Fatalf("warm doc must carry the profile: %v", routing["domainStrategy"])
	}
	dns, _ := docs[0]["dns"].(map[string]any)
	servers, _ := dns["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("warm doc dns servers = %v", servers)
	}
}

func TestApplyCommonHeadersFallsBackToJsonRoutingProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var object map[string]any
	if err := json.Unmarshal([]byte(bakedRoutingPayload), &object); err != nil {
		t.Fatalf("payload: %v", err)
	}
	happDeeplink := "happ://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(mustMarshal(t, object)))
	incyDeeplink := "incy://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(mustMarshal(t, map[string]any{"Name": "RoscomVPN"})))

	cases := []struct {
		name      string
		jsonRules string
		happRules string
		want      string
	}{
		{name: "inline json becomes a happ deeplink", jsonRules: bakedRoutingPayload, want: happDeeplink},
		{name: "happ deeplink passes through", jsonRules: happDeeplink, want: happDeeplink},
		{name: "incy deeplink passes through", jsonRules: incyDeeplink, want: incyDeeplink},
		{name: "blank profile keeps the header unset", jsonRules: "", want: ""},
		{name: "unusable profile keeps the header unset", jsonRules: "happ://routing/onadd/%%%", want: ""},
		{name: "explicit happ rules take precedence", jsonRules: bakedRoutingPayload, happRules: "happ://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(`{"A":1}`)), want: "happ://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(`{"A":1}`))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			(&SUBController{subJsonRoutingRules: tc.jsonRules}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", false, tc.happRules, false)
			if got := recorder.Header().Get("Routing"); got != tc.want {
				t.Fatalf("Routing = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyCommonHeadersJsonRoutingRemoteFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })

	const source = "https://example.com/DEFAULT.JSON"
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(200, mustMarshal(t, fullRoutingPayload())), nil
	}), false)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&SUBController{subJsonRoutingRules: source}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", false, "", false)
	if got := recorder.Header().Get("Routing"); got != "" {
		t.Fatalf("cold cache must keep the header unset, got %q", got)
	}

	primeRemoteRouting(t, routingSourceResolver, remoteRoutingJson, source)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	(&SUBController{subJsonRoutingRules: source}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", false, "", false)
	got := recorder.Header().Get("Routing")
	if !strings.HasPrefix(got, "happ://routing/onadd/") {
		t.Fatalf("warm cache Routing = %q", got)
	}
	decoded, err := decodeRoutingBase64(strings.TrimPrefix(got, "happ://routing/onadd/"))
	if err != nil {
		t.Fatalf("deeplink payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("deeplink JSON: %v", err)
	}
	if payload["Name"] != "RoscomVPN" {
		t.Fatalf("deeplink payload = %v", payload)
	}
	waitRemoteRoutingIdle(t, routingSourceResolver)
}

func TestSubJson_BakedRoutingRemoteUpdateReachesDocuments(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4807, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })
	const source = "https://example.com/DEFAULT.JSON"

	current := mustMarshal(t, fullRoutingPayload())
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(200, current), nil
	}), false)

	js := NewSubJsonService("", "", "", source, NewSubService(""))
	// A cold resolver fails closed (default routing); prime the cache first.
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingJson, source)
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	routing, _ := docs[0]["routing"].(map[string]any)
	if routing["domainStrategy"] != "IPIfNonMatch" {
		t.Fatalf("first doc must carry the profile: %v", routing["domainStrategy"])
	}

	// The operator edits the published profile; after the cache TTL expires,
	// the next request must re-bake the template with the new payload.
	updated := fullRoutingPayload()
	updated["DomainStrategy"] = "AsIs"
	current = mustMarshal(t, updated)
	waitRemoteRoutingIdle(t, routingSourceResolver)
	staleKey := remoteRoutingKey{kind: remoteRoutingJson, source: source}
	routingSourceResolver.mu.Lock()
	entry := routingSourceResolver.entries[staleKey]
	entry.FetchedAt = time.Now().Add(-remoteRoutingCacheTTL - time.Minute).Unix()
	routingSourceResolver.entries[staleKey] = entry
	delete(routingSourceResolver.lastAttempt, staleKey)
	routingSourceResolver.mu.Unlock()
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingJson, source)
	out, _, err = js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs = parseSubJsonDocs(t, out)
	routing, _ = docs[0]["routing"].(map[string]any)
	if routing["domainStrategy"] != "AsIs" {
		t.Fatalf("updated profile must reach the documents without a restart: %v", routing["domainStrategy"])
	}
	waitRemoteRoutingIdle(t, routingSourceResolver)
}

func TestRemoteRoutingJsonHasItsOwnPersistedRow(t *testing.T) {
	seedSubDB(t)
	const source = "https://example.com/DEFAULT.JSON"

	// Two happ-payload settings pointing at different sources must not
	// overwrite each other's persisted cache rows.
	happSource := "https://example.com/HAPP.json"
	for _, tc := range []struct {
		kind    remoteRoutingKind
		source  string
		payload string
	}{
		{kind: remoteRoutingHapp, source: happSource, payload: `{"Name":"happ-profile"}`},
		{kind: remoteRoutingJson, source: source, payload: `{"Name":"json-profile"}`},
	} {
		deeplink, err := normalizeHappRouting([]byte(tc.payload))
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		newRemoteRoutingResolver(nil, false).persistEntry(tc.kind, remoteRoutingCacheEntry{
			Source: tc.source, Content: deeplink, FetchedAt: time.Now().Unix(),
		})
	}

	for _, tc := range []struct {
		kind   remoteRoutingKind
		source string
		want   string
	}{
		{kind: remoteRoutingHapp, source: happSource, want: "happ-profile"},
		{kind: remoteRoutingJson, source: source, want: "json-profile"},
	} {
		resolver := newRemoteRoutingResolver(nil, true)
		resolver.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
		resolver.ensurePersistedLoaded()
		got, remote, err := resolver.resolve(tc.kind, tc.source)
		if err != nil || !remote {
			t.Fatalf("resolve kind=%s: remote=%v err=%v", tc.kind, remote, err)
		}
		decoded, err := decodeRoutingBase64(strings.TrimPrefix(got, "happ://routing/onadd/"))
		if err != nil {
			t.Fatalf("decode kind=%s: %v", tc.kind, err)
		}
		var payload map[string]any
		if json.Unmarshal(decoded, &payload) != nil || payload["Name"] != tc.want {
			t.Fatalf("kind=%s payload = %s", tc.kind, decoded)
		}
	}
}
