package sub

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// A client-side chain makes the subscriber's client dial this panel's inbound
// through one upstream SOCKS/HTTP proxy (model.ChainProxy), or through one of
// several picked by strategy (model.ChainSelector). Xray expresses the hop with
// streamSettings.sockopt.dialerProxy, mihomo with the proxy's dialer-proxy.
//
// dialerProxy resolves an outbound tag only — the bundled core has no balancer
// support there — so a balanced chain emits one endpoint copy per hop, each
// dialing through its own hop, and routes through a balancer over those copies.
const (
	// chainBalancerPrefix is the routing.balancers tag per selector; the
	// endpoint copies are tagged "<prefix>-<id>-<index>", so a balancer never
	// selects itself.
	chainBalancerPrefix = "chain"
	chainVariantPrefix  = "chain-"
	chainHopPrefix      = "hop-"
	chainGroupIntervalS = 300
	chainGroupTolerance = 50
)

// chainableProtocol reports whether a chain applies to an inbound protocol. The
// QUIC-based and tunnel protocols dial without streamSettings, and share links
// have no field for a hop, so the assignment is rejected instead of ignored.
func chainableProtocol(protocol model.Protocol) bool {
	switch protocol {
	case model.VMESS, model.VLESS, model.Trojan, model.Shadowsocks:
		return true
	default:
		return false
	}
}

func chainHopTag(proxyId int) string {
	return fmt.Sprintf("%s%d", chainHopPrefix, proxyId)
}

// chainBalancerTag names a selector's routing balancer in a JSON document.
func chainBalancerTag(selectorId int) string {
	return fmt.Sprintf("%s-%d", chainBalancerPrefix, selectorId)
}

// chainVariantTag names one endpoint copy: the copy dials the hop at the same
// index, and the balancer selects among them by the prefix "chain-<id>-".
func chainVariantTag(selectorId, index int) string {
	return fmt.Sprintf("%s-%d-%d", chainBalancerPrefix, selectorId, index)
}

func chainVariantPrefixFor(selectorId int) string {
	return chainBalancerTag(selectorId) + "-"
}

// chainPlan is the resolved chain of one inbound: the hops to dial through, in
// emission order, and the selector that chose them (nil for a single proxy).
type chainPlan struct {
	selector *model.ChainSelector
	hops     []model.ChainProxy
}

// direct reports that no chain is assigned, so the endpoint dials directly.
func (p *chainPlan) direct() bool { return p == nil }

// unusable reports an assignment that resolved to no enabled hop. The endpoint
// must then be left out of the subscription entirely: emitting it would silently
// send the subscriber's traffic around the chain it was assigned.
func (p *chainPlan) unusable() bool { return p != nil && len(p.hops) == 0 }

// balanced reports a selector with more than one usable hop; a one-hop selector
// needs no routing balancer and emits like a single-proxy chain.
func (p *chainPlan) balanced() bool { return p != nil && p.selector != nil && len(p.hops) > 1 }

func (p *chainPlan) strategy() string {
	if p == nil || p.selector == nil {
		return ""
	}
	return p.selector.Strategy
}

// chainEndpoint is one copy of an endpoint outbound and the hop it dials.
type chainEndpoint struct {
	tag    string
	hopTag string
}

// endpoints lists the endpoint copies the plan needs: exactly one dialing
// directly, one dialing the single hop, or one per hop for a balanced selector.
// An unusable plan lists none — emitting a hop-free endpoint would bypass the
// chain, so callers drop the endpoint instead.
func (p *chainPlan) endpoints() []chainEndpoint {
	if p.direct() {
		return []chainEndpoint{{tag: "proxy"}}
	}
	if p.unusable() {
		return nil
	}
	if p.balanced() {
		out := make([]chainEndpoint, len(p.hops))
		for i := range p.hops {
			out[i] = chainEndpoint{tag: chainVariantTag(p.selector.Id, i), hopTag: chainHopTag(p.hops[i].Id)}
		}
		return out
	}
	return []chainEndpoint{{tag: "proxy", hopTag: chainHopTag(p.hops[0].Id)}}
}

type chainPlanKey struct {
	proxyId    int
	selectorId int
}

// chainAssignmentFor resolves an inbound's chain once per subscription render.
// The cache key is the assignment itself, so an unsaved inbound (id 0) resolves too.
func (s *SubService) chainAssignmentFor(inbound *model.Inbound) *chainPlan {
	if inbound == nil || (inbound.ChainProxyId <= 0 && inbound.ChainSelectorId <= 0) {
		return nil
	}
	// A stale assignment left on a protocol that cannot carry a hop (set before
	// the save-time check existed, or pushed by a node) is ignored, not honored.
	if !chainableProtocol(inbound.Protocol) {
		return nil
	}
	key := chainPlanKey{proxyId: inbound.ChainProxyId, selectorId: inbound.ChainSelectorId}
	if plan, ok := s.chainPlans[key]; ok {
		return plan
	}
	plan := chainPlanFor(inbound)
	if s.chainPlans == nil {
		s.chainPlans = map[chainPlanKey]*chainPlan{}
	}
	s.chainPlans[key] = plan
	return plan
}

func chainPlanFor(inbound *model.Inbound) *chainPlan {
	db := database.GetDB()
	if db == nil {
		return &chainPlan{}
	}
	if inbound.ChainSelectorId > 0 {
		var selector model.ChainSelector
		if err := db.First(&selector, inbound.ChainSelectorId).Error; err != nil {
			logger.Warningf("sub: chain selector %d for inbound %d: %v", inbound.ChainSelectorId, inbound.Id, err)
			return &chainPlan{}
		}
		if !selector.Enabled {
			return &chainPlan{}
		}
		return &chainPlan{selector: &selector, hops: enabledChainHops(selector.ProxyIds)}
	}
	var proxy model.ChainProxy
	if err := db.First(&proxy, inbound.ChainProxyId).Error; err != nil {
		logger.Warningf("sub: chain proxy %d for inbound %d: %v", inbound.ChainProxyId, inbound.Id, err)
		return &chainPlan{}
	}
	if !proxy.Enabled {
		return &chainPlan{}
	}
	return &chainPlan{hops: []model.ChainProxy{proxy}}
}

// enabledChainHops loads the selectable hops in the selector's configured order,
// skipping disabled or since-deleted proxies.
func enabledChainHops(proxyIds []int) []model.ChainProxy {
	ids := make([]int, 0, len(proxyIds))
	seen := make(map[int]bool, len(proxyIds))
	for _, id := range proxyIds {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	var proxies []model.ChainProxy
	if err := database.GetDB().Order("id asc").Find(&proxies, ids).Error; err != nil {
		logger.Warningf("sub: chain selector hops %v: %v", ids, err)
		return nil
	}
	byId := make(map[int]model.ChainProxy, len(proxies))
	for _, proxy := range proxies {
		byId[proxy.Id] = proxy
	}
	hops := make([]model.ChainProxy, 0, len(ids))
	for _, id := range ids {
		proxy, ok := byId[id]
		if !ok || !proxy.Enabled {
			continue
		}
		hops = append(hops, proxy)
	}
	return hops
}

// chainHopName is a hop's subscriber-visible name: its remark, else an endpoint
// label. mihomo needs it unique — the Clash renderer dedupes.
func chainHopName(proxy *model.ChainProxy) string {
	if proxy.Remark != "" {
		return proxy.Remark
	}
	return fmt.Sprintf("%s-%s-%d", proxy.Type, proxy.Server, proxy.Port)
}

func chainHopTLS(proxy *model.ChainProxy) bool {
	return proxy.Type == "http" && proxy.TLS
}

// chainHopOutbound builds the xray outbound JSON that dials through one hop.
// socks and http share the servers[].users[] shape, so one builder covers both.
// TLS verification stays on: this core removed allowInsecure, and the panel's
// other JSON output never emits it either.
func chainHopOutbound(proxy *model.ChainProxy) map[string]any {
	server := map[string]any{
		"address": proxy.Server,
		"port":    proxy.Port,
	}
	if proxy.Username != "" || proxy.Password != "" {
		server["users"] = []any{map[string]any{"user": proxy.Username, "pass": proxy.Password}}
	}
	outbound := map[string]any{
		"tag":      chainHopTag(proxy.Id),
		"protocol": proxy.Type,
		"settings": map[string]any{"servers": []any{server}},
	}
	if chainHopTLS(proxy) {
		// A bare security block: xray defaults serverName to the address.
		outbound["streamSettings"] = map[string]any{"security": "tls"}
	}
	return outbound
}

// chainHopClashProxy builds the mihomo proxy that dials through one hop.
func chainHopClashProxy(proxy *model.ChainProxy, name string) map[string]any {
	protocol := "socks5"
	if proxy.Type == "http" {
		protocol = "http"
	}
	hop := map[string]any{
		"name":   name,
		"type":   protocol,
		"server": proxy.Server,
		"port":   proxy.Port,
	}
	// HTTP CONNECT has no UDP relay, so only SOCKS5 advertises it.
	if protocol == "socks5" {
		hop["udp"] = true
	}
	if proxy.Username != "" || proxy.Password != "" {
		hop["username"] = proxy.Username
		hop["password"] = proxy.Password
	}
	if chainHopTLS(proxy) {
		hop["tls"] = true
		hop["servername"] = proxy.Server
	}
	return hop
}

// chainClashGroupType maps a chain selector's xray strategy onto the one mihomo
// group that behaves the same way. mihomo has no random group, and its
// load-balance spreads connections the way xray's random spreads requests.
func chainClashGroupType(strategy string) (kind string, loadBalanceStrategy string) {
	switch strategy {
	case "leastPing":
		return "url-test", ""
	case "roundRobin", "random":
		return "load-balance", "round-robin"
	default:
		return "load-balance", "consistent-hashing"
	}
}
