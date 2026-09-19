package singbox

import (
	"encoding/json"
	"fmt"
	"net"
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
}

func NewConfig() *Config {
	return &Config{
		Schema: "https://sing-box.sagernet.org/schema.json",
		Log: map[string]any{"level": "info"},
		Inbounds: []map[string]any{},
		Outbounds: []map[string]any{},
		Route: map[string]any{"final": "direct"},
		Experimental: map[string]any{
			"v2ray_api": map[string]any{
				"listen": "127.0.0.1:10086",
				"stats": map[string]any{
					"enabled": true,
					"inbounds": []string{},
					"outbounds": []string{},
					"users": []string{},
				},
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
	case "socks", "http", "shadowsocks", "vmess", "vless", "trojan":
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
		case "AsIs": out["domain_strategy"] = "prefer_ipv4"
		case "IPIfNonMatch", "IPOnDemand": out["domain_strategy"] = "prefer_ipv4"
		}
	}
	return out, nil
}

func TranslateXrayDNS(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	serversRaw, _ := raw["servers"].([]any)
	servers := make([]map[string]any, 0, len(serversRaw))
	for _, item := range serversRaw {
		switch v := item.(type) {
		case string:
			if v != "" { servers = append(servers, map[string]any{"address": v}) }
		case map[string]any:
			addr := rawString(v, "address")
			if addr == "" { continue }
			s := map[string]any{"address": addr}
			if clientIP := rawString(v, "clientIp"); clientIP != "" { s["address_resolver"] = clientIP }
			servers = append(servers, s)
		}
	}
	if len(servers) > 0 { out["servers"] = servers }
	return out, nil
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
	if err := translateStream(out, protocol, rawObject(raw, "streamSettings")); err != nil {
		return nil, err
	}
	return out, nil
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
		case "trojan", "shadowsocks", "hysteria2":
			if password, ok := client["password"].(string); ok && password != "" {
				user["password"] = password
			} else if auth, ok := client["auth"].(string); ok && auth != "" && protocol == "hysteria2" {
				user["password"] = auth
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
	if security == "tls" {
		tls := rawObject(stream, "tlsSettings")
		t := map[string]any{"enabled": true}
		if serverName, ok := tls["serverName"].(string); ok && serverName != "" { t["server_name"] = serverName }
		if certs, ok := tls["certificates"].([]any); ok && len(certs) > 0 { t["certificate"] = certs }
		out["tls"] = t
	} else if security == "reality" {
		return fmt.Errorf("inbound %q uses Xray REALITY; configure a sing-box-native TLS/Reality profile before switching cores", rawString(out, "tag"))
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
	default:
		return fmt.Errorf("inbound %q uses unsupported Xray transport %q", rawString(out, "tag"), network)
	}
	_ = protocol
	return nil
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
