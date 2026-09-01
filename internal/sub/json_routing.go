package sub

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// jsonRoutingSpec is the canonical form of the generic Happ/INCY routing
// payload (inline JSON, happ:// or incy:// deeplink, or remote URL).
type jsonRoutingSpec struct {
	DomainStrategy    string
	RemoteDNSDomain   string
	RemoteDNSIP       string
	DomesticDNSDomain string
	DomesticDNSIP     string
	DnsHosts          map[string]string
	RouteOrder        []string // e.g. {"block","proxy","direct"}; default {"block","direct","proxy"}
	DirectSites       []string
	DirectIp          []string
	ProxySites        []string
	ProxyIp           []string
	BlockSites        []string
	BlockIp           []string
}

func (s jsonRoutingSpec) empty() bool {
	return s.DomainStrategy == "" && s.RemoteDNSDomain == "" && s.RemoteDNSIP == "" &&
		s.DomesticDNSDomain == "" && s.DomesticDNSIP == "" && len(s.DnsHosts) == 0 &&
		len(s.RouteOrder) == 0 && len(s.DirectSites) == 0 && len(s.DirectIp) == 0 &&
		len(s.ProxySites) == 0 && len(s.ProxyIp) == 0 && len(s.BlockSites) == 0 && len(s.BlockIp) == 0
}

var jsonRoutingDeeplinkPrefixes = []string{"happ://routing/onadd/", "incy://routing/onadd/"}

// resolveJsonRoutingSpec parses the routing payload, degrading to an empty
// spec on error — a bad setting must never take the subscription server down.
func resolveJsonRoutingSpec(raw string) jsonRoutingSpec {
	spec, remote, err := parseJsonRoutingSpec(raw)
	if err != nil {
		if remote {
			logger.Warning("subJsonRoutingRules: remote source unavailable, emitting default routing")
		} else {
			logger.Warning("subJsonRoutingRules: ", err)
		}
		return jsonRoutingSpec{}
	}
	return spec
}

// parseJsonRoutingSpec resolves raw (inline JSON, happ:// or incy:// deeplink,
// or https:// URL) into a spec. Remote URLs go through the shared
// remoteRoutingResolver cache; the caller degrades on error, never fails.
func parseJsonRoutingSpec(raw string) (jsonRoutingSpec, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return jsonRoutingSpec{}, false, nil
	}

	if _, remote, err := common.ParseRemoteRoutingURL(trimmed); remote || (err != nil && trimmed != raw) {
		if err != nil {
			return jsonRoutingSpec{}, true, err
		}
		resolved, remote, err := resolveRoutingSource(remoteRoutingHapp, trimmed)
		if err != nil || !remote {
			return jsonRoutingSpec{}, true, err
		}
		trimmed = resolved
	}

	payload := []byte(trimmed)
	if _, rest, ok := cutAnyPrefix(trimmed, jsonRoutingDeeplinkPrefixes); ok {
		decoded, err := decodeRoutingBase64(rest)
		if err != nil {
			return jsonRoutingSpec{}, false, fmt.Errorf("invalid routing deeplink payload: %w", err)
		}
		payload = decoded
	} else if !strings.HasPrefix(trimmed, "{") {
		return jsonRoutingSpec{}, false, errors.New("routing payload must be a JSON object or a happ/incy deeplink")
	}

	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return jsonRoutingSpec{}, false, fmt.Errorf("invalid routing payload JSON: %w", err)
	}
	if object == nil {
		return jsonRoutingSpec{}, false, errors.New("routing payload must be a JSON object")
	}

	spec, err := buildJsonRoutingSpec(object)
	if err != nil {
		return jsonRoutingSpec{}, false, err
	}
	return spec, false, nil
}

func cutAnyPrefix(s string, prefixes []string) (string, string, bool) {
	for _, prefix := range prefixes {
		if after, ok := strings.CutPrefix(s, prefix); ok {
			return prefix, after, true
		}
	}
	return "", "", false
}

func buildJsonRoutingSpec(object map[string]any) (jsonRoutingSpec, error) {
	spec := jsonRoutingSpec{}
	var err error
	if spec.DomainStrategy, err = routingString(object, "DomainStrategy"); err != nil {
		return spec, err
	}
	if spec.RemoteDNSDomain, err = routingString(object, "RemoteDNSDomain"); err != nil {
		return spec, err
	}
	if spec.RemoteDNSIP, err = routingString(object, "RemoteDNSIP"); err != nil {
		return spec, err
	}
	if spec.DomesticDNSDomain, err = routingString(object, "DomesticDNSDomain"); err != nil {
		return spec, err
	}
	if spec.DomesticDNSIP, err = routingString(object, "DomesticDNSIP"); err != nil {
		return spec, err
	}
	if spec.DirectSites, err = routingList(object, "DirectSites"); err != nil {
		return spec, err
	}
	if spec.DirectIp, err = routingList(object, "DirectIp"); err != nil {
		return spec, err
	}
	if spec.ProxySites, err = routingList(object, "ProxySites"); err != nil {
		return spec, err
	}
	if spec.ProxyIp, err = routingList(object, "ProxyIp"); err != nil {
		return spec, err
	}
	if spec.BlockSites, err = routingList(object, "BlockSites"); err != nil {
		return spec, err
	}
	if spec.BlockIp, err = routingList(object, "BlockIp"); err != nil {
		return spec, err
	}
	if spec.DnsHosts, err = routingHosts(object, "DnsHosts"); err != nil {
		return spec, err
	}
	order, err := routingString(object, "RouteOrder")
	if err != nil {
		return spec, err
	}
	for _, segment := range strings.Split(order, "-") {
		switch segment {
		case "block", "proxy", "direct":
			spec.RouteOrder = append(spec.RouteOrder, segment)
		}
	}
	return spec, nil
}

func routingString(object map[string]any, key string) (string, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("routing field %q must be a string", key)
	}
	return text, nil
}

func routingList(object map[string]any, key string) ([]string, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return nil, nil
	}
	entries, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("routing field %q must be an array of strings", key)
	}
	list := make([]string, 0, len(entries))
	for _, entry := range entries {
		text, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("routing field %q must be an array of strings", key)
		}
		list = append(list, text)
	}
	return list, nil
}

func routingHosts(object map[string]any, key string) (map[string]string, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return nil, nil
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("routing field %q must be a string-to-string map", key)
	}
	hosts := make(map[string]string, len(raw))
	for name, entry := range raw {
		address, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("routing field %q must be a string-to-string map", key)
		}
		hosts[name] = address
	}
	return hosts, nil
}

func routeOrderGroups(order []string) []string {
	if len(order) == 0 {
		return []string{"block", "direct", "proxy"}
	}
	return order
}

// applyJsonRouting patches the base template with the spec's dns and routing
// subtrees, mirroring the njs patcher panel admins previously ran behind nginx.
func applyJsonRouting(configJson map[string]any, spec jsonRoutingSpec) {
	domestic := spec.DomesticDNSDomain
	if domestic == "" {
		ip := spec.DomesticDNSIP
		if ip == "" {
			ip = "77.88.8.8"
		}
		domestic = "https://" + ip + "/dns-query"
	}
	remote := spec.RemoteDNSDomain
	if remote == "" {
		ip := spec.RemoteDNSIP
		if ip == "" {
			ip = "8.8.8.8"
		}
		remote = "https://" + ip + "/dns-query"
	}

	dns := map[string]any{
		"tag":           "dns_out",
		"queryStrategy": "UseIP",
		"servers":       []any{},
	}
	if len(spec.DirectSites) > 0 {
		dns["servers"] = append(dns["servers"].([]any), map[string]any{
			"address": domestic,
			"domains": spec.DirectSites,
		})
	}
	dns["servers"] = append(dns["servers"].([]any), map[string]any{
		"address":      remote,
		"skipFallback": false,
	})
	if len(spec.DnsHosts) > 0 {
		dns["hosts"] = spec.DnsHosts
	}

	domainStrategy := spec.DomainStrategy
	if domainStrategy == "" {
		domainStrategy = "IPIfNonMatch"
	}

	groups := map[string][]map[string]any{
		"block": {
			{"domain": stringList(spec.BlockSites), "outboundTag": "block"},
			{"ip": stringList(spec.BlockIp), "outboundTag": "block"},
		},
		"direct": {
			{"domain": stringList(spec.DirectSites), "outboundTag": "direct"},
			{"ip": stringList(spec.DirectIp), "outboundTag": "direct"},
		},
		"proxy": {
			{"domain": stringList(spec.ProxySites), "outboundTag": "proxy"},
			{"ip": stringList(spec.ProxyIp), "outboundTag": "proxy"},
		},
	}
	rules := make([]any, 0, len(routeOrderGroups(spec.RouteOrder))*2+1)
	for _, group := range routeOrderGroups(spec.RouteOrder) {
		for _, rule := range groups[group] {
			var key string
			if _, ok := rule["domain"]; ok {
				key = "domain"
			} else {
				key = "ip"
			}
			if len(rule[key].([]string)) == 0 {
				continue
			}
			entry := map[string]any{"type": "field", key: rule[key], "outboundTag": rule["outboundTag"]}
			rules = append(rules, entry)
		}
	}
	rules = append(rules, map[string]any{"type": "field", "network": "tcp,udp", "outboundTag": "proxy"})

	configJson["dns"] = dns
	configJson["routing"] = map[string]any{
		"domainStrategy": domainStrategy,
		"rules":          rules,
	}
}

func stringList(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	return list
}
