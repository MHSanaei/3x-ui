package service

import (
	"encoding/json"
	"net"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

// dnsAllowRuleShape identifies routing rules this file manages: a plain
// "type=field, ip=[...], outboundTag=direct" rule with no other matchers.
// Rules shaped like this are treated as owned by EnsureDnsServerRouting and
// are kept in sync with the current dns.servers config on every save;
// anything else (including rules an admin wrote by hand that happen to also
// allow-list an IP) is left untouched.
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
	for key := range rule {
		switch key {
		case "type", "outboundTag", "ip":
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
		ips := readRuleIPs(rule["ip"])
		for _, ip := range ips {
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

// dnsServerAddress extracts the raw "address" of a dns.servers entry, which
// is either a bare string or an object with an "address" field (see
// frontend/src/schemas/dns.ts DnsServerEntrySchema).
func dnsServerAddress(entry any) string {
	switch v := entry.(type) {
	case string:
		return v
	case map[string]any:
		addr, _ := v["address"].(string)
		return addr
	default:
		return ""
	}
}

// privateLiteralIP extracts a literal, private/internal IP address from a
// dns.servers address string, or "" if the address is a domain name, a
// special Xray keyword (localhost, fakedns, ...), or resolves to a public
// IP. Xray-core DNS server addresses may carry a URI scheme
// (tcp://, tcp+local://, https://, https+local://, quic://, quic+local://)
// and, for DoH, a path and/or a bracketed IPv6 host — all of that is
// stripped down to the bare host before checking.
func privateLiteralIP(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	if idx := strings.Index(address, "://"); idx != -1 {
		address = address[idx+3:]
	}
	// Drop a DoH path, e.g. "1.1.1.1/dns-query".
	if idx := strings.Index(address, "/"); idx != -1 {
		address = address[:idx]
	}

	host := address
	if strings.HasPrefix(host, "[") {
		// Bracketed IPv6, with or without a port: "[::1]" / "[::1]:53".
		if end := strings.Index(host, "]"); end != -1 {
			host = host[1:end]
		}
	} else if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Domain name, or a special keyword like "localhost"/"fakedns" —
		// neither is something we can safely allow-list by IP here.
		return ""
	}
	if !netsafe.IsBlockedIP(ip) {
		return ""
	}
	return ip.String()
}

// collectPrivateDnsServerIPs returns the sorted, de-duplicated set of
// literal private/internal IPs configured under dns.servers.
func collectPrivateDnsServerIPs(dnsRaw json.RawMessage) []string {
	if len(dnsRaw) == 0 {
		return nil
	}
	var dns struct {
		Servers []any `json:"servers"`
	}
	if err := json.Unmarshal(dnsRaw, &dns); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var ips []string
	for _, entry := range dns.Servers {
		ip := privateLiteralIP(dnsServerAddress(entry))
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips
}

// EnsureDnsServerRouting keeps a "direct" allow-rule for any private/internal
// dns.servers address in sync, inserted immediately before the panel's
// default geoip:private block rule.
//
// Why this matters: Xray's own DNS client traffic is dispatched through the
// same routing table as proxied client traffic. If dns.servers points at a
// private IP (e.g. a self-hosted AdGuard Home / Pi-hole reachable on the
// same Docker network as Xray — a common self-hosted setup) and the panel's
// default private-IP block rule is active, Xray's own DNS lookups get
// silently dropped by that rule. Xray then falls back to dialing
// destinations by raw hostname once its internal DNS attempt times out
// (~4s), so proxied connections still work, just with a multi-second stall
// added to every new domain, with no error surfaced to the client or admin.
//
// This only touches the routing table when there is at least one private
// dns.servers IP AND an existing geoip:private block rule; it is a no-op
// otherwise. Rules it previously inserted (see dnsAllowRuleShape) are kept
// up to date on every save — removed if no longer needed, updated if the
// set of private DNS IPs changed. Manually-authored rules are never
// touched.
func EnsureDnsServerRouting(raw string) (string, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, err
	}

	privateIPs := collectPrivateDnsServerIPs(cfg["dns"])

	var routing map[string]json.RawMessage
	if r, ok := cfg["routing"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &routing); err != nil {
			return raw, err
		}
	}
	if routing == nil {
		return raw, nil
	}

	var rules []map[string]any
	if r, ok := routing["rules"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &rules); err != nil {
			return raw, err
		}
	}

	ownedIdx := -1
	for i, rule := range rules {
		if dnsAllowRuleShape(rule) {
			ownedIdx = i
			break
		}
	}

	blockIdx := findPrivateBlockRule(rules)

	changed := false
	switch {
	case len(privateIPs) == 0:
		if ownedIdx >= 0 {
			rules = append(rules[:ownedIdx], rules[ownedIdx+1:]...)
			changed = true
		}
	case blockIdx < 0:
		// No private-IP block rule active, so there is nothing for our
		// allow-rule to pre-empt. Drop a stale one if it exists.
		if ownedIdx >= 0 {
			rules = append(rules[:ownedIdx], rules[ownedIdx+1:]...)
			changed = true
		}
	case ownedIdx >= 0:
		if !equalStringSlices(readRuleIPs(rules[ownedIdx]["ip"]), privateIPs) {
			rules[ownedIdx]["ip"] = privateIPs
			changed = true
		}
	default:
		allowRule := map[string]any{
			"type":        "field",
			"ip":          privateIPs,
			"outboundTag": "direct",
		}
		rules = append(rules[:blockIdx:blockIdx], append([]map[string]any{allowRule}, rules[blockIdx:]...)...)
		changed = true
	}

	if !changed {
		return raw, nil
	}

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return raw, err
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

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
