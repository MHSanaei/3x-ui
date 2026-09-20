package singbox

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// TranslateXrayRouting provides a compile-safe compatibility layer for the
// panel's Xray routing configuration. Unsupported legacy matchers are rejected
// instead of being emitted as invalid sing-box fields.
func TranslateXrayRouting(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	rulesRaw, _ := raw["rules"].([]any)
	rules := make([]map[string]any, 0, len(rulesRaw))
	for i, item := range rulesRaw {
		xr, ok := item.(map[string]any)
		if !ok {
			continue
		}
		r := map[string]any{}
		if tags := compatStringSlice(xr["inboundTag"]); len(tags) > 0 {
			r["inbound"] = tags
		}
		if domains := compatStringSlice(xr["domain"]); len(domains) > 0 {
			if err := translateCompatDomains(r, domains); err != nil {
				return nil, fmt.Errorf("routing rule %d: %w", i, err)
			}
		}
		if ips := compatStringSlice(xr["ip"]); len(ips) > 0 {
			cidrs := make([]string, 0, len(ips))
			for _, ip := range ips {
				if net.ParseIP(ip) == nil {
					if _, _, err := net.ParseCIDR(ip); err != nil {
						return nil, fmt.Errorf("routing rule %d: unsupported IP matcher %q", i, ip)
					}
				}
				cidrs = append(cidrs, ip)
			}
			r["ip_cidr"] = cidrs
		}
		if port := compatString(xr["port"]); port != "" {
			r["port"] = port
		}
		if sourcePort := compatString(xr["sourcePort"]); sourcePort != "" {
			r["source_port"] = sourcePort
		}
		if network := compatString(xr["network"]); network != "" {
			if network != "tcp" && network != "udp" {
				return nil, fmt.Errorf("routing rule %d: unsupported network %q", i, network)
			}
			r["network"] = network
		}
		if users := compatStringSlice(xr["user"]); len(users) > 0 {
			r["user"] = users
		}
		if protocols := compatStringSlice(xr["protocol"]); len(protocols) > 0 {
			r["protocol"] = protocols
		}
		if outbound := compatString(xr["outboundTag"]); outbound != "" {
			r["action"] = "route"
			r["outbound"] = outbound
		} else if balancer := compatString(xr["balancerTag"]); balancer != "" {
			return nil, fmt.Errorf("routing rule %d uses unsupported balancerTag %q", i, balancer)
		} else {
			return nil, fmt.Errorf("routing rule %d has no outboundTag", i)
		}
		rules = append(rules, r)
	}
	if len(rules) > 0 {
		out["rules"] = rules
	}
	if strategy := compatString(raw["domainStrategy"]); strategy != "" && strategy != "AsIs" {
		return nil, fmt.Errorf("Xray routing domainStrategy %q cannot be represented by the compatibility layer", strategy)
	}
	return out, nil
}

func translateCompatDomains(dst map[string]any, domains []string) error {
	var exact, suffix, keyword, regex []string
	for _, value := range domains {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "!") || strings.HasPrefix(value, "geosite:") || strings.HasPrefix(value, "ext:") || strings.HasPrefix(value, "dotless:") {
			return fmt.Errorf("unsupported domain matcher %q", value)
		}
		switch {
		case strings.HasPrefix(value, "regexp:"):
			regex = append(regex, strings.TrimPrefix(value, "regexp:"))
		case strings.HasPrefix(value, "domain:"):
			suffix = append(suffix, strings.TrimPrefix(value, "domain:"))
		case strings.HasPrefix(value, "keyword:"):
			keyword = append(keyword, strings.TrimPrefix(value, "keyword:"))
		case strings.HasPrefix(value, "full:"):
			exact = append(exact, strings.TrimPrefix(value, "full:"))
		default:
			exact = append(exact, value)
		}
	}
	if len(exact) > 0 { dst["domain"] = exact }
	if len(suffix) > 0 { dst["domain_suffix"] = suffix }
	if len(keyword) > 0 { dst["domain_keyword"] = keyword }
	if len(regex) > 0 { dst["domain_regex"] = regex }
	return nil
}

// TranslateXrayDNS converts the common Xray DNS server forms used by the
// panel to the generic sing-box DNS object without depending on sing-box
// internal Go packages.
func TranslateXrayDNS(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	serversRaw, _ := raw["servers"].([]any)
	servers := make([]map[string]any, 0, len(serversRaw))
	for i, item := range serversRaw {
		var address string
		switch value := item.(type) {
		case string:
			address = strings.TrimSpace(value)
		case map[string]any:
			address = strings.TrimSpace(compatString(value["address"]))
		default:
			continue
		}
		if address == "" {
			continue
		}
		server := map[string]any{"tag": fmt.Sprintf("dns-%d", i+1)}
		if strings.EqualFold(address, "localhost") || strings.EqualFold(address, "local") {
			server["type"] = "local"
		} else {
			if strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "h3://") {
				server["type"] = "https"
				defaultPort := 443
				if strings.HasPrefix(address, "h3://") {
					server["type"] = "h3"
				}
				if u, err := url.Parse(address); err == nil && u.Hostname() != "" {
					server["server"] = u.Hostname()
					server["server_port"] = parseCompatPort(u.Port(), defaultPort)
					if u.EscapedPath() != "" { server["path"] = u.EscapedPath() }
				} else {
					return nil, fmt.Errorf("DNS server %d has invalid address %q", i, address)
				}
			} else if strings.HasPrefix(address, "tls://") {
				server["type"] = "tls"
				if u, err := url.Parse(address); err == nil && u.Hostname() != "" {
					server["server"] = u.Hostname()
					server["server_port"] = parseCompatPort(u.Port(), 853)
				} else { return nil, fmt.Errorf("DNS server %d has invalid TLS address %q", i, address) }
			} else if strings.HasPrefix(address, "quic://") {
				server["type"] = "quic"
				if u, err := url.Parse(address); err == nil && u.Hostname() != "" {
					server["server"] = u.Hostname()
					server["server_port"] = parseCompatPort(u.Port(), 853)
				} else { return nil, fmt.Errorf("DNS server %d has invalid QUIC address %q", i, address) }
			} else {
				server["type"] = "udp"
				clean := strings.TrimPrefix(address, "udp://")
				if u, err := url.Parse("udp://"+clean); err == nil && u.Hostname() != "" {
					server["server"] = u.Hostname()
					server["server_port"] = parseCompatPort(u.Port(), 53)
				} else { return nil, fmt.Errorf("DNS server %d has invalid UDP address %q", i, address) }
			}
		}
		servers = append(servers, server)
	}
	if len(servers) > 0 {
		out["servers"] = servers
		out["final"] = servers[0]["tag"]
	}
	if clientIP := compatString(raw["clientIp"]); clientIP != "" {
		if net.ParseIP(clientIP) == nil { return nil, fmt.Errorf("DNS has invalid clientIp %q", clientIP) }
		out["client_subnet"] = clientIP
	}
	return out, nil
}

func compatString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func compatStringSlice(v any) []string {
	switch values := v.(type) {
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s := compatString(value); s != "" { out = append(out, s) }
		}
		return out
	case []string:
		return values
	case string:
		if s := strings.TrimSpace(values); s != "" { return []string{s} }
	}
	return nil
}

func parseCompatPort(value string, fallback int) int {
	if value == "" { return fallback }
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 { return fallback }
	return port
}
