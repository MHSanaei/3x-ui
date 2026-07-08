package service

import (
	"encoding/json"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

// dnsAllowRuleShape identifies routing rules this file manages: a plain
// "type=field, ip=[...], port=..., outboundTag=direct" rule with no other
// matchers. An "enabled" key is tolerated as long as it's true — the
// Routing tab's rule editor (RuleFormModal.tsx submit()) and its enabled
// switch (RoutingTab.tsx toggleRule()) always write that key back, even
// when nothing else changed, so requiring its absence would disown the
// rule the first time an admin so much as opens it in the UI. A rule
// toggled off (enabled=false) is treated as no longer ours: the admin
// explicitly turned it off, and re-enabling it on the next save would
// silently override that choice.
//
// Rules shaped like this are kept in sync with the current dns.servers
// config on every save; anything else (including rules an admin wrote by
// hand that happen to also allow-list an IP) is left untouched.
func dnsAllowRuleShape(rule map[string]any) bool {
	if t, _ := rule["type"].(string); t != "field" {
		return false
	}
	if out, _ := rule["outboundTag"].(string); out != "direct" {
		return false
	}
	if _, ok := rule["ip"]; !ok {
		return false
	}
	if _, ok := rule["port"]; !ok {
		return false
	}
	for key := range rule {
		switch key {
		case "type", "outboundTag", "ip", "port":
			continue
		case "enabled":
			if enabled, ok := rule[key].(bool); !ok || !enabled {
				return false
			}
			continue
		default:
			return false
		}
	}
	return true
}

// findPrivateBlockRule returns the index of a routing rule that blocks
// geoip:private (the panel's default anti-SSRF rule), or -1 if none is
// present. Matched by shape (outboundTag=blocked, ip contains
// "geoip:private") rather than position, since admins can reorder rules.
func findPrivateBlockRule(rules []map[string]any) int {
	for i, rule := range rules {
		if out, _ := rule["outboundTag"].(string); out != "blocked" {
			continue
		}
		for _, ip := range readRuleIPs(rule["ip"]) {
			if strings.EqualFold(ip, "geoip:private") {
				return i
			}
		}
	}
	return -1
}

func readRuleIPs(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

// dnsServerEndpoint is a literal (ip, port) pair extracted from a
// dns.servers entry.
type dnsServerEndpoint struct {
	ip   string
	port int
}

// privateDnsServerEndpoint extracts a literal, private/internal (ip, port)
// endpoint from a dns.servers entry, or ok=false if the entry is a domain
// name, a special Xray keyword (localhost, fakedns, ...), or resolves to a
// public IP.
//
// A dns.servers entry is either a bare string or an object with an
// "address" field (see frontend/src/schemas/dns.ts DnsServerEntrySchema);
// the object form may also carry an explicit "port" (default 53 there,
// per DnsServerObjectInnerSchema), which takes precedence over any port
// embedded in the address itself.
func privateDnsServerEndpoint(entry any) (dnsServerEndpoint, bool) {
	var address string
	explicitPort := 0
	switch v := entry.(type) {
	case string:
		address = v
	case map[string]any:
		address, _ = v["address"].(string)
		if p, ok := v["port"].(float64); ok && p > 0 {
			explicitPort = int(p)
		}
	default:
		return dnsServerEndpoint{}, false
	}

	host, port := splitAddressHostPort(address)
	if host == "" {
		return dnsServerEndpoint{}, false
	}
	if explicitPort > 0 {
		port = explicitPort
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Domain name, or a special keyword like "localhost"/"fakedns" —
		// neither is something we can safely allow-list by IP here.
		return dnsServerEndpoint{}, false
	}
	if !netsafe.IsBlockedIP(ip) {
		return dnsServerEndpoint{}, false
	}
	return dnsServerEndpoint{ip: ip.String(), port: port}, true
}

// splitAddressHostPort extracts the bare host and port (defaulting to 53)
// from an Xray-core DNS server address string. Those may carry a URI
// scheme (tcp://, tcp+local://, https://, https+local://, quic://,
// quic+local://) and, for DoH, a path and/or a bracketed IPv6 host — all
// of that is stripped down to host[:port] before parsing.
func splitAddressHostPort(address string) (host string, port int) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", 0
	}

	if idx := strings.Index(address, "://"); idx != -1 {
		address = address[idx+3:]
	}
	// Drop a DoH path, e.g. "1.1.1.1/dns-query".
	if idx := strings.Index(address, "/"); idx != -1 {
		address = address[:idx]
	}

	port = 53
	host = address
	if strings.HasPrefix(host, "[") {
		// Bracketed IPv6, with or without a port: "[::1]" / "[::1]:53".
		end := strings.Index(host, "]")
		if end == -1 {
			return host, port
		}
		rest := host[end+1:]
		host = host[1:end]
		if p, ok := strings.CutPrefix(rest, ":"); ok {
			if n, err := strconv.Atoi(p); err == nil {
				port = n
			}
		}
		return host, port
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return host, port
}

// dnsAllowPortGroup is the set of private literal IPs that share a single
// port among the configured dns.servers, e.g. two internal resolvers both
// queried on :53.
type dnsAllowPortGroup struct {
	port int
	ips  []string
}

// collectPrivateDnsAllowGroups returns the private dns.servers endpoints
// grouped by port, sorted by port ascending (ips within a group sorted and
// de-duplicated) for deterministic output.
func collectPrivateDnsAllowGroups(dnsRaw json.RawMessage) []dnsAllowPortGroup {
	if len(dnsRaw) == 0 {
		return nil
	}
	var dns struct {
		Servers []any `json:"servers"`
	}
	if err := json.Unmarshal(dnsRaw, &dns); err != nil {
		return nil
	}

	byPort := make(map[int]map[string]bool)
	for _, entry := range dns.Servers {
		ep, ok := privateDnsServerEndpoint(entry)
		if !ok {
			continue
		}
		if byPort[ep.port] == nil {
			byPort[ep.port] = make(map[string]bool)
		}
		byPort[ep.port][ep.ip] = true
	}

	ports := make([]int, 0, len(byPort))
	for p := range byPort {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	groups := make([]dnsAllowPortGroup, 0, len(ports))
	for _, p := range ports {
		ips := make([]string, 0, len(byPort[p]))
		for ip := range byPort[p] {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		groups = append(groups, dnsAllowPortGroup{port: p, ips: ips})
	}
	return groups
}

// EnsureDnsServerRouting keeps a set of managed "direct" allow-rules — one
// per distinct port among any private/internal dns.servers addresses —
// in sync, positioned immediately before the panel's default
// geoip:private block rule.
//
// Why this matters: Xray's own DNS client traffic is dispatched through
// the same routing table as proxied client traffic. If dns.servers points
// at a private IP (e.g. a self-hosted AdGuard Home / Pi-hole reachable on
// the same Docker network as Xray — a common self-hosted setup) and the
// panel's default private-IP block rule is active, Xray's own DNS lookups
// get silently dropped by that rule. Xray then falls back to dialing
// destinations by raw hostname once its internal DNS attempt times out
// (~4s), so proxied connections still work, just with a multi-second stall
// added to every new domain, with no error surfaced to the client or
// admin.
//
// Each managed rule is scoped to its port (not just the IP), so the
// exception only reopens the DNS traffic that actually needs it rather
// than every port on the private host. On every save, all previously
// managed rules are stripped out and a fresh set is rebuilt from the
// current dns.servers config and reinserted right before the block rule
// (recomputing its index after the strip) — this corrects both content
// drift (dns.servers changed) and position drift (an admin dragged a
// managed rule below the block rule in the Routing tab, which would
// otherwise silently reintroduce the stall with nothing to notice or fix
// it). The rebuilt result is only written back if it actually differs
// from the input, so well-formed configs aren't churned on every save.
// Manually-authored rules are never touched — see dnsAllowRuleShape.
func EnsureDnsServerRouting(raw string) (string, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, err
	}

	groups := collectPrivateDnsAllowGroups(cfg["dns"])

	var routing map[string]json.RawMessage
	if r, ok := cfg["routing"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &routing); err != nil {
			return raw, err
		}
	}
	if routing == nil {
		return raw, nil
	}

	var original []map[string]any
	if r, ok := routing["rules"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &original); err != nil {
			return raw, err
		}
	}

	rebuilt := rebuildDnsAllowRules(original, groups)

	rulesJSON, err := json.Marshal(rebuilt)
	if err != nil {
		return raw, err
	}

	// Compare against the original rules JSON, not the parsed Go values:
	// json.Unmarshal into []map[string]any turns "ip" arrays into []any,
	// while the rules this function builds use []string — those hold
	// identical content but are different types under reflect.DeepEqual,
	// which would otherwise report a no-op input as changed and churn the
	// JSON on every save for no reason.
	origRulesJSON := routing["rules"]
	if len(origRulesJSON) == 0 {
		origRulesJSON = json.RawMessage("[]")
	}
	if jsonEqual(origRulesJSON, rulesJSON) {
		return raw, nil
	}

	routing["rules"] = rulesJSON
	routingJSON, err := json.Marshal(routing)
	if err != nil {
		return raw, err
	}
	cfg["routing"] = routingJSON

	out, err := json.Marshal(cfg)
	if err != nil {
		return raw, err
	}
	return string(out), nil
}

// rebuildDnsAllowRules strips any existing managed rules out of rules,
// then — if a geoip:private block rule is present and groups is non-empty
// — reinserts a freshly built managed rule per group immediately before
// it. This uniformly handles content updates, position drift, and removal
// (an empty groups list just leaves the managed rules stripped).
func rebuildDnsAllowRules(rules []map[string]any, groups []dnsAllowPortGroup) []map[string]any {
	clean := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		if !dnsAllowRuleShape(rule) {
			clean = append(clean, rule)
		}
	}

	blockIdx := findPrivateBlockRule(clean)
	if blockIdx < 0 || len(groups) == 0 {
		return clean
	}

	managed := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		managed = append(managed, map[string]any{
			"type":        "field",
			"ip":          g.ips,
			"port":        strconv.Itoa(g.port),
			"outboundTag": "direct",
		})
	}

	// Capacity hint uses len(clean) alone (not len(clean)+len(managed)):
	// summing two independent lengths for a make() size risks overflow on
	// pathological input per static analysis, and clean's length already
	// covers most of the eventual size on its own.
	out := make([]map[string]any, 0, len(clean))
	out = append(out, clean[:blockIdx]...)
	out = append(out, managed...)
	out = append(out, clean[blockIdx:]...)
	return out
}

// jsonEqual reports whether a and b decode to structurally identical
// values. Used instead of comparing raw bytes (key order, whitespace) or
// reflect.DeepEqual on already-parsed Go values (which is type-sensitive
// to []any vs []string and would misreport identical content as changed).
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
