package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedChainProxy(t *testing.T, proxy *model.ChainProxy) *model.ChainProxy {
	t.Helper()
	if proxy.Type == "" {
		proxy.Type = "socks"
	}
	if proxy.Server == "" {
		proxy.Server = "chain.example.com"
	}
	if proxy.Port == 0 {
		proxy.Port = 1080
	}
	if err := database.GetDB().Create(proxy).Error; err != nil {
		t.Fatalf("seed chain proxy: %v", err)
	}
	return proxy
}

func seedChainSelector(t *testing.T, selector *model.ChainSelector) *model.ChainSelector {
	t.Helper()
	if err := database.GetDB().Create(selector).Error; err != nil {
		t.Fatalf("seed chain selector: %v", err)
	}
	return selector
}

// assignChainInbound points an existing inbound at a chain row; the seed helper
// creates inbounds without one.
func assignChainInbound(t *testing.T, inboundId, chainProxyId, chainSelectorId int) {
	t.Helper()
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", inboundId).
		Updates(map[string]any{"chain_proxy_id": chainProxyId, "chain_selector_id": chainSelectorId}).Error; err != nil {
		t.Fatalf("assign chain: %v", err)
	}
}

// docByRemarks finds the document whose remark starts with the inbound remark;
// the full remark also carries the client email.
func docByRemarks(t *testing.T, out, remarkPrefix string) map[string]any {
	t.Helper()
	for _, doc := range parseSubJsonDocs(t, out) {
		if remark, _ := doc["remarks"].(string); strings.HasPrefix(remark, remarkPrefix) {
			return doc
		}
	}
	t.Fatalf("document %q missing:\n%s", remarkPrefix, out)
	return nil
}

func docOutboundByTag(t *testing.T, doc map[string]any, tag string) map[string]any {
	t.Helper()
	outbounds, _ := doc["outbounds"].([]any)
	for _, ob := range outbounds {
		outbound, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if outbound["tag"] == tag {
			return outbound
		}
	}
	t.Fatalf("outbound %q missing from %v", tag, docOutboundTags(doc))
	return nil
}

func dialerProxyOf(t *testing.T, outbound map[string]any) string {
	t.Helper()
	stream, _ := outbound["streamSettings"].(map[string]any)
	sockopt, _ := stream["sockopt"].(map[string]any)
	value, _ := sockopt["dialerProxy"].(string)
	return value
}

func docRules(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	routing, _ := doc["routing"].(map[string]any)
	rawRules, _ := routing["rules"].([]any)
	rules := make([]map[string]any, 0, len(rawRules))
	for _, raw := range rawRules {
		if rule, ok := raw.(map[string]any); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func docBalancers(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	routing, _ := doc["routing"].(map[string]any)
	rawBalancers, _ := routing["balancers"].([]any)
	balancers := make([]map[string]any, 0, len(rawBalancers))
	for _, raw := range rawBalancers {
		if entry, ok := raw.(map[string]any); ok {
			balancers = append(balancers, entry)
		}
	}
	return balancers
}

// A single assigned proxy turns the endpoint into a chained one: it dials the
// hop outbound by tag, and the hop carries the upstream credentials.
func TestSubJson_ChainProxyAddsHopAndDialerProxy(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "chained", 4901, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{
		Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080,
		Username: "operator", Password: "s3cret", Enabled: true,
	})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "chained")

	proxy := docOutboundByTag(t, doc, "proxy")
	if got := dialerProxyOf(t, proxy); got != chainHopTag(hop.Id) {
		t.Fatalf("dialerProxy = %q, want %q", got, chainHopTag(hop.Id))
	}
	// The proxy keeps its own endpoint settings; the hop is a separate outbound.
	stream, _ := proxy["streamSettings"].(map[string]any)
	if stream["network"] != "ws" {
		t.Fatalf("endpoint transport = %v, want ws", stream["network"])
	}

	hopOut := docOutboundByTag(t, doc, chainHopTag(hop.Id))
	if hopOut["protocol"] != "socks" {
		t.Fatalf("hop protocol = %v, want socks", hopOut["protocol"])
	}
	settings, _ := hopOut["settings"].(map[string]any)
	servers, _ := settings["servers"].([]any)
	if len(servers) != 1 {
		t.Fatalf("hop servers = %v, want one", settings["servers"])
	}
	server, _ := servers[0].(map[string]any)
	if server["address"] != "hop.example.com" || server["port"] != float64(1080) {
		t.Fatalf("hop server = %v, want the configured endpoint", server)
	}
	users, _ := server["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("hop users = %v, want one credential pair", server["users"])
	}
	user, _ := users[0].(map[string]any)
	if user["user"] != "operator" || user["pass"] != "s3cret" {
		t.Fatalf("hop credentials = %v, want the configured pair", user)
	}
}

// An HTTPS proxy needs the hop outbound to speak TLS to it, at the hop's own
// server name — the endpoint's TLS settings must not leak onto the hop.
func TestSubJson_ChainHTTPSProxyUsesOwnTLS(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "https-chain", 4902, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{
		Remark: "https-entry", Type: "http", Server: "https-proxy.example.com", Port: 8443,
		TLS: true, Enabled: true,
	})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "https-chain")
	hopOut := docOutboundByTag(t, doc, chainHopTag(hop.Id))
	if hopOut["protocol"] != "http" {
		t.Fatalf("hop protocol = %v, want http", hopOut["protocol"])
	}
	stream, _ := hopOut["streamSettings"].(map[string]any)
	if stream["security"] != "tls" {
		t.Fatalf("hop security = %v, want tls", stream["security"])
	}
	// This core removed allowInsecure, so emitting it would make the config
	// unloadable — verification has to stay on.
	if _, has := stream["tlsSettings"]; has {
		t.Fatalf("hop must not carry TLS knobs the core rejects: %v", stream)
	}
	endpoint := docOutboundByTag(t, doc, "proxy")
	endpointStream, _ := endpoint["streamSettings"].(map[string]any)
	if endpointStream["security"] != "tls" || endpointStream["serverName"] == "https-proxy.example.com" {
		t.Fatalf("endpoint TLS picked up the hop: %v", endpointStream)
	}
}

// A selector with several hops emits one endpoint copy per hop and routes
// through a balancer over them: the bundled core resolves dialerProxy by
// outbound tag only, so a single endpoint cannot carry a strategy.
func TestSubJson_ChainSelectorEmitsBalancedCopies(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "balanced", 4903, 1, wsTLSStream)
	a := seedChainProxy(t, &model.ChainProxy{Remark: "a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
	b := seedChainProxy(t, &model.ChainProxy{Remark: "b", Type: "socks", Server: "b.example.com", Port: 1081, Enabled: true})
	c := seedChainProxy(t, &model.ChainProxy{Remark: "c", Type: "http", Server: "c.example.com", Port: 3128, Enabled: true})
	selector := seedChainSelector(t, &model.ChainSelector{
		Remark: "entries", Strategy: "roundRobin",
		ProxyIds: []int{a.Id, b.Id, c.Id}, Enabled: true,
	})
	assignChainInbound(t, ib.Id, 0, selector.Id)

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "balanced")

	wantTags := []string{
		chainVariantTag(selector.Id, 0), chainVariantTag(selector.Id, 1), chainVariantTag(selector.Id, 2),
		chainHopTag(a.Id), chainHopTag(b.Id), chainHopTag(c.Id), "direct", "block",
	}
	if got := strings.Join(docOutboundTags(doc), ","); got != strings.Join(wantTags, ",") {
		t.Fatalf("outbound tags = %v, want %v", docOutboundTags(doc), wantTags)
	}
	for i, hop := range []*model.ChainProxy{a, b, c} {
		copyTag := chainVariantTag(selector.Id, i)
		if got := dialerProxyOf(t, docOutboundByTag(t, doc, copyTag)); got != chainHopTag(hop.Id) {
			t.Fatalf("%s dialerProxy = %q, want %q", copyTag, got, chainHopTag(hop.Id))
		}
	}

	balancers := docBalancers(t, doc)
	if len(balancers) != 1 {
		t.Fatalf("balancers = %v, want one", balancers)
	}
	if balancers[0]["tag"] != chainBalancerTag(selector.Id) {
		t.Fatalf("balancer tag = %v", balancers[0]["tag"])
	}
	selectorPrefixes, _ := balancers[0]["selector"].([]any)
	if len(selectorPrefixes) != 1 || selectorPrefixes[0] != chainVariantPrefixFor(selector.Id) {
		t.Fatalf("balancer selector = %v, want only this selector's copies", selectorPrefixes)
	}
	strategy, _ := balancers[0]["strategy"].(map[string]any)
	if strategy["type"] != "roundRobin" {
		t.Fatalf("strategy = %v, want roundRobin", strategy)
	}

	// Every rule that used to dial the plain proxy must dial the balancer now;
	// a leftover outboundTag "proxy" would route around the chain.
	for _, rule := range docRules(t, doc) {
		if rule["outboundTag"] == "proxy" {
			t.Fatalf("rule still targets the plain proxy outbound: %v", rule)
		}
		if rule["balancerTag"] != nil && rule["balancerTag"] != chainBalancerTag(selector.Id) {
			t.Fatalf("rule targets an unexpected balancer: %v", rule)
		}
	}
}

// only one hop is usable: the chain needs no balancer and emits as a plain hop.
func TestSubJson_ChainSelectorSingleUsableHop(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "one-hop", 4904, 1, wsTLSStream)
	only := seedChainProxy(t, &model.ChainProxy{Remark: "only", Type: "socks", Server: "only.example.com", Port: 1080, Enabled: true})
	disabled := seedChainProxy(t, &model.ChainProxy{Remark: "off", Type: "socks", Server: "off.example.com", Port: 1081, Enabled: false})
	selector := seedChainSelector(t, &model.ChainSelector{
		Remark: "entries", Strategy: "leastPing",
		ProxyIds: []int{only.Id, disabled.Id}, Enabled: true,
	})
	assignChainInbound(t, ib.Id, 0, selector.Id)

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "one-hop")
	proxy := docOutboundByTag(t, doc, "proxy")
	if got := dialerProxyOf(t, proxy); got != chainHopTag(only.Id) {
		t.Fatalf("dialerProxy = %q, want %q", got, chainHopTag(only.Id))
	}
	if balancers := docBalancers(t, doc); len(balancers) != 0 {
		t.Fatalf("single-hop chain must not emit a balancer: %v", balancers)
	}
	if _, has := doc["burstObservatory"]; has {
		t.Fatal("single-hop chain must not emit an observatory")
	}
	if strings.Join(docOutboundTags(doc), ",") == "" || strings.Contains(strings.Join(docOutboundTags(doc), ","), chainHopTag(disabled.Id)) {
		t.Fatalf("disabled hop leaked into the document: %v", docOutboundTags(doc))
	}
}

// The probing strategies need a burst observatory over the copies, plus a
// fallback so a total probe outage still dispatches.
func TestSubJson_ChainSelectorLeastPingEmitsObservatory(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "probed", 4905, 1, wsTLSStream)
	a := seedChainProxy(t, &model.ChainProxy{Remark: "a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
	b := seedChainProxy(t, &model.ChainProxy{Remark: "b", Type: "socks", Server: "b.example.com", Port: 1081, Enabled: true})
	selector := seedChainSelector(t, &model.ChainSelector{
		Remark: "entries", Strategy: "leastPing",
		ProxyIds: []int{a.Id, b.Id}, Enabled: true,
	})
	assignChainInbound(t, ib.Id, 0, selector.Id)

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "probed")
	observatory, _ := doc["burstObservatory"].(map[string]any)
	subjects, _ := observatory["subjectSelector"].([]any)
	if len(subjects) != 1 || subjects[0] != chainVariantPrefixFor(selector.Id) {
		t.Fatalf("observatory subjectSelector = %v", subjects)
	}
	balancers := docBalancers(t, doc)
	if balancers[0]["fallbackTag"] != chainVariantTag(selector.Id, 0) {
		t.Fatalf("fallbackTag = %v, want the first copy", balancers[0]["fallbackTag"])
	}
}

// An assignment that resolves to no enabled hop must drop the endpoint: emitting
// it would hand the subscriber a direct connection around the chain.
func TestSubJson_ChainUnusableDropsEndpoint(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T) (proxyId, selectorId int)
	}{
		{
			name: "disabled proxy",
			seed: func(t *testing.T) (int, int) {
				hop := seedChainProxy(t, &model.ChainProxy{Remark: "off", Type: "socks", Server: "off.example.com", Port: 1080, Enabled: false})
				return hop.Id, 0
			},
		},
		{
			name: "deleted proxy",
			seed: func(t *testing.T) (int, int) { return 4242, 0 },
		},
		{
			name: "disabled selector",
			seed: func(t *testing.T) (int, int) {
				hop := seedChainProxy(t, &model.ChainProxy{Remark: "a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
				selector := seedChainSelector(t, &model.ChainSelector{Remark: "off", Strategy: "random", ProxyIds: []int{hop.Id}, Enabled: false})
				return 0, selector.Id
			},
		},
		{
			name: "selector whose hops are all disabled",
			seed: func(t *testing.T) (int, int) {
				a := seedChainProxy(t, &model.ChainProxy{Remark: "a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: false})
				selector := seedChainSelector(t, &model.ChainSelector{Remark: "dead", Strategy: "random", ProxyIds: []int{a.Id}, Enabled: true})
				return 0, selector.Id
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			ib := seedSubInbound(t, "s1", "gone", 4906, 1, wsTLSStream)
			proxyId, selectorId := tc.seed(t)
			assignChainInbound(t, ib.Id, proxyId, selectorId)

			out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			if out != "" {
				t.Fatalf("endpoint with an unusable chain must not be emitted:\n%s", out)
			}
		})
	}
}

// An inbound without an assignment keeps the previous shape, hop-free.
func TestSubJson_NoChainKeepsDirectEndpoint(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "plain", 4907, 1, wsTLSStream)
	seedChainProxy(t, &model.ChainProxy{Remark: "unused", Type: "socks", Server: "unused.example.com", Port: 1080, Enabled: true})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "plain")
	if got := strings.Join(docOutboundTags(doc), ","); got != "proxy,direct,block" {
		t.Fatalf("outbound tags = %v, want the unchained set", docOutboundTags(doc))
	}
	if got := dialerProxyOf(t, docOutboundByTag(t, doc, "proxy")); got != "" {
		t.Fatalf("unchained endpoint carries dialerProxy %q", got)
	}
}

// A balancer document re-emits chained members, so it must carry the hops its
// members dial through — otherwise the core refuses the config.
func TestSubJson_BalancerCarriesMemberChainHops(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "chained", 4908, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true})
	assignChainInbound(t, ib.Id, hop.Id, 0)
	seedSubBalancer(t, &model.SubBalancer{Remark: "auto", Strategy: "random", InboundIds: []int{ib.Id}, SortOrder: 2, Enabled: true})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "auto")
	hops := docOutboundByTag(t, doc, chainHopTag(hop.Id))
	if hops["protocol"] != "socks" {
		t.Fatalf("balancer document lost the member hop: %v", docOutboundTags(doc))
	}
	if _, err := buildXrayConfig(t, doc); err != nil {
		t.Fatalf("xray refuses the balancer document: %v", err)
	}
}

// The strongest statement the subscription can make: the bundled core accepts
// the chained document — dialerProxy, hop outbound, balancer and observatory.
func TestSubJson_ChainDocumentsAreAcceptedByXray(t *testing.T) {
	cases := []struct {
		name     string
		strategy string
		hops     int
	}{
		{"single proxy", "", 1},
		{"random selector", "random", 2},
		{"roundRobin selector", "roundRobin", 2},
		{"leastPing selector", "leastPing", 2},
		{"leastLoad selector", "leastLoad", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			ib := seedSubInbound(t, "s1", "validated", 4910, 1, wsTLSStream)
			ids := make([]int, 0, tc.hops)
			for i := 0; i < tc.hops; i++ {
				hop := seedChainProxy(t, &model.ChainProxy{
					Remark: "hop", Type: "socks", Server: "hop.example.com", Port: 1080 + i,
					Username: "u", Password: "p", Enabled: true,
				})
				ids = append(ids, hop.Id)
			}
			if tc.strategy == "" {
				assignChainInbound(t, ib.Id, ids[0], 0)
			} else {
				selector := seedChainSelector(t, &model.ChainSelector{
					Remark: "entries", Strategy: tc.strategy, ProxyIds: ids, Enabled: true,
				})
				assignChainInbound(t, ib.Id, 0, selector.Id)
			}

			out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			doc := docByRemarks(t, out, "validated")
			if _, err := buildXrayConfig(t, doc); err != nil {
				t.Fatalf("xray refuses the chained document: %v", err)
			}
		})
	}
}

// A balanced chain inside a sub-balancer document contributes one member per
// hop: the document's own balancer picks among them, and no member dials around
// the chain. The hop outbounds travel with them or xray refuses the config.
func TestSubJson_BalancerWithBalancedChainMembers(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "chained", 4912, 1, wsTLSStream)
	a := seedChainProxy(t, &model.ChainProxy{Remark: "a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
	b := seedChainProxy(t, &model.ChainProxy{Remark: "b", Type: "socks", Server: "b.example.com", Port: 1081, Enabled: true})
	selector := seedChainSelector(t, &model.ChainSelector{
		Remark: "entries", Strategy: "roundRobin", ProxyIds: []int{a.Id, b.Id}, Enabled: true,
	})
	assignChainInbound(t, ib.Id, 0, selector.Id)
	seedSubBalancer(t, &model.SubBalancer{Remark: "auto", Strategy: "random", InboundIds: []int{ib.Id}, SortOrder: 2, Enabled: true})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "auto")
	tags := docOutboundTags(doc)
	members := 0
	for _, tag := range tags {
		if strings.HasPrefix(tag, "bal-1-") {
			members++
		}
	}
	if members != 2 {
		t.Fatalf("balancer members = %v, want one per hop", tags)
	}
	if !strings.Contains(strings.Join(tags, ","), chainHopTag(a.Id)) ||
		!strings.Contains(strings.Join(tags, ","), chainHopTag(b.Id)) {
		t.Fatalf("balancer document lost a hop outbound: %v", tags)
	}
	// Every member dials a hop, so no member bypasses the chain.
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "bal-1-") {
			continue
		}
		if got := dialerProxyOf(t, docOutboundByTag(t, doc, tag)); !strings.HasPrefix(got, chainHopPrefix) {
			t.Fatalf("member %q dialerProxy = %q, want a hop", tag, got)
		}
	}
	if _, err := buildXrayConfig(t, doc); err != nil {
		t.Fatalf("xray refuses the balancer document: %v", err)
	}
}

// A share link cannot express a hop, so the raw subscription must skip a
// chained inbound instead of handing the subscriber a direct link around it.
func TestSub_ChainInboundOmittedFromRawLinks(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "plain", 4915, 1, wsTLSStream)
	chained := seedSubInbound(t, "s1", "chained", 4916, 2, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true})
	assignChainInbound(t, chained.Id, hop.Id, 0)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %v, want only the unchained inbound", links)
	}
	if !strings.Contains(links[0], "plain") {
		t.Fatalf("link = %q, want the unchained inbound", links[0])
	}

	// The JSON subscription still carries the chained inbound, with its hop.
	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	doc := docByRemarks(t, out, "chained")
	if got := dialerProxyOf(t, docOutboundByTag(t, doc, "proxy")); got != chainHopTag(hop.Id) {
		t.Fatalf("JSON dialerProxy = %q, want %q", got, chainHopTag(hop.Id))
	}
}

// buildXrayConfig runs a generated document through the core's own config
// loader, which is what a real client does before it can connect.
func buildXrayConfig(t *testing.T, doc map[string]any) (*conf.Config, error) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	config := new(conf.Config)
	if err := json.Unmarshal(raw, config); err != nil {
		return nil, err
	}
	if _, err := config.Build(); err != nil {
		return nil, err
	}
	return config, nil
}
