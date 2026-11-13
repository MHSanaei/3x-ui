package sub

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func clashDoc(t *testing.T, raw string) map[string]any {
	t.Helper()
	doc := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("subscription is not YAML: %v\n%s", err, raw)
	}
	return doc
}

func clashProxyByName(t *testing.T, doc map[string]any, name string) map[string]any {
	t.Helper()
	proxies, _ := doc["proxies"].([]any)
	for _, raw := range proxies {
		proxy, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if proxy["name"] == name {
			return proxy
		}
	}
	t.Fatalf("proxy %q missing from %v", name, clashProxyNames(doc))
	return nil
}

func clashProxyNames(doc map[string]any) []string {
	proxies, _ := doc["proxies"].([]any)
	names := make([]string, 0, len(proxies))
	for _, raw := range proxies {
		if proxy, ok := raw.(map[string]any); ok {
			if name, ok := proxy["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

func clashGroups(doc map[string]any) []map[string]any {
	raw, _ := doc["proxy-groups"].([]any)
	groups := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if group, ok := item.(map[string]any); ok {
			groups = append(groups, group)
		}
	}
	return groups
}

func clashGroupByName(t *testing.T, doc map[string]any, name string) map[string]any {
	t.Helper()
	for _, group := range clashGroups(doc) {
		if group["name"] == name {
			return group
		}
	}
	t.Fatalf("group %q missing from %v", name, clashGroups(doc))
	return nil
}

func chainYaml(t *testing.T, subId string) string {
	t.Helper()
	out, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	return out
}

// A chained endpoint dials mihomo through a hop proxy that the profile also
// declares: mihomo refuses a dialer-proxy naming an unknown proxy.
func TestClash_ChainProxySetsDialerProxy(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "chained", 4920, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{
		Remark: "entry-de", Type: "socks", Server: "hop.example.com", Port: 1080,
		Username: "operator", Password: "s3cret", Enabled: true,
	})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	doc := clashDoc(t, chainYaml(t, "s1"))
	endpoint := clashProxyByName(t, doc, "chained-chained@e")
	if endpoint["dialer-proxy"] != "entry-de" {
		t.Fatalf("dialer-proxy = %v, want the hop name", endpoint["dialer-proxy"])
	}
	hopProxy := clashProxyByName(t, doc, "entry-de")
	if hopProxy["type"] != "socks5" || hopProxy["server"] != "hop.example.com" {
		t.Fatalf("hop proxy = %v", hopProxy)
	}
	// YAML numbers decode as int or float depending on the emitter; compare text.
	if got := fmt.Sprint(hopProxy["port"]); got != "1080" {
		t.Fatalf("hop port = %v, want 1080", hopProxy["port"])
	}
	if hopProxy["username"] != "operator" || hopProxy["password"] != "s3cret" {
		t.Fatalf("hop credentials = %v", hopProxy)
	}
	if _, leaked := endpoint[clashChainPlanKey]; leaked {
		t.Fatal("internal chain marker leaked into the profile")
	}
	// Hops are entry points: the exit selector must not offer them.
	proxyGroup := clashGroupByName(t, doc, "PROXY")
	members, _ := proxyGroup["proxies"].([]any)
	for _, member := range members {
		if member == "entry-de" {
			t.Fatalf("hop offered as an exit: %v", members)
		}
	}
}

// An HTTPS hop keeps its own TLS knobs, and the endpoint's own TLS is untouched.
func TestClash_ChainHTTPSHopUsesTLS(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "https", 4921, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{
		Remark: "https-entry", Type: "http", Server: "https-proxy.example.com", Port: 8443,
		TLS: true, Enabled: true,
	})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	doc := clashDoc(t, chainYaml(t, "s1"))
	hopProxy := clashProxyByName(t, doc, "https-entry")
	if hopProxy["type"] != "http" {
		t.Fatalf("hop type = %v, want http", hopProxy["type"])
	}
	if hopProxy["tls"] != true || hopProxy["servername"] != "https-proxy.example.com" {
		t.Fatalf("hop TLS = %v", hopProxy)
	}
	endpoint := clashProxyByName(t, doc, "https-https@e")
	if _, chained := endpoint["tls"]; !chained {
		t.Fatalf("endpoint lost its own TLS: %v", endpoint)
	}
}

// A selector becomes one mihomo group the endpoint dials through; the leastPing
// strategy maps onto mihomo's url-test, which picks the fastest hop.
func TestClash_ChainSelectorEmitsGroup(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "balanced", 4922, 1, wsTLSStream)
	a := seedChainProxy(t, &model.ChainProxy{Remark: "hop-a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
	b := seedChainProxy(t, &model.ChainProxy{Remark: "hop-b", Type: "socks", Server: "b.example.com", Port: 1081, Enabled: true})
	selector := seedChainSelector(t, &model.ChainSelector{
		Remark: "entries", Strategy: "leastPing", ProxyIds: []int{a.Id, b.Id}, Enabled: true,
	})
	assignChainInbound(t, ib.Id, 0, selector.Id)

	doc := clashDoc(t, chainYaml(t, "s1"))
	group := clashGroupByName(t, doc, "entries")
	if group["type"] != "url-test" {
		t.Fatalf("group type = %v, want url-test", group["type"])
	}
	members, _ := group["proxies"].([]any)
	if strings.Join(toStrings(members), ",") != "hop-a,hop-b" {
		t.Fatalf("group members = %v", members)
	}
	endpoint := clashProxyByName(t, doc, "balanced-balanced@e")
	if endpoint["dialer-proxy"] != "entries" {
		t.Fatalf("dialer-proxy = %v, want the group name", endpoint["dialer-proxy"])
	}
}

// mihomo has no random group; a random selector spreads connections like
// round-robin, and leastLoad keeps mihomo's consistent hashing.
func TestClash_ChainSelectorStrategyMapping(t *testing.T) {
	cases := []struct {
		strategy     string
		wantType     string
		wantBalancer string
	}{
		{"random", "load-balance", "round-robin"},
		{"roundRobin", "load-balance", "round-robin"},
		{"leastLoad", "load-balance", "consistent-hashing"},
		{"leastPing", "url-test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.strategy, func(t *testing.T) {
			seedSubDB(t)
			ib := seedSubInbound(t, "s1", "mapped", 4923, 1, wsTLSStream)
			a := seedChainProxy(t, &model.ChainProxy{Remark: "hop-a", Type: "socks", Server: "a.example.com", Port: 1080, Enabled: true})
			b := seedChainProxy(t, &model.ChainProxy{Remark: "hop-b", Type: "socks", Server: "b.example.com", Port: 1081, Enabled: true})
			selector := seedChainSelector(t, &model.ChainSelector{
				Remark: "entries", Strategy: tc.strategy, ProxyIds: []int{a.Id, b.Id}, Enabled: true,
			})
			assignChainInbound(t, ib.Id, 0, selector.Id)

			group := clashGroupByName(t, clashDoc(t, chainYaml(t, "s1")), "entries")
			if group["type"] != tc.wantType {
				t.Fatalf("group type = %v, want %v", group["type"], tc.wantType)
			}
			if got, _ := group["strategy"].(string); got != tc.wantBalancer {
				t.Fatalf("load-balance strategy = %q, want %q", got, tc.wantBalancer)
			}
		})
	}
}

// A hop whose remark matches an endpoint name must still resolve: the endpoint
// dials the hop proxy, not itself.
func TestClash_ChainHopNameCollisionIsDisambiguated(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "same", 4924, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{
		Remark: "same-same@e", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true,
	})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	doc := clashDoc(t, chainYaml(t, "s1"))
	names := clashProxyNames(doc)
	segments := map[string]bool{}
	for _, name := range names {
		if segments[name] {
			t.Fatalf("duplicate proxy name %q in %v", name, names)
		}
		segments[name] = true
	}
	endpoint := clashProxyByName(t, doc, "same-same@e")
	hopName, _ := endpoint["dialer-proxy"].(string)
	if hopName == "" || hopName == "same-same@e" {
		t.Fatalf("dialer-proxy = %q, want a distinct hop name", hopName)
	}
	if hopProxy := clashProxyByName(t, doc, hopName); hopProxy["server"] != "hop.example.com" {
		t.Fatalf("dialer-proxy resolved to %v, want the hop", hopProxy)
	}
}

// Clash for Windows cannot dial through a hop, so a chained endpoint is dropped
// instead of silently connecting directly.
func TestClash_LegacyDropsChainedEndpoints(t *testing.T) {
	seedSubDB(t)
	// trojan is one of the few protocols the legacy filter keeps, so the drop
	// under test is the chain, not the protocol.
	seedLegacyTrojanInbound(t, "s1", "plain", 4925, 1)
	chained := seedLegacyTrojanInbound(t, "s1", "chained", 4926, 2)
	hop := seedChainProxy(t, &model.ChainProxy{Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true})
	assignChainInbound(t, chained.Id, hop.Id, 0)

	out, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClashLegacy("s1", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClashLegacy: %v", err)
	}
	doc := clashDoc(t, out)
	names := strings.Join(clashProxyNames(doc), ",")
	if !strings.Contains(names, "plain-plain@e") {
		t.Fatalf("unchained endpoint missing: %v", names)
	}
	if strings.Contains(names, "chained-chained@e") || strings.Contains(names, "entry") {
		t.Fatalf("legacy profile must not carry a chain: %v", names)
	}
}

// A profile whose every endpoint is chained has nothing the legacy format can
// express, so it reports that instead of shipping a direct-connection profile.
func TestClash_LegacyReportsWhenOnlyChainedEndpointsRemain(t *testing.T) {
	seedSubDB(t)
	only := seedLegacyTrojanInbound(t, "s2", "only-chained", 4927, 1)
	hop := seedChainProxy(t, &model.ChainProxy{Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true})
	assignChainInbound(t, only.Id, hop.Id, 0)

	_, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClashLegacy("s2", "sub.example.com")
	if !errors.Is(err, errNoLegacyClashProxies) {
		t.Fatalf("err = %v, want errNoLegacyClashProxies", err)
	}
}

// seedLegacyTrojanInbound creates a TLS trojan inbound the legacy filter keeps.
func seedLegacyTrojanInbound(t *testing.T, subId, tag string, port, subSortIndex int) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	email := tag + "@e"
	settings := fmt.Sprintf(`{"clients":[{"password":%q,"email":%q,"subId":%q,"enable":true}]}`, "pw-"+tag, email, subId)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: model.Trojan, Remark: tag, Settings: settings,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`,
		SubSortIndex:   subSortIndex,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed trojan inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Password: "pw-" + tag, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
	return ib
}

// The marker a plan is threaded through must never survive into the profile.
func TestClash_ChainMarkerNeverLeaks(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "leak", 4928, 1, wsTLSStream)
	hop := seedChainProxy(t, &model.ChainProxy{Remark: "entry", Type: "socks", Server: "hop.example.com", Port: 1080, Enabled: true})
	assignChainInbound(t, ib.Id, hop.Id, 0)

	if raw := chainYaml(t, "s1"); strings.Contains(raw, clashChainPlanKey) {
		t.Fatalf("internal marker in the profile:\n%s", raw)
	}
}

func toStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
