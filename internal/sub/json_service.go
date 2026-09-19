package sub

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

//go:embed default.json
var defaultJson string

// SubJsonService handles JSON subscription configuration generation and management.
type SubJsonService struct {
	configJson       map[string]any
	defaultOutbounds []json_util.RawMessage
	finalMask        string
	mux              string
	observatory      subBalancerObservatoryConfig

	// bakedRouting is re-resolved per request: a remote URL may be cold at
	// construction time and warm up later via the cron job.
	routingRules   string
	bakedRoutingMu sync.Mutex
	bakedRouting   *bakedRoutingState

	// dnsBlock is the panel DNS override, fixed for the service's lifetime.
	dnsBlock map[string]any

	SubService *SubService
}

type bakedRoutingState struct {
	spec       jsonRoutingSpec
	configJson map[string]any
}

// NewSubJsonService creates a new JSON subscription service with the given configuration.
func NewSubJsonService(mux string, rules string, finalMask string, routingRules string, subService *SubService) *SubJsonService {
	var configJson map[string]any
	var defaultOutbounds []json_util.RawMessage
	_ = json.Unmarshal([]byte(defaultJson), &configJson)
	if outboundSlices, ok := configJson["outbounds"].([]any); ok {
		for _, defaultOutbound := range outboundSlices {
			jsonBytes, _ := json.Marshal(defaultOutbound)
			defaultOutbounds = append(defaultOutbounds, jsonBytes)
		}
	}

	// A baked routing profile replaces the template's dns and routing subtrees
	// outright; the legacy simple-rules setting only applies without a profile.
	if routingRules == "" && rules != "" {
		var newRules []any
		routing, _ := configJson["routing"].(map[string]any)
		defaultRules, _ := routing["rules"].([]any)
		_ = json.Unmarshal([]byte(rules), &newRules)
		defaultRules = append(newRules, defaultRules...)
		routing["rules"] = defaultRules
		configJson["routing"] = routing
	}

	return &SubJsonService{
		configJson:       configJson,
		defaultOutbounds: defaultOutbounds,
		finalMask:        finalMask,
		mux:              mux,
		routingRules:     routingRules,
		observatory:      defaultSubBalancerObservatoryConfig(),
		SubService:       subService,
	}
}

// Re-resolved per call so an upstream edit reaches the documents without a
// restart; a failed resolve keeps the last good template.
func (s *SubJsonService) bakedTemplate() map[string]any {
	if s.routingRules == "" && s.dnsBlock == nil {
		return s.configJson
	}
	spec := resolveJsonRoutingSpec(s.routingRules)
	s.bakedRoutingMu.Lock()
	defer s.bakedRoutingMu.Unlock()
	if s.bakedRouting != nil {
		if spec.empty() || spec.equal(s.bakedRouting.spec) {
			return s.bakedRouting.configJson
		}
	} else if spec.empty() && s.dnsBlock == nil {
		return s.configJson
	}
	template := make(map[string]any, len(s.configJson)+2)
	maps.Copy(template, s.configJson)
	if !spec.empty() {
		applyJsonRouting(template, spec)
	}
	// The panel-level DNS block is an explicit choice, so it also replaces the
	// dns subtree a routing profile would otherwise bake in.
	if s.dnsBlock != nil {
		template["dns"] = s.dnsBlock
	}
	s.bakedRouting = &bakedRoutingState{spec: spec, configJson: template}
	return template
}

// GetJson generates a JSON subscription configuration for the given subscription ID and host.
func (s *SubJsonService) GetJson(subId string, host string, alwaysReturnArray bool) (string, string, error) {
	subReq := s.SubService.ForRequest(host)
	subReq.subscriptionBody = true
	inbounds, err := subReq.getInboundsBySubId(subId)
	if err != nil {
		return "", "", err
	}
	externalLinks, err := subReq.getClientExternalLinksBySubId(subId)
	if err != nil {
		return "", "", err
	}
	if len(inbounds) == 0 && len(externalLinks) == 0 {
		return "", "", nil
	}

	var header string
	var hasInactiveExternal bool
	var hasEnabledClient bool

	seenEmails := make(map[string]struct{})
	entries := make([]subConfigEntry, 0, len(inbounds))
	// Prepare Inbounds
	for _, inbound := range inbounds {
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		subReq.projectThroughFallbackMaster(inbound)
		if hostEps := subReq.hostEndpoints(inbound, "json"); len(hostEps) > 0 {
			injectExternalProxy(inbound, hostEps)
		}

		var inboundConfigs []json_util.RawMessage
		for _, client := range clients {
			if client.Enable {
				hasEnabledClient = true
			}
			seenEmails[client.Email] = struct{}{}
			inboundConfigs = append(inboundConfigs, s.getConfig(subReq, inbound, client, host)...)
		}
		if len(inboundConfigs) > 0 {
			entries = append(entries, subConfigEntry{
				sortIndex: inbound.SubSortIndex,
				id:        inbound.Id,
				configs:   inboundConfigs,
			})
		}
	}
	entries = s.appendBalancerEntries(entries)

	// Inbounds arrive sorted by (sub_sort_index, id); balancers interleave by
	// the same key and, on an equal number, follow the inbound group.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].sortIndex != entries[j].sortIndex {
			return entries[i].sortIndex < entries[j].sortIndex
		}
		if entries[i].kind != entries[j].kind {
			return entries[i].kind < entries[j].kind
		}
		return entries[i].id < entries[j].id
	})
	var configArray []json_util.RawMessage
	for _, entry := range entries {
		configArray = append(configArray, entry.configs...)
	}
	for _, ext := range externalLinks {
		if ext.Enable {
			hasEnabledClient = true
		}
		if !ext.Active {
			seenEmails[ext.Email] = struct{}{}
			hasInactiveExternal = true
			continue
		}
		for _, el := range expandEntry(ext) {
			outbound := parsedExternalOutbound(el.Link)
			if outbound == nil {
				continue
			}
			seenEmails[ext.Email] = struct{}{}
			remark := el.Name
			if remark == "" {
				remark = ext.Email
			}
			newOutbounds := []json_util.RawMessage{outbound}
			newOutbounds = append(newOutbounds, s.defaultOutbounds...)
			newConfigJson := make(map[string]any)
			maps.Copy(newConfigJson, s.bakedTemplate())
			newConfigJson["outbounds"] = newOutbounds
			newConfigJson["remarks"] = remark
			newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
			configArray = append(configArray, newConfig)
		}
	}

	if len(configArray) == 0 && !hasInactiveExternal {
		return "", "", nil
	}

	emails := make([]string, 0, len(seenEmails))
	for e := range seenEmails {
		emails = append(emails, e)
	}
	slices.Sort(emails)
	traffic, _ := subReq.AggregateTrafficByEmails(emails)
	traffic.Enable = hasEnabledClient
	header = subReq.subscriptionUserinfo(traffic)

	if mode, remark := subReq.resolveInfoNodeRemark(subId, emails, traffic, len(configArray) > 0); mode != infoNodeNone {
		dummyConfig := s.genDummySocksConfig(remark)
		if mode == infoNodeExpired || mode == infoNodeDepleted {
			configArray = []json_util.RawMessage{dummyConfig}
		} else {
			configArray = append([]json_util.RawMessage{dummyConfig}, configArray...)
		}
	}

	if len(configArray) == 0 {
		return "", header, nil
	}

	var finalJson []byte
	if len(configArray) == 1 && !alwaysReturnArray {
		finalJson, _ = json.MarshalIndent(configArray[0], "", "  ")
	} else {
		finalJson, _ = json.MarshalIndent(configArray, "", "  ")
	}

	return string(finalJson), header, nil
}

// GetSingBoxJson converts the panel's per-client JSON profiles to native sing-box configs.
// The existing /json/ format remains Xray-compatible; callers opt into this format
// explicitly with ?format=sing-box so existing subscriptions are not changed.
func (s *SubJsonService) GetSingBoxJson(subId string, host string, alwaysReturnArray bool) (string, string, error) {
	// Native subscriptions intentionally do not call GetJson(). We still reuse
	// the panel's mature per-client endpoint generation helpers internally, but
	// collect and translate the model-backed entries before an Xray document is
	// assembled. This keeps subscription filtering, node fallback, external
	// proxy handling and client eligibility identical to the panel's source of
	// truth without depending on serialized Xray subscription documents.
	subReq := s.SubService.ForRequest(host)
	subReq.subscriptionBody = true
	inbounds, err := subReq.getInboundsBySubId(subId)
	if err != nil {
		return "", "", err
	}
	externalLinks, err := subReq.getClientExternalLinksBySubId(subId)
	if err != nil {
		return "", "", err
	}
	if len(inbounds) == 0 && len(externalLinks) == 0 {
		return "", "", nil
	}

	type nativeOutbound struct {
		tag string
		out map[string]any
	}
	var proxies []nativeOutbound
	seenEmails := make(map[string]struct{})
	hasEnabledClient := false
	hasInactiveExternal := false

	var wireguardEndpoint map[string]any
	var wireguardAddresses []string
	for _, inbound := range inbounds {
		if inbound.Protocol != model.WireGuard {
			continue
		}
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		subReq.projectThroughFallbackMaster(inbound)
		var settings map[string]any
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)
		secretKey, _ := settings["secretKey"].(string)
		if secretKey == "" {
			continue
		}
		serverPublicKey := ""
		if pub, err := wgutil.PublicKeyFromPrivate(secretKey); err == nil {
			serverPublicKey = pub
		}
		for _, client := range clients {
			seenEmails[client.Email] = struct{}{}
			if !client.Enable || client.PrivateKey == "" || serverPublicKey == "" {
				continue
			}
			addresses := append([]string(nil), client.AllowedIPs...)
			if len(addresses) == 0 {
				addresses = []string{"10.0.0.2/32"}
			}
			peer := map[string]any{"address": inbound.Listen, "port": inbound.Port, "public_key": serverPublicKey, "allowed_ips": []string{"0.0.0.0/0", "::/0"}}
			if client.PreSharedKey != "" { peer["pre_shared_key"] = client.PreSharedKey }
			if ka := client.KeepAliveSeconds(); ka > 0 { peer["persistent_keepalive_interval"] = ka }
			wireguardEndpoint = map[string]any{"type":"wireguard","tag":"wg-endpoint","address":addresses,"private_key":client.PrivateKey,"peers":[]any{peer}}
			if mtu, ok := settings["mtu"].(float64); ok && mtu > 0 { wireguardEndpoint["mtu"] = int(mtu) }
			wireguardAddresses = addresses
			hasEnabledClient = true
			break
		}
		if wireguardEndpoint != nil { break }
	}
	if wireguardEndpoint != nil {
		cfg := map[string]any{
			"$schema":"https://sing-box.sagernet.org/schema.json",
			"endpoints":[]any{wireguardEndpoint},
			"inbounds":[]any{map[string]any{"type":"tun","tag":"tun-in","address":wireguardAddresses,"auto_route":true,"strict_route":true}},
			"outbounds":[]any{map[string]any{"type":"direct","tag":"direct"},map[string]any{"type":"block","tag":"blocked"}},
			"route":map[string]any{"rules":[]any{map[string]any{"action":"route","outbound":"wg-endpoint"}},"final":"direct","auto_detect_interface":true},
		}
		encoded, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil { return "", "", err }
		emails := make([]string, 0, len(seenEmails))
		for email := range seenEmails { emails = append(emails, email) }
		slices.Sort(emails)
		traffic, _ := subReq.AggregateTrafficByEmails(emails)
		traffic.Enable = hasEnabledClient
		header := subReq.subscriptionUserinfo(traffic)
		if alwaysReturnArray { arr,_:=json.MarshalIndent([]json.RawMessage{encoded},"","  "); return string(arr),header,nil }
		return string(encoded),header,nil
	}

	for _, inbound := range inbounds {
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		subReq.projectThroughFallbackMaster(inbound)
		if hostEps := subReq.hostEndpoints(inbound, "json"); len(hostEps) > 0 {
			injectExternalProxy(inbound, hostEps)
		}
		for _, client := range clients {
			seenEmails[client.Email] = struct{}{}
			if client.Enable {
				hasEnabledClient = true
			}
			for _, raw := range s.getConfig(subReq, inbound, client, host) {
				var xrayCfg map[string]any
				if err := json.Unmarshal(raw, &xrayCfg); err != nil {
					return "", "", err
				}
				outs, _ := xrayCfg["outbounds"].([]any)
				if len(outs) == 0 {
					continue
				}
				proxy, ok := outs[0].(map[string]any)
				if !ok {
					continue
				}
				var native map[string]any
				if nativeType, ok := proxy["type"].(string); ok && nativeType != "" {
					native = proxy
				} else {
					translated, err := singbox.TranslateXrayOutbound(proxy)
					if err != nil {
						return "", "", fmt.Errorf("client %q: %w", client.Email, err)
					}
					native = translated
				}
				tag := client.Email
				if tag == "" {
					tag = fmt.Sprintf("proxy-%d", len(proxies)+1)
				}
				if len(proxies) > 0 {
					tag = fmt.Sprintf("%s-%d", tag, len(proxies)+1)
				}
				native["tag"] = tag
				proxies = append(proxies, nativeOutbound{tag: tag, out: native})
			}
		}
	}

	for _, ext := range externalLinks {
		if ext.Enable {
			hasEnabledClient = true
		}
		if !ext.Active {
			seenEmails[ext.Email] = struct{}{}
			hasInactiveExternal = true
			continue
		}
		for _, el := range expandEntry(ext) {
			outbound := parsedExternalOutbound(el.Link)
			if outbound == nil {
				continue
			}
			rawOutbound, err := json.Marshal(outbound)
			if err != nil {
				return "", "", err
			}
			var xrayOutbound map[string]any
			if err := json.Unmarshal(rawOutbound, &xrayOutbound); err != nil {
				return "", "", err
			}
			native, err := singbox.TranslateXrayOutbound(xrayOutbound)
			if err != nil {
				return "", "", err
			}
			seenEmails[ext.Email] = struct{}{}
			tag := el.Name
			if tag == "" {
				tag = ext.Email
			}
			if tag == "" {
				tag = fmt.Sprintf("external-%d", len(proxies)+1)
			}
			native["tag"] = tag
			proxies = append(proxies, nativeOutbound{tag: tag, out: native})
		}
	}

	if len(proxies) == 0 && !hasInactiveExternal {
		return "", "", nil
	}

	outbounds := make([]any, 0, len(proxies)+4)
	proxyTags := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		outbounds = append(outbounds, proxy.out)
		proxyTags = append(proxyTags, proxy.tag)
	}
	outbounds = append(outbounds, map[string]any{"type":"direct","tag":"direct"})
	outbounds = append(outbounds, map[string]any{"type":"block","tag":"blocked"})
	if len(proxyTags) > 1 {
		outbounds = append(outbounds,
			map[string]any{"type":"urltest","tag":"auto","outbounds":proxyTags},
			map[string]any{"type":"selector","tag":"select","outbounds":proxyTags,"default":proxyTags[0]},
		)
	}

	sbCfg := map[string]any{
		"$schema": "https://sing-box.sagernet.org/schema.json",
		"outbounds": outbounds,
	}
	if template := s.bakedTemplate(); template != nil {
		if rawDNS, ok := template["dns"].(map[string]any); ok {
			if dns, err := singbox.TranslateXrayDNS(rawDNS); err == nil && len(dns) > 0 {
				sbCfg["dns"] = dns
			}
		}
		if rawRouting, ok := template["routing"].(map[string]any); ok {
			if route, err := singbox.TranslateXrayRouting(rawRouting); err == nil && len(route) > 0 {
				sbCfg["route"] = route
			}
		}
	}

	emails := make([]string, 0, len(seenEmails))
	for email := range seenEmails {
		emails = append(emails, email)
	}
	slices.Sort(emails)
	traffic, _ := subReq.AggregateTrafficByEmails(emails)
	traffic.Enable = hasEnabledClient
	header := subReq.subscriptionUserinfo(traffic)

	encoded, err := json.MarshalIndent(sbCfg, "", "  ")
	if err != nil {
		return "", header, err
	}
	if alwaysReturnArray {
		arr, _ := json.MarshalIndent([]json.RawMessage{encoded}, "", "  ")
		return string(arr), header, nil
	}
	return string(encoded), header, nil
}


// subConfigEntry is one ordered block of the JSON subscription: an inbound's
// configs (kind 0) or a balancer config (kind 1).
type subConfigEntry struct {
	sortIndex int
	kind      int
	id        int
	configs   []json_util.RawMessage
}

const (
	subBalancerTag      = "balancer"
	subBalancerProbeURL = "https://www.google.com/generate_204"
)

// subBalancerObservatoryConfig is the panel-wide burstObservatory ping config
// emitted into every client-side balancer doc (subJsonObservatory setting).
type subBalancerObservatoryConfig struct {
	Destination  string `json:"destination"`
	Connectivity string `json:"connectivity"`
	Interval     string `json:"interval"`
	Sampling     int    `json:"sampling"`
	Timeout      string `json:"timeout"`
	HTTPMethod   string `json:"httpMethod"`
}

func defaultSubBalancerObservatoryConfig() subBalancerObservatoryConfig {
	return subBalancerObservatoryConfig{
		Destination:  subBalancerProbeURL,
		Connectivity: "",
		Interval:     "1m",
		Sampling:     2,
		Timeout:      "5s",
		HTTPMethod:   "HEAD",
	}
}

// SetObservatoryConfig overrides defaults from the panel JSON setting. An empty
// cfg keeps all defaults; invalid values fall back with a warning, never panic.
func (s *SubJsonService) SetObservatoryConfig(cfg string) {
	s.observatory = defaultSubBalancerObservatoryConfig()
	if cfg == "" {
		return
	}
	var parsed subBalancerObservatoryConfig
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		logger.Warningf("subJsonObservatory: invalid JSON %q, using defaults: %v", cfg, err)
		return
	}
	if parsed.Destination != "" {
		if validProbeURL(parsed.Destination) {
			s.observatory.Destination = parsed.Destination
		} else {
			logger.Warningf("subJsonObservatory: invalid destination %q, keeping default %q", parsed.Destination, s.observatory.Destination)
		}
	}
	if parsed.Connectivity != "" {
		if validProbeURL(parsed.Connectivity) {
			s.observatory.Connectivity = parsed.Connectivity
		} else {
			logger.Warningf("subJsonObservatory: invalid connectivity %q, keeping default (skip)", parsed.Connectivity)
		}
	}
	if parsed.Interval != "" {
		if _, err := time.ParseDuration(parsed.Interval); err == nil {
			s.observatory.Interval = parsed.Interval
		} else {
			logger.Warningf("subJsonObservatory: invalid interval %q, keeping default %q", parsed.Interval, s.observatory.Interval)
		}
	}
	if parsed.Sampling > 0 {
		s.observatory.Sampling = parsed.Sampling
	}
	if parsed.Timeout != "" {
		if _, err := time.ParseDuration(parsed.Timeout); err == nil {
			s.observatory.Timeout = parsed.Timeout
		} else {
			logger.Warningf("subJsonObservatory: invalid timeout %q, keeping default %q", parsed.Timeout, s.observatory.Timeout)
		}
	}
	if parsed.HTTPMethod == "HEAD" || parsed.HTTPMethod == "GET" {
		s.observatory.HTTPMethod = parsed.HTTPMethod
	}
}

// validProbeURL accepts only absolute http(s) URLs so a malformed probe or
// connectivity value can't slip into the emitted burstObservatory.
func validProbeURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || u == nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func (s *SubJsonService) balancerObservatory(prefix string) map[string]any {
	o := s.observatory
	return map[string]any{
		"subjectSelector": []string{prefix},
		"pingConfig": map[string]any{
			"destination":  o.Destination,
			"connectivity": o.Connectivity,
			"interval":     o.Interval,
			"sampling":     o.Sampling,
			"timeout":      o.Timeout,
			"httpMethod":   o.HTTPMethod,
		},
	}
}

// appendBalancerEntries appends one entry per enabled balancer that has at
// least one member outbound among the inbound entries.
func (s *SubJsonService) appendBalancerEntries(entries []subConfigEntry) []subConfigEntry {
	balancers := getEnabledSubBalancers()
	if len(balancers) == 0 {
		return entries
	}
	// Pre-pass: pull each inbound doc's proxy outbound once so every balancer
	// reuses it instead of re-unmarshalling the whole document per balancer.
	entryProxies := make([][]map[string]any, len(entries))
	for i, entry := range entries {
		if entry.kind != 0 {
			continue
		}
		for _, config := range entry.configs {
			if proxy := extractProxyOutbound(config); proxy != nil {
				entryProxies[i] = append(entryProxies[i], proxy)
			}
		}
	}
	for i := range balancers {
		config := s.buildBalancerConfig(&balancers[i], entries, entryProxies)
		if config == nil {
			continue
		}
		entries = append(entries, subConfigEntry{
			sortIndex: balancers[i].SortOrder,
			kind:      1,
			id:        balancers[i].Id,
			configs:   []json_util.RawMessage{config},
		})
	}
	return entries
}

// extractProxyOutbound returns the first outbound of a document when it is the
// proxy (tag == "proxy"), else nil — the only member shape a balancer retags.
func extractProxyOutbound(config json_util.RawMessage) map[string]any {
	var doc map[string]any
	if json.Unmarshal(config, &doc) != nil {
		return nil
	}
	outbounds, _ := doc["outbounds"].([]any)
	if len(outbounds) == 0 {
		return nil
	}
	outbound, _ := outbounds[0].(map[string]any)
	if outbound == nil || outbound["tag"] != "proxy" {
		return nil
	}
	return outbound
}

func getEnabledSubBalancers() []model.SubBalancer {
	var balancers []model.SubBalancer
	if err := database.GetDB().Model(&model.SubBalancer{}).
		Where("enabled = ?", true).
		Order("sort_order asc, id asc").Find(&balancers).Error; err != nil {
		logger.Error("SubJsonService - getEnabledSubBalancers:", err)
		return nil
	}
	return balancers
}

// Suffix by proxy protocol, not transport network — a vmess/tcp member used to
// be mislabelled "vless".
func balancerMemberSuffix(protocol string) string {
	if protocol == "" {
		return "other"
	}
	return protocol
}

// balMember is one retagged member outbound and the inbound it came from.
type balMember struct {
	tag       string
	inboundId int
}

// leastLoadCosts builds xray's static strategy costs: higher value = picked
// less often; nil unless a member carries an explicit weight (all-1.0 bloat).
func leastLoadCosts(balancer *model.SubBalancer, members []balMember) []any {
	if balancer.Strategy != "leastLoad" || len(members) == 0 || len(balancer.MemberWeights) == 0 {
		return nil
	}
	costs := make([]any, 0, len(members))
	configured := false
	for _, m := range members {
		value := 1.0
		if weight, ok := balancer.MemberWeights[m.inboundId]; ok && weight > 0 {
			value = weight
			configured = true
		}
		// Anchored regexp: plain cost matching is substring-based in xray, so
		// an unanchored "bal-1-vless" would also swallow "bal-1-vless-2".
		costs = append(costs, map[string]any{
			"regexp": true,
			"match":  "^" + m.tag + "$",
			"value":  value,
		})
	}
	if !configured {
		return nil
	}
	return costs
}

// buildBalancerConfig assembles the balancer profile: members retagged under a
// per-balancer prefix, a routing.balancers entry, and (for leastPing/leastLoad) an observatory.
func (s *SubJsonService) buildBalancerConfig(balancer *model.SubBalancer, entries []subConfigEntry, entryProxies [][]map[string]any) json_util.RawMessage {
	prefix := fmt.Sprintf("bal-%d-", balancer.Id)
	usedTags := make(map[string]bool)
	var proxies []json_util.RawMessage
	// Members in emission order with their owning inbound, so costs[] can
	// reference the exact retagged tags assigned here.
	var members []balMember
	var firstTag string
	// entryProxies is the pre-extracted proxy outbounds per entry; kind!=0 rows
	// have none. Clone before retagging so the cached map stays reusable.
	for i, entry := range entries {
		if entry.kind != 0 || !slices.Contains(balancer.InboundIds, entry.id) {
			continue
		}
		for _, outbound := range entryProxies[i] {
			protocol, _ := outbound["protocol"].(string)
			base := prefix + balancerMemberSuffix(protocol)
			tag := base
			for suffix := 2; usedTags[tag]; suffix++ {
				tag = fmt.Sprintf("%s-%d", base, suffix)
			}
			usedTags[tag] = true
			member := maps.Clone(outbound)
			member["tag"] = tag
			if raw, err := json.MarshalIndent(member, "", "  "); err == nil {
				members = append(members, balMember{tag: tag, inboundId: entry.id})
				if firstTag == "" {
					firstTag = tag
				}
				proxies = append(proxies, raw)
			}
		}
	}
	if len(proxies) == 0 {
		return nil
	}

	outbounds := append([]json_util.RawMessage{}, proxies...)
	outbounds = append(outbounds, s.defaultOutbounds...)

	// One template per document: two resolves could straddle a profile refresh
	// and pair this document's dns with the other revision's routing.
	template := s.bakedTemplate()
	// Clone the shared routing subtree (and each rule map) before pointing
	// rules at the balancer.
	baseRouting, _ := template["routing"].(map[string]any)
	routing := make(map[string]any, len(baseRouting)+1)
	maps.Copy(routing, baseRouting)
	baseRules, _ := baseRouting["rules"].([]any)
	rules := make([]any, 0, len(baseRules)+1)
	for _, rule := range baseRules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			rules = append(rules, rule)
			continue
		}
		ruleMap = maps.Clone(ruleMap)
		if ruleMap["outboundTag"] == "proxy" {
			delete(ruleMap, "outboundTag")
			ruleMap["balancerTag"] = subBalancerTag
		}
		rules = append(rules, ruleMap)
	}
	routing["rules"] = rules
	isObservatory := balancer.Strategy == "leastPing" || balancer.Strategy == "leastLoad"
	strategyEntry := map[string]any{"type": balancer.Strategy}
	if costs := leastLoadCosts(balancer, members); costs != nil {
		strategyEntry["settings"] = map[string]any{"costs": costs}
	}
	balancerEntry := map[string]any{
		"tag":      subBalancerTag,
		"selector": []string{prefix},
		"strategy": strategyEntry,
	}
	if isObservatory && firstTag != "" {
		// With all probes failing, route to the first member instead of
		// failing dispatch.
		balancerEntry["fallbackTag"] = firstTag
	}
	routing["balancers"] = []any{balancerEntry}

	newConfigJson := make(map[string]any, len(template)+2)
	maps.Copy(newConfigJson, template)
	newConfigJson["outbounds"] = outbounds
	newConfigJson["remarks"] = balancer.Remark
	newConfigJson["routing"] = routing
	// leastPing/leastLoad require a burst observatory (Xray refuses to start
	// them without one); fallbackTag above covers the probe-outage case.
	if isObservatory {
		newConfigJson["burstObservatory"] = s.balancerObservatory(prefix)
	}

	config, _ := json.MarshalIndent(newConfigJson, "", "  ")
	return config
}

func (s *SubJsonService) getConfig(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
	var newJsonArray []json_util.RawMessage
	stream := s.streamData(inbound.StreamSettings, subKey(client))

	// When externalProxy is empty the JSON config falls back to a
	// synthetic one whose `dest` is the host the client connects to.
	// For node-managed inbounds we want the node's address — request
	// host won't reach the right xray. resolveInboundAddress already
	// implements the node→subscriber-host fallback chain.
	defaultDest := subReq.resolveInboundAddress(inbound)
	if defaultDest == "" {
		defaultDest = host
	}

	// Per-inbound xmux takes precedence over the global subJsonMux.
	// When xmux is present inside xhttpSettings, XHTTP multiplexing
	// is handled by xmux — don't also set the legacy outbound.Mux.
	mux := s.mux
	if xhttp, ok := stream["xhttpSettings"].(map[string]any); ok {
		if _, hasXmux := xhttp["xmux"]; hasXmux {
			mux = ""
		}
	}

	externalProxies, ok := stream["externalProxy"].([]any)
	hasExternalProxy := ok && len(externalProxies) > 0
	if !hasExternalProxy {
		externalProxies = []any{
			map[string]any{
				"forceTls": "same",
				"dest":     defaultDest,
				"port":     float64(inbound.Port),
				"remark":   "",
			},
		}
	}

	delete(stream, "externalProxy")
	network, _ := stream["network"].(string)

	for _, ep := range externalProxies {
		extPrxy, ok := ep.(map[string]any)
		if !ok {
			continue
		}
		// Expand the host's {{VAR}} remark template for this client (no-op for
		// the synthetic/legacy entry) before it's used as the config remark.
		subReq.renderHostRemark(inbound, client, extPrxy, network)
		inbound.Listen, _ = extPrxy["dest"].(string)
		if port, ok := extPrxy["port"].(float64); ok {
			inbound.Port = int(port)
		}
		newStream := cloneStreamForExternalProxy(stream)
		forceTls, _ := extPrxy["forceTls"].(string)
		switch forceTls {
		case "tls":
			if newStream["security"] != "tls" {
				newStream["security"] = "tls"
				newStream["tlsSettings"] = map[string]any{}
			}
		case "none":
			if newStream["security"] != "none" {
				newStream["security"] = "none"
				delete(newStream, "tlsSettings")
			}
		}
		security, _ := newStream["security"].(string)
		if hasExternalProxy {
			applyExternalProxyTLSToStream(extPrxy, newStream, security)
		}
		applyHostStreamOverrides(extPrxy, newStream)
		streamSettings, _ := json.MarshalIndent(newStream, "", "  ")
		hostMux := hostMuxOverride(extPrxy)

		var newOutbounds []json_util.RawMessage

		switch inbound.Protocol {
		case "vmess":
			if native := nativeVMessOutbound(inbound, newStream, client); native != nil {
				newOutbounds = append(newOutbounds, native)
			}
		case "vless":
			vc := client
			vc.ID = applyVlessRoute(client.ID, hostVlessRoute(extPrxy))
			// Same gate the raw link and the Clash proxy apply: a flow left
			// over from a transport Vision supported produces an outbound
			// xray refuses to start.
			newNetwork, _ := newStream["network"].(string)
			if vc.Flow != "" && !vlessFlowAllowed(newNetwork, security, subReq.linkSettings(inbound)) {
				vc.Flow = ""
			}
			if native := nativeVLESSOutbound(inbound, newStream, vc, subReq); native != nil {
				newOutbounds = append(newOutbounds, native)
			}
		case "trojan", "shadowsocks":
			if native := nativeServerOutbound(inbound, newStream, client, subReq); native != nil {
				newOutbounds = append(newOutbounds, native)
			}
		case "hysteria":
			if version := hysteriaVersion(inbound.Settings, newStream); version == 2 {
				if native := s.genNativeHysteria2(inbound, newStream, client); native != nil {
					newOutbounds = append(newOutbounds, native)
				}
			} else {
				newOutbounds = append(newOutbounds, s.genHy(inbound, newStream, client, jsonMux(mux, hostMux)))
			}
		case "tuic":
			if native := s.genNativeTUIC(inbound, newStream, client); native != nil {
				newOutbounds = append(newOutbounds, native)
			}
		case "wireguard":
			// Native sing-box WireGuard is an endpoint since 1.11. The legacy
			// JSON generator still emits the Xray-compatible outbound; native
			// subscription assembly handles WireGuard separately.
			continue
		case "amneziawg":
			continue
		}

		newOutbounds = append(newOutbounds, s.defaultOutbounds...)
		newConfigJson := make(map[string]any)
		maps.Copy(newConfigJson, s.bakedTemplate())

		transport, _ := newStream["network"].(string)
		newConfigJson["outbounds"] = newOutbounds
		newConfigJson["remarks"] = subReq.endpointRemark(inbound, client.Email, extPrxy, transport)

		newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
		newJsonArray = append(newJsonArray, newConfig)
	}

	return newJsonArray
}

func (s *SubJsonService) streamData(stream string, clientKey string) map[string]any {
	var streamSettings map[string]any
	if err := json.Unmarshal([]byte(stream), &streamSettings); err != nil || streamSettings == nil {
		streamSettings = map[string]any{}
	}
	security, _ := streamSettings["security"].(string)
	switch security {
	case "tls":
		if tlsSettings, ok := streamSettings["tlsSettings"].(map[string]any); ok {
			streamSettings["tlsSettings"] = s.tlsData(tlsSettings)
		} else {
			delete(streamSettings, "tlsSettings")
		}
	case "reality":
		if realitySettings, ok := streamSettings["realitySettings"].(map[string]any); ok {
			streamSettings["realitySettings"] = s.realityData(realitySettings, clientKey)
		} else {
			delete(streamSettings, "realitySettings")
		}
	}
	delete(streamSettings, "sockopt")

	if s.finalMask != "" {
		s.applyGlobalFinalMask(streamSettings)
	}

	// remove proxy protocol
	network, _ := streamSettings["network"].(string)
	switch network {
	case "tcp":
		streamSettings["tcpSettings"] = s.removeAcceptProxy(streamSettings["tcpSettings"])
	case "ws":
		streamSettings["wsSettings"] = s.removeAcceptProxy(streamSettings["wsSettings"])
	case "httpupgrade":
		streamSettings["httpupgradeSettings"] = s.removeAcceptProxy(streamSettings["httpupgradeSettings"])
	case "xhttp":
		streamSettings["xhttpSettings"] = s.removeAcceptProxy(streamSettings["xhttpSettings"])
		if xhttp, ok := streamSettings["xhttpSettings"].(map[string]any); ok {
			delete(xhttp, "noSSEHeader")
			delete(xhttp, "scMaxBufferedPosts")
			delete(xhttp, "scStreamUpServerSecs")
			delete(xhttp, "serverMaxHeaderBytes")
			// Values matching xray-core's own defaults stay off the wire:
			// old panels seeded them into every stored config and the
			// literal scMinPostsIntervalMs=30 is a DPI fingerprint (#5141).
			if v, _ := xhttp["scMaxEachPostBytes"].(string); v == "" || v == "1000000" {
				delete(xhttp, "scMaxEachPostBytes")
			}
			if v, _ := xhttp["scMinPostsIntervalMs"].(string); v == "" || v == "30" {
				delete(xhttp, "scMinPostsIntervalMs")
			}
		}
	}
	return streamSettings
}

func (s *SubJsonService) applyGlobalFinalMask(streamSettings map[string]any) {
	var fm map[string]any
	if err := json.Unmarshal([]byte(s.finalMask), &fm); err != nil || len(fm) == 0 {
		return
	}
	merged := mergeFinalMask(streamSettings["finalmask"], fm)
	if len(merged) > 0 {
		streamSettings["finalmask"] = merged
	}
}

func (s *SubJsonService) removeAcceptProxy(setting any) map[string]any {
	netSettings, ok := setting.(map[string]any)
	if ok {
		delete(netSettings, "acceptProxyProtocol")
	}
	return netSettings
}

func (s *SubJsonService) tlsData(tData map[string]any) map[string]any {
	tlsData := make(map[string]any, 1)
	tlsClientSettings, _ := tData["settings"].(map[string]any)

	tlsData["serverName"] = tData["serverName"]
	tlsData["alpn"] = tData["alpn"]
	if fingerprint, ok := tlsClientSettings["fingerprint"].(string); ok {
		tlsData["fingerprint"] = fingerprint
	}
	if cs, ok := tData["cipherSuites"].(string); ok && cs != "" {
		tlsData["cipherSuites"] = cs
	}
	if ech, ok := tlsClientSettings["echConfigList"].(string); ok && ech != "" {
		tlsData["echConfigList"] = ech
	}
	if vcn, ok := verifyPeerCertByNameValue(tlsClientSettings); ok {
		tlsData["verifyPeerCertByName"] = vcn
	}
	// xray-core now parses pinnedPeerCertSha256 as a comma-separated string, not
	// an array; emit the joined form so v2ray clients can import the config (#5401).
	if pins, ok := pinnedSha256List(tlsClientSettings); ok {
		tlsData["pinnedPeerCertSha256"] = strings.Join(pins, ",")
	}
	return tlsData
}

func (s *SubJsonService) realityData(rData map[string]any, clientKey string) map[string]any {
	rltyData := make(map[string]any, 1)
	rltyClientSettings, _ := rData["settings"].(map[string]any)

	rltyData["show"] = false
	rltyData["publicKey"] = rltyClientSettings["publicKey"]
	rltyData["fingerprint"] = rltyClientSettings["fingerprint"]
	rltyData["mldsa65Verify"] = rltyClientSettings["mldsa65Verify"]

	seed, _ := rltyClientSettings["spiderX"].(string)
	rltyData["spiderX"] = deriveSpiderX(seed, clientKey)
	shortIds, ok := rData["shortIds"].([]any)
	if ok && len(shortIds) > 0 {
		rltyData["shortId"], _ = shortIds[random.Num(len(shortIds))].(string)
	} else {
		rltyData["shortId"] = ""
	}
	serverNames, ok := rData["serverNames"].([]any)
	if ok && len(serverNames) > 0 {
		rltyData["serverName"], _ = serverNames[random.Num(len(serverNames))].(string)
	} else {
		rltyData["serverName"] = ""
	}

	return rltyData
}

// jsonMux picks the per-host mux override when present, else the global mux.
func jsonMux(global, override string) string {
	if override != "" {
		return override
	}
	return global
}

func (s *SubJsonService) genVnext(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, mux string) json_util.RawMessage {
	outbound := Outbound{
		Protocol: string(inbound.Protocol),
		Tag:      "proxy",
	}
	if mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	security := normalizeVmessSecurity(client.Security)
	outbound.Settings = map[string]any{
		"address":  inbound.Listen,
		"port":     inbound.Port,
		"id":       client.ID,
		"security": security,
		"level":    8,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genVless(subReq *SubService, inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, mux string) json_util.RawMessage {
	outbound := Outbound{
		Protocol: string(inbound.Protocol),
		Tag:      "proxy",
	}
	if mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	// Add encryption for VLESS outbound from inbound settings
	inboundSettings := subReq.linkSettings(inbound)
	encryption, _ := inboundSettings["encryption"].(string)

	settings := map[string]any{
		"address":    inbound.Listen,
		"port":       inbound.Port,
		"id":         client.ID,
		"encryption": encryption,
		"level":      8,
	}
	if client.Flow != "" && !inbound.DisableFlow {
		settings["flow"] = client.Flow
		outbound.Mux = muxWithoutTCP(mux)
	}
	outbound.Settings = settings
	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

// XTLS flows reject TCP mux.cool ("unexpected network TCP"); concurrency -1
// turns only that off and keeps the XUDP keys (Xray reads them under enabled).
func muxWithoutTCP(mux string) json_util.RawMessage {
	if mux == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(mux), &m); err != nil || m == nil {
		return nil
	}
	m["concurrency"] = -1
	out, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return json_util.RawMessage(out)
}

func (s *SubJsonService) genServer(subReq *SubService, inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, mux string) json_util.RawMessage {
	outbound := Outbound{}

	serverData := make([]ServerSetting, 1)
	serverData[0] = ServerSetting{
		Address:  inbound.Listen,
		Port:     inbound.Port,
		Level:    8,
		Password: client.Password,
	}

	if inbound.Protocol == model.Shadowsocks {
		inboundSettings := subReq.linkSettings(inbound)
		method, _ := inboundSettings["method"].(string)
		serverData[0].Method = method

		// server password in multi-user 2022 protocols
		if strings.HasPrefix(method, "2022") {
			if serverPassword, ok := inboundSettings["password"].(string); ok {
				serverData[0].Password = fmt.Sprintf("%s:%s", serverPassword, client.Password)
			}
		}
	}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	// Wrap the endpoint in a "servers" array (the standard Xray schema for
	// Shadowsocks/Trojan outbounds). The flat top-level form only parses on very
	// recent xray-core; older bundled cores (e.g. in v2rayN) reject it, so SS
	// links fail to connect. See genVnext/genVless for the VMess/VLESS shape.
	server := map[string]any{
		"address":  serverData[0].Address,
		"port":     serverData[0].Port,
		"password": serverData[0].Password,
		"level":    8,
	}
	if inbound.Protocol == model.Shadowsocks {
		server["method"] = serverData[0].Method
	}
	outbound.Settings = map[string]any{
		"servers": []any{server},
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func nativeTLSAndTransport(stream map[string]any) map[string]any {
	out := map[string]any{}
	security, _ := stream["security"].(string)
	if security == "tls" || security == "reality" {
		tlsSettings, _ := stream["tlsSettings"].(map[string]any)
		tls := map[string]any{"enabled": true}
		if v, _ := tlsSettings["serverName"].(string); v != "" { tls["server_name"] = v }
		if v, ok := tlsSettings["alpn"].([]any); ok && len(v) > 0 { tls["alpn"] = v }
		if v, ok := tlsSettings["allowInsecure"].(bool); ok { tls["insecure"] = v }
		if v, _ := tlsSettings["fingerprint"].(string); v != "" { tls["utls"] = map[string]any{"enabled": true, "fingerprint": v} }
		if security == "reality" {
			realitySettings, _ := tlsSettings["realitySettings"].(map[string]any)
			reality := map[string]any{"enabled": true}
			if v, _ := realitySettings["publicKey"].(string); v != "" { reality["public_key"] = v }
			if v, _ := realitySettings["shortId"].(string); v != "" { reality["short_id"] = v }
			if len(reality) > 1 { tls["reality"] = reality }
		}
		out["tls"] = tls
	}
	network, _ := stream["network"].(string)
	switch network {
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		tr := map[string]any{"type":"ws"}
		if v, _ := ws["path"].(string); v != "" { tr["path"] = v }
		if v, ok := ws["headers"].(map[string]any); ok && len(v) > 0 { tr["headers"] = v }
		out["transport"] = tr
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		tr := map[string]any{"type":"grpc"}
		if v, _ := grpc["serviceName"].(string); v != "" { tr["service_name"] = v }
		out["transport"] = tr
	case "http", "h2":
		httpSettings, _ := stream["httpSettings"].(map[string]any)
		tr := map[string]any{"type":"http"}
		if v, ok := httpSettings["host"].([]any); ok && len(v)>0 { tr["host"] = v }
		if v, _ := httpSettings["path"].(string); v != "" { tr["path"] = v }
		if v, _ := httpSettings["method"].(string); v != "" { tr["method"] = v }
		if v, ok := httpSettings["headers"].(map[string]any); ok && len(v)>0 { tr["headers"] = v }
		out["transport"] = tr
	case "httpupgrade":
		settings, _ := stream["httpupgradeSettings"].(map[string]any)
		tr := map[string]any{"type":"httpupgrade"}
		if v, _ := settings["host"].(string); v != "" { tr["host"] = v }
		if v, _ := settings["path"].(string); v != "" { tr["path"] = v }
		if v, ok := settings["headers"].(map[string]any); ok && len(v)>0 { tr["headers"] = v }
		out["transport"] = tr
	case "quic":
		settings, _ := stream["quicSettings"].(map[string]any)
		tr := map[string]any{"type":"quic"}
		if v, ok := settings["initial_packet_size"]; ok { tr["initial_packet_size"] = v }
		if v, ok := settings["disable_path_mtu_discovery"]; ok { tr["disable_path_mtu_discovery"] = v }
		out["transport"] = tr
	}
	return out
}

func nativeWireGuardEndpoint(inbound *model.Inbound, client model.Client, settings map[string]any) map[string]any {
	ep := map[string]any{
		"type": "wireguard",
		"tag": "wg-endpoint",
		"address": []any{},
		"private_key": "",
		"peers": []any{},
	}
	if v, _ := settings["address"].([]any); len(v) > 0 { ep["address"] = v }
	if v, _ := settings["privateKey"].(string); v != "" { ep["private_key"] = v }
	peer := map[string]any{
		"address": inbound.Listen,
		"port": inbound.Port,
		"public_key": client.PublicKey,
		"allowed_ips": []any{"0.0.0.0/0", "::/0"},
	}
	if v, _ := settings["mtu"].(float64); v > 0 { ep["mtu"] = int(v) }
	if v, _ := settings["reserved"].([]any); len(v) == 3 { peer["reserved"] = v }
	if v, _ := settings["preSharedKey"].(string); v != "" { peer["pre_shared_key"] = v }
	if v, _ := settings["persistentKeepalive"].(float64); v > 0 { peer["persistent_keepalive_interval"] = int(v) }
	ep["peers"] = []any{peer}
	return ep
}

func hysteriaVersion(settingsJSON string, stream map[string]any) int {
	var settings map[string]any
	_ = json.Unmarshal([]byte(settingsJSON), &settings)
	if version, ok := settings["version"].(float64); ok && int(version) == 2 {
		return 2
	}
	if version, ok := stream["version"].(float64); ok && int(version) == 2 {
		return 2
	}
	return 1
}

func (s *SubJsonService) genNativeHysteria2(inbound *model.Inbound, stream map[string]any, client model.Client) json_util.RawMessage {
	var settings map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	var hy map[string]any
	if v, ok := stream["hysteriaSettings"].(map[string]any); ok {
		hy = v
	}
	raw := map[string]any{
		"protocol": "hysteria2",
		"tag": "proxy",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address": inbound.Listen,
				"port": inbound.Port,
				"password": client.Auth,
			}},
		},
		"streamSettings": stream,
	}
	if rawSettings, ok := raw["settings"].(map[string]any); ok {
		for _, key := range []string{"up_mbps", "down_mbps", "hop_interval", "hop_interval_max", "bbr_profile", "disable_chrome_parrot"} {
			if v, exists := hy[key]; exists { rawSettings[key] = v }
		}
		if obfs, ok := hy["obfs"].(map[string]any); ok { rawSettings["obfs"] = obfs }
		if v, ok := hy["masquerade"]; ok { rawSettings["masquerade"] = v }
		if v, ok := hy["ignoreClientBandwidth"]; ok { rawSettings["ignore_client_bandwidth"] = v }
		if v, ok := hy["ignore_client_bandwidth"]; ok { rawSettings["ignore_client_bandwidth"] = v }
		if v, ok := hy["bbr_profile"]; ok { rawSettings["bbr_profile"] = v }
		if v, ok := hy["disable_chrome_parrot"]; ok { rawSettings["disable_chrome_parrot"] = v }
	}
	translated, err := singbox.TranslateXrayOutbound(raw)
	if err != nil { return nil }
	translated["tag"] = "proxy"
	b, _ := json.MarshalIndent(translated, "", "  ")
	return b
}

func (s *SubJsonService) genNativeTUIC(inbound *model.Inbound, stream map[string]any, client model.Client) json_util.RawMessage {
	raw := map[string]any{
		"protocol": "tuic",
		"tag": "proxy",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address": inbound.Listen,
				"port": inbound.Port,
				"id": client.ID,
				"uuid": client.ID,
				"password": client.Password,
			}},
		},
		"streamSettings": stream,
	}
	translated, err := singbox.TranslateXrayOutbound(raw)
	if err != nil { return nil }
	translated["tag"] = "proxy"
	b, _ := json.MarshalIndent(translated, "", "  ")
	return b
}

func (s *SubJsonService) genHy(inbound *model.Inbound, newStream map[string]any, client model.Client, mux string) json_util.RawMessage {
	outbound := Outbound{
		Protocol: string(inbound.Protocol),
		Tag:      "proxy",
	}

	if mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}

	var settings, stream map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	version, _ := settings["version"].(float64)
	outbound.Settings = map[string]any{
		"version": int(version),
		"address": inbound.Listen,
		"port":    inbound.Port,
	}

	_ = json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	hyStream, _ := stream["hysteriaSettings"].(map[string]any)
	outHyStream := map[string]any{
		"version": int(version),
		"auth":    client.Auth,
	}
	if udpIdleTimeout, ok := hyStream["udpIdleTimeout"].(float64); ok {
		outHyStream["udpIdleTimeout"] = int(udpIdleTimeout)
	}
	if masquerade, ok := hyStream["masquerade"].(map[string]any); ok {
		outHyStream["masquerade"] = masquerade
	}
	newStream["hysteriaSettings"] = outHyStream

	if finalmask, ok := hyStream["finalmask"].(map[string]any); ok {
		newStream["finalmask"] = mergeFinalMask(newStream["finalmask"], finalmask)
	}

	newStream["network"] = "hysteria"
	newStream["security"] = "tls"

	outbound.StreamSettings, _ = json.MarshalIndent(newStream, "", "  ")

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

// genWireguard builds an Xray wireguard outbound for a native WireGuard inbound,
// mirroring genWireguardLink: the peer public key is derived from the inbound
// secretKey, the client owns the private key / tunnel address / pre-shared key,
// and the peer routes the full tunnel. Returns nil when the client has no key.
func (s *SubJsonService) genWireguard(inbound *model.Inbound, client model.Client) json_util.RawMessage {
	if client.PrivateKey == "" {
		return nil
	}
	var settings map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	addresses := append([]string(nil), client.AllowedIPs...)
	if len(addresses) == 0 {
		addresses = []string{"10.0.0.2/32"}
	}
	peer := map[string]any{
		"address": inbound.Listen,
		"port": inbound.Port,
		"public_key": client.PublicKey,
		"allowed_ips": []string{"0.0.0.0/0", "::/0"},
	}
	if client.PreSharedKey != "" { peer["pre_shared_key"] = client.PreSharedKey }
	if ka := client.KeepAliveSeconds(); ka > 0 { peer["persistent_keepalive_interval"] = ka }
	endpoint := map[string]any{
		"type": "wireguard",
		"tag": "wg-endpoint",
		"address": addresses,
		"private_key": client.PrivateKey,
		"peers": []any{peer},
	}
	if mtu, ok := settings["mtu"].(float64); ok && mtu > 0 { endpoint["mtu"] = int(mtu) }
	result, _ := json.MarshalIndent(map[string]any{"endpoint": endpoint, "outbound": map[string]any{"type":"direct","tag":"proxy"}}, "", "  ")
	return result
}

func (s *SubJsonService) genDummySocksConfig(remark string) json_util.RawMessage {
	outbound := map[string]any{
		"protocol": "socks",
		"tag":      "proxy",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address": "127.0.0.1",
					"port":    1080,
				},
			},
		},
	}
	rawOutbound, _ := json.Marshal(outbound)
	newOutbounds := []json_util.RawMessage{rawOutbound}
	newOutbounds = append(newOutbounds, s.defaultOutbounds...)

	newConfigJson := make(map[string]any)
	maps.Copy(newConfigJson, s.configJson)
	if s.dnsBlock != nil {
		newConfigJson["dns"] = s.dnsBlock
	}
	newConfigJson["outbounds"] = newOutbounds
	newConfigJson["remarks"] = remark

	newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
	return newConfig
}

func mergeFinalMask(base any, extra map[string]any) map[string]any {
	merged := map[string]any{}
	if baseMap, ok := base.(map[string]any); ok {
		for key, value := range baseMap {
			switch key {
			case "tcp", "udp":
				if masks, ok := value.([]any); ok {
					merged[key] = append([]any(nil), masks...)
				}
			default:
				merged[key] = value
			}
		}
	}

	for key, value := range extra {
		switch key {
		case "tcp", "udp":
			baseMasks, _ := merged[key].([]any)
			extraMasks, _ := value.([]any)
			if len(extraMasks) > 0 {
				merged[key] = append(baseMasks, extraMasks...)
			}
		case "quicParams":
			if _, exists := merged[key]; !exists {
				merged[key] = value
			}
		default:
			merged[key] = value
		}
	}

	return merged
}

type Outbound struct {
	Protocol       string               `json:"protocol"`
	Tag            string               `json:"tag"`
	StreamSettings json_util.RawMessage `json:"streamSettings"`
	Mux            json_util.RawMessage `json:"mux,omitempty"`
	Settings       map[string]any       `json:"settings,omitempty"`
}

type ServerSetting struct {
	Password string `json:"password"`
	Level    int    `json:"level"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Flow     string `json:"flow,omitempty"`
	Method   string `json:"method,omitempty"`
}
