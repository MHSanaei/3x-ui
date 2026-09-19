package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Config struct {
	Log map[string]any `json:"log,omitempty"`
	DNS map[string]any `json:"dns,omitempty"`
	Inbounds []map[string]any `json:"inbounds,omitempty"`
	Outbounds []map[string]any `json:"outbounds,omitempty"`
	Route map[string]any `json:"route,omitempty"`
	Experimental map[string]any `json:"experimental,omitempty"`
}

func NewConfig() *Config {
	return &Config{
		Log: map[string]any{"level": "info"},
		Inbounds: []map[string]any{},
		Outbounds: []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		Route: map[string]any{"final": "direct"},
	}
}

func (c *Config) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
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
