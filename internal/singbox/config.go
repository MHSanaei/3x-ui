package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	Schema string `json:"$schema,omitempty"`
	Log map[string]any `json:"log,omitempty"`
	DNS map[string]any `json:"dns,omitempty"`
	Inbounds []map[string]any `json:"inbounds,omitempty"`
	Outbounds []map[string]any `json:"outbounds,omitempty"`
	Route map[string]any `json:"route,omitempty"`
	Experimental map[string]any `json:"experimental,omitempty"`
	Services []map[string]any `json:"services,omitempty"`
}

func NewConfig() *Config {
	return &Config{
		Schema: "https://sing-box.sagernet.org/schema.json",
		Log: map[string]any{"level": "info"},
		Inbounds: []map[string]any{},
		Outbounds: []map[string]any{},
		Route: map[string]any{"final": "direct"},
		Experimental: map[string]any{
			"clash_api": map[string]any{
				"external_controller": "127.0.0.1:10090",
			},
		},
		Services: []map[string]any{
			{
				"type": "api",
				"tag": "panel-api",
				"listen": "127.0.0.1",
				"listen_port": 10091,
			},
		},
	}
}

func (c *Config) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func TranslateXrayOutbound(raw map[string]any) (map[string]any, error) {
	protocol, _ := raw["protocol"].(string)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	tag := rawString(raw, "tag")
	if tag == "" { return nil, fmt.Errorf("outbound tag is empty") }

	out := map[string]any{"tag": tag}
	switch protocol {
	case "freedom":
		out["type"] = "direct"
	case "blackhole":
		out["type"] = "block"
	case "socks", "http", "shadowsocks", "vmess", "vless", "trojan", "hysteria", "hysteria2", "tuic":
		out["type"] = protocol
	default:
		return nil, fmt.Errorf("sing-box does not support Xray outbound protocol %q through the compatibility translator", protocol)
	}

	settings := rawObject(raw, "settings")
	switch protocol {
	case "socks", "http":
		servers, _ := settings["servers"].([]any)
		if len(servers) == 0 { return nil, fmt.Errorf("outbound %q has no server", tag) }
		server, _ := servers[0].(map[string]any)
		if address := rawString(server, "address"); address != "" { out["server"] = address }
		out["server_port"] = rawInt(server, "port")
		if users, ok := server["users"].([]any); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]any); ok {
				if username := rawString(u, "user"); username != "" { out["username"] = username }
				if password := rawString(u, "pass"); password != "" { out["password"] = password }
			}
		}
	case "shadowsocks":
		servers, _ := settings["servers"].([]any)
		if len(servers) == 0 { return nil, fmt.Errorf("outbound %q has no server", tag) }
		server, _ := servers[0].(map[string]any)
		out["server"] = rawString(server, "address")
		out["server_port"] = rawInt(server, "port")
		out["method"] = rawString(server, "method")
		out["password"] = rawString(server, "password")
	case "vmess", "vless", "trojan":
		vnext, _ := settings["vnext"].([]any)
		if len(vnext) == 0 {
			servers, _ := settings["servers"].([]any)
			if len(servers) > 0 {
				vnext = servers
			}
		}
		if len(vnext) == 0 { return nil, fmt.Errorf("outbound %q has no server", tag) }
		server, _ := vnext[0].(map[string]any)
		out["server"] = rawString(server, "address")
		out["server_port"] = rawInt(server, "port")
		if users, ok := server["users"].([]any); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]any); ok {
				switch protocol {
				case "vless", "vmess":
					out["uuid"] = rawString(u, "id")
				case "trojan":
					out["password"] = rawString(u, "password")
				}
			}
		}
	case "hysteria", "hysteria2", "tuic":
		servers, _ := settings["servers"].([]any)
		if len(servers) == 0 {
			if server := rawObject(settings, "server"); len(server) > 0 {
				servers = []any{server}
			}
		}
		if len(servers) == 0 { return nil, fmt.Errorf("outbound %q has no server", tag) }
		server, _ := servers[0].(map[string]any)
		out["server"] = rawString(server, "address")
		out["server_port"] = rawInt(server, "port")
		if out["server"] == "" || rawInt(server, "port") == 0 {
			return nil, fmt.Errorf("outbound %q has invalid server", tag)
		}
		if auth := rawString(server, "auth"); auth != "" {
			if protocol == "hysteria" { out["auth_str"] = auth } else if protocol == "hysteria2" { out["password"] = auth }
		}
		if password := rawString(server, "password"); password != "" {
			out["password"] = password
		}
		if protocol == "tuic" {
			if uuid := rawString(server, "id"); uuid != "" { out["uuid"] = uuid }
			if uuid := rawString(server, "uuid"); uuid != "" { out["uuid"] = uuid }
		}
		for _, key := range []string{"up_mbps", "down_mbps", "hop_interval", "hop_interval_max", "bbr_profile", "congestion_control", "auth_timeout", "heartbeat"} {
			copyStringOrInt(settings, server, out, key)
		}
		if protocol == "hysteria2" {
			if obfs := rawObject(settings, "obfs"); len(obfs) > 0 { out["obfs"] = obfs }
			copyBool(settings, out, "disable_chrome_parrot")
		}
		if protocol == "tuic" {
			copyString(settings, out, "udp_relay_mode")
			copyBool(settings, out, "udp_over_stream")
			copyBool(settings, out, "zero_rtt_handshake")
		}
	}
	if stream := rawObject(raw, "streamSettings"); len(stream) > 0 {
		if err := translateStream(out, protocol, stream); err != nil { return nil, err }
	}
	return out, nil
}

func TranslateXrayRouting(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	rulesRaw, _ := raw["rules"].([]any)
	rules := make([]map[string]any, 0, len(rulesRaw))
	for _, item := range rulesRaw {
		xr, ok := item.(map[string]any)
		if !ok { continue }
		r := map[string]any{}
		if tags := stringSlice(xr["inboundTag"]); len(tags) > 0 { r["inbound"] = tags }
		if domains := stringSlice(xr["domain"]); len(domains) > 0 { r["domain"] = domains }
		if ips := stringSlice(xr["ip"]); len(ips) > 0 { r["ip_cidr"] = ips }
		if ports := rawString(xr, "port"); ports != "" { r["port"] = ports }
		if network := rawString(xr, "network"); network != "" {
			switch network {
			case "tcp": r["network"] = "tcp"
			case "udp": r["network"] = "udp"
			}
		}
		if outbound := rawString(xr, "outboundTag"); outbound != "" { r["outbound"] = outbound }
		if len(r) > 0 { rules = append(rules, r) }
	}
	if len(rules) > 0 { out["rules"] = rules }
	if ds := rawString(raw, "domainStrategy"); ds != "" {
		switch ds {
		case "AsIs": out["default_domain_strategy"] = "prefer_ipv4"
		case "IPIfNonMatch", "IPOnDemand": out["default_domain_strategy"] = "prefer_ipv4"
		}
	}
	return out, nil
}

func TranslateXrayDNS(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	serversRaw, _ := raw["servers"].([]any)
	servers := make([]map[string]any, 0, len(serversRaw))
	for i, item := range serversRaw {
		var addr string
		var resolver string
		switch v := item.(type) {
		case string:
			addr = strings.TrimSpace(v)
		case map[string]any:
			addr = strings.TrimSpace(rawString(v, "address"))
			resolver = strings.TrimSpace(rawString(v, "clientIp"))
		}
		if addr == "" { continue }
		server := map[string]any{"tag": fmt.Sprintf("dns-%d", i+1)}
		if strings.EqualFold(addr, "localhost") || strings.EqualFold(addr, "local") {
			server["type"] = "local"
		} else {
			switch {
			case strings.HasPrefix(addr, "https://"):
				server["type"] = "https"
				if u, err := url.Parse(addr); err == nil {
					server["server"] = u.Hostname()
					if p := u.Port(); p != "" { server["server_port"] = atoiOr(p, 443) } else { server["server_port"] = 443 }
					server["path"] = u.EscapedPath()
					if server["path"] == "" { server["path"] = "/dns-query" }
				}
			case strings.HasPrefix(addr, "tls://"):
				server["type"] = "tls"
				if u, err := url.Parse(addr); err == nil {
					server["server"] = u.Hostname()
					if p := u.Port(); p != "" { server["server_port"] = atoiOr(p, 853) } else { server["server_port"] = 853 }
				}
			case strings.HasPrefix(addr, "quic://"):
				server["type"] = "quic"
				if u, err := url.Parse(addr); err == nil {
					server["server"] = u.Hostname()
					if p := u.Port(); p != "" { server["server_port"] = atoiOr(p, 853) } else { server["server_port"] = 853 }
				}
			default:
				server["type"] = "udp"
				if u, err := url.Parse(addr); err == nil && u.Hostname() != "" {
					server["server"] = u.Hostname()
					if p := u.Port(); p != "" { server["server_port"] = atoiOr(p, 53) } else { server["server_port"] = 53 }
				} else {
					server["server"] = strings.TrimPrefix(addr, "udp://")
					server["server_port"] = 53
				}
			}
		}
		if resolver != "" && server["type"] != "local" { server["domain_resolver"] = resolver }
		servers = append(servers, server)
	}
	if len(servers) > 0 {
		out["servers"] = servers
		out["final"] = servers[0]["tag"]
	}
	if strategy := rawString(raw, "queryStrategy"); strategy != "" {
		switch strategy {
		case "UseIPv4": out["strategy"] = "ipv4_only"
		case "UseIPv6": out["strategy"] = "ipv6_only"
		case "UseIP", "UseIPV4AndIPv6": out["strategy"] = "prefer_ipv4"
		}
	}
	return out, nil
}

func atoiOr(value string, fallback int) int {
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n <= 0 { return fallback }
	return n
}

func stringSlice(v any) []string {
	switch values := v.(type) {
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" { out = append(out, s) }
		}
		return out
	case []string:
		return values
	case string:
		if value := strings.TrimSpace(values); value != "" { return []string{value} }
	}
	return nil
}

func TranslateXrayInbound(raw map[string]any) (map[string]any, error) {
	protocol, _ := raw["protocol"].(string)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		return nil, fmt.Errorf("inbound protocol is empty")
	}
	out := map[string]any{
		"tag": rawString(raw, "tag"),
		"listen": normalizeListen(rawString(raw, "listen")),
		"listen_port": rawInt(raw, "port"),
	}
	switch protocol {
	case "vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic", "hysteria", "socks", "http", "mixed", "direct":
		out["type"] = protocol
	default:
		return nil, fmt.Errorf("sing-box does not support Xray inbound protocol %q through the compatibility translator", protocol)
	}
	settings := rawObject(raw, "settings")
	if err := translateUsers(out, protocol, settings); err != nil {
		return nil, err
	}
	if err := translateProtocolSettings(out, protocol, settings); err != nil {
		return nil, err
	}
	if err := translateStream(out, protocol, rawObject(raw, "streamSettings")); err != nil {
		return nil, err
	}
	return out, nil
}

func translateProtocolSettings(out map[string]any, protocol string, settings map[string]any) error {
	if protocol == "hysteria" {
		copyString(settings, out, "obfs")
		copyString(settings, out, "up")
		copyString(settings, out, "down")
		copyInt(settings, out, "up_mbps")
		copyInt(settings, out, "down_mbps")
	}
	if protocol == "hysteria2" {
		copyInt(settings, out, "up_mbps")
		copyInt(settings, out, "down_mbps")
		copyBool(settings, out, "ignore_client_bandwidth")
		copyString(settings, out, "masquerade")
		copyString(settings, out, "bbr_profile")
		if obfs := rawObject(settings, "obfs"); len(obfs) > 0 {
			out["obfs"] = obfs
		}
	}
	if protocol == "tuic" {
		copyString(settings, out, "congestion_control")
		copyString(settings, out, "auth_timeout")
		copyBool(settings, out, "zero_rtt_handshake")
		copyString(settings, out, "heartbeat")
	}
	return nil
}

func copyString(src, dst map[string]any, key string) {
	if value := rawString(src, key); value != "" {
		dst[key] = value
	}
}

func copyInt(src, dst map[string]any, key string) {
	if value, ok := src[key].(float64); ok {
		dst[key] = int(value)
		return
	}
	if value, ok := src[key].(int); ok {
		dst[key] = value
	}
}

func copyStringOrInt(src, server, dst map[string]any, key string) {
	if value := rawString(src, key); value != "" {
		dst[key] = value
		return
	}
	if value := rawInt(src, key); value != 0 {
		dst[key] = value
		return
	}
	if value := rawString(server, key); value != "" {
		dst[key] = value
		return
	}
	if value := rawInt(server, key); value != 0 {
		dst[key] = value
	}
}

func copyBool(src, dst map[string]any, key string) {
	if value, ok := src[key].(bool); ok {
		dst[key] = value
	}
}

func translateUsers(out map[string]any, protocol string, settings map[string]any) error {
	clients, _ := settings["clients"].([]any)
	if len(clients) == 0 {
		return nil
	}
	users := make([]map[string]any, 0, len(clients))
	for _, item := range clients {
		client, ok := item.(map[string]any)
		if !ok { continue }
		user := map[string]any{}
		if email, ok := client["email"].(string); ok && email != "" { user["name"] = email }
		switch protocol {
		case "vless", "vmess":
			if id, ok := client["id"].(string); ok && id != "" { user["uuid"] = id }
			if flow, ok := client["flow"].(string); ok && flow != "" && protocol == "vless" { user["flow"] = flow }
		case "trojan", "shadowsocks":
			if password, ok := client["password"].(string); ok && password != "" {
				user["password"] = password
			}
		case "hysteria2":
			if password, ok := client["password"].(string); ok && password != "" {
				user["password"] = password
			} else if auth, ok := client["auth"].(string); ok && auth != "" {
				user["password"] = auth
			}
		case "hysteria":
			if auth, ok := client["auth"].(string); ok && auth != "" {
				user["auth_str"] = auth
			}
		case "tuic":
			if id, ok := client["id"].(string); ok && id != "" { user["uuid"] = id }
			if password, ok := client["password"].(string); ok && password != "" { user["password"] = password }
		}
		users = append(users, user)
	}
	if len(users) > 0 { out["users"] = users }
	return nil
}

func translateStream(out map[string]any, protocol string, stream map[string]any) error {
	if len(stream) == 0 { return nil }
	security, _ := stream["security"].(string)
	if security == "tls" || security == "reality" {
		tls := rawObject(stream, "tlsSettings")
		t := map[string]any{"enabled": true}
		if serverName, ok := tls["serverName"].(string); ok && serverName != "" { t["server_name"] = serverName }
		if alpn := stringSlice(tls["alpn"]); len(alpn) > 0 { t["alpn"] = alpn }
		if allow, ok := tls["allowInsecure"].(bool); ok { t["insecure"] = allow }
		if fingerprint := rawString(tls, "fingerprint"); fingerprint != "" { t["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint} }
		if certs, ok := tls["certificates"].([]any); ok && len(certs) > 0 {
			if certificate, key := translateXrayCertificate(certs[0]); certificate != "" || key != "" {
				if strings.HasPrefix(certificate, "-----BEGIN") {
					t["certificate"] = []string{certificate}
				} else if certificate != "" {
					t["certificate_path"] = certificate
				}
				if strings.HasPrefix(key, "-----BEGIN") {
					t["key"] = []string{key}
				} else if key != "" {
					t["key_path"] = key
				}
			}
		}
		if security == "reality" {
			reality := rawObject(tls, "realitySettings")
			if len(reality) == 0 {
				return fmt.Errorf("inbound %q uses Xray REALITY without realitySettings", rawString(out, "tag"))
			}
			r := map[string]any{"enabled": true}
			if privateKey := rawString(reality, "privateKey"); privateKey != "" {
				dest := rawString(reality, "dest")
				host, port := splitRealityDestination(dest)
				if host == "" || port == 0 {
					return fmt.Errorf("inbound %q has invalid Xray REALITY destination %q", rawString(out, "tag"), dest)
				}
				r["handshake"] = map[string]any{"server": host, "server_port": port}
				r["private_key"] = privateKey
				if ids := stringSlice(reality["shortIds"]); len(ids) > 0 { r["short_id"] = ids }
				if maxDiff := rawDurationMillis(reality["maxTimeDiff"]); maxDiff != "" { r["max_time_difference"] = maxDiff }
			} else if publicKey := rawString(reality, "publicKey"); publicKey != "" {
				r["public_key"] = publicKey
				if shortID := rawString(reality, "shortId"); shortID != "" { r["short_id"] = shortID }
				if ids := stringSlice(reality["shortIds"]); len(ids) > 0 && r["short_id"] == nil { r["short_id"] = ids[0] }
			} else {
				return fmt.Errorf("inbound %q has unsupported Xray REALITY settings", rawString(out, "tag"))
			}
			t["reality"] = r
		}
		out["tls"] = t
	}
	network, _ := stream["network"].(string)
	switch network {
	case "", "tcp":
		return nil
	case "ws":
		ws := rawObject(stream, "wsSettings")
		transport := map[string]any{"type": "ws"}
		if path, ok := ws["path"].(string); ok && path != "" { transport["path"] = path }
		if headers, ok := ws["headers"].(map[string]any); ok && len(headers) > 0 { transport["headers"] = headers }
		out["transport"] = transport
	case "grpc":
		grpc := rawObject(stream, "grpcSettings")
		transport := map[string]any{"type": "grpc"}
		if name, ok := grpc["serviceName"].(string); ok && name != "" { transport["service_name"] = name }
		out["transport"] = transport
	case "http", "h2":
		httpSettings := rawObject(stream, "httpSettings")
		transport := map[string]any{"type": "http"}
		if hosts := stringSlice(httpSettings["host"]); len(hosts) > 0 { transport["host"] = hosts }
		if path, ok := httpSettings["path"].(string); ok && path != "" { transport["path"] = path }
		if method, ok := httpSettings["method"].(string); ok && method != "" { transport["method"] = method }
		if headers, ok := httpSettings["headers"].(map[string]any); ok && len(headers) > 0 { transport["headers"] = headers }
		out["transport"] = transport
	case "httpupgrade":
		settings := rawObject(stream, "httpupgradeSettings")
		transport := map[string]any{"type": "httpupgrade"}
		if host, ok := settings["host"].(string); ok && host != "" { transport["host"] = host }
		if path, ok := settings["path"].(string); ok && path != "" { transport["path"] = path }
		if headers, ok := settings["headers"].(map[string]any); ok && len(headers) > 0 { transport["headers"] = headers }
		out["transport"] = transport
	case "quic":
		settings := rawObject(stream, "quicSettings")
		transport := map[string]any{"type": "quic"}
		if security, ok := settings["security"].(string); ok && security != "" {
			return fmt.Errorf("inbound %q uses Xray QUIC encryption %q; sing-box QUIC transport has no additional encryption", rawString(out, "tag"), security)
		}
		translateQUICSettings(transport, settings)
		out["transport"] = transport
	case "splithttp", "xhttp":
		return fmt.Errorf("inbound %q uses Xray XHTTP/SplitHTTP; configure a sing-box-native HTTP transport before switching cores", rawString(out, "tag"))
	default:
		return fmt.Errorf("inbound %q uses unsupported Xray transport %q", rawString(out, "tag"), network)
	}
	_ = protocol
	return nil
}

func translateQUICSettings(dst, settings map[string]any) {
	// sing-box 1.14 shares these QUIC fields across Hysteria/Hysteria2/TUIC.
	// Accept both native sing-box names and the camelCase spelling commonly
	// used by Xray-derived templates.
	if value := rawInt(settings, "initial_packet_size"); value > 0 {
		dst["initial_packet_size"] = value
	} else if value := rawInt(settings, "initialPacketSize"); value > 0 {
		dst["initial_packet_size"] = value
	}
	if value, ok := settings["disable_path_mtu_discovery"].(bool); ok {
		dst["disable_path_mtu_discovery"] = value
	} else if value, ok := settings["disablePathMTUDiscovery"].(bool); ok {
		dst["disable_path_mtu_discovery"] = value
	}
}

func translateXrayCertificate(value any) (string, string) {
	cert, ok := value.(map[string]any)
	if !ok {
		return "", ""
	}
	certificate := rawString(cert, "certificateFile")
	key := rawString(cert, "keyFile")
	if certificate == "" {
		certificate = rawString(cert, "certificate")
	}
	if key == "" {
		key = rawString(cert, "key")
	}
	return certificate, key
}

func splitRealityDestination(value string) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		n, _ := strconv.Atoi(port)
		return host, n
	}
	return value, 443
}

func rawDurationMillis(v any) string {
	var millis int64
	switch value := v.(type) {
	case int:
		millis = int64(value)
	case int64:
		millis = value
	case float64:
		millis = int64(value)
	case string:
		millis, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	}
	if millis <= 0 {
		return ""
	}
	seconds := millis / 1000
	remaining := millis % 1000
	if remaining == 0 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dms", millis)
}

func normalizeListen(value string) string {
	if value == "" { return "::" }
	if net.ParseIP(value) != nil { return value }
	return value
}

func rawString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func rawInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int: return v
	case int64: return int(v)
	case float64: return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default: return 0
	}
}

func rawObject(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}
