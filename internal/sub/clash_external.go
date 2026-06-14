package sub

import (
	"fmt"
	"strconv"
	"strings"
)

// clashProxyFromExternal parses a pasted share link and converts it into a
// mihomo/Clash proxy entry named `name`. Returns nil for links Clash can't
// represent (the entry is then skipped, mirroring how getProxies drops
// unsupported inbound protocols). vmess/vless/trojan reuse the existing
// applyTransport/applySecurity helpers; ss/hysteria2/wireguard map directly.
func (s *SubClashService) clashProxyFromExternal(rawLink, name string) map[string]any {
	ob := parseExternalLink(rawLink)
	if ob == nil {
		return nil
	}
	protocol, _ := ob["protocol"].(string)
	settings, _ := ob["settings"].(map[string]any)
	stream, _ := ob["streamSettings"].(map[string]any)
	if stream == nil {
		stream = map[string]any{}
	}
	if settings == nil {
		return nil
	}

	proxy := map[string]any{"name": name, "udp": true}

	switch protocol {
	case "vmess":
		vnext, _ := settings["vnext"].([]any)
		if len(vnext) == 0 {
			return nil
		}
		vn, _ := vnext[0].(map[string]any)
		users, _ := vn["users"].([]any)
		if vn == nil || len(users) == 0 {
			return nil
		}
		user, _ := users[0].(map[string]any)
		proxy["type"] = "vmess"
		proxy["server"] = fmt.Sprint(vn["address"])
		proxy["port"] = clashInt(vn["port"])
		proxy["uuid"] = fmt.Sprint(user["id"])
		proxy["alterId"] = 0
		cipher, _ := user["security"].(string)
		if cipher == "" {
			cipher = "auto"
		}
		proxy["cipher"] = cipher
	case "vless":
		proxy["type"] = "vless"
		proxy["server"] = fmt.Sprint(settings["address"])
		proxy["port"] = clashInt(settings["port"])
		proxy["uuid"] = fmt.Sprint(settings["id"])
		if flow, _ := settings["flow"].(string); flow != "" {
			proxy["flow"] = flow
		}
	case "trojan":
		server := firstServer(settings)
		if server == nil {
			return nil
		}
		proxy["type"] = "trojan"
		proxy["server"] = fmt.Sprint(server["address"])
		proxy["port"] = clashInt(server["port"])
		proxy["password"] = fmt.Sprint(server["password"])
	case "shadowsocks":
		server := firstServer(settings)
		if server == nil {
			server = settings
		}
		method, _ := server["method"].(string)
		if method == "" {
			return nil
		}
		proxy["type"] = "ss"
		proxy["server"] = fmt.Sprint(server["address"])
		proxy["port"] = clashInt(server["port"])
		proxy["cipher"] = method
		proxy["password"] = fmt.Sprint(server["password"])
		return proxy
	case "hysteria":
		return clashHysteriaFromExternal(settings, stream, name)
	case "wireguard":
		return clashWireguardFromExternal(settings, name)
	default:
		return nil
	}

	network, _ := stream["network"].(string)
	if !s.applyTransport(proxy, network, stream) {
		return nil
	}
	security, _ := stream["security"].(string)
	if !s.applySecurity(proxy, security, stream) {
		return nil
	}
	return proxy
}

func firstServer(settings map[string]any) map[string]any {
	servers, _ := settings["servers"].([]any)
	if len(servers) == 0 {
		return nil
	}
	server, _ := servers[0].(map[string]any)
	return server
}

func clashHysteriaFromExternal(settings, stream map[string]any, name string) map[string]any {
	hy, _ := stream["hysteriaSettings"].(map[string]any)
	auth := ""
	if hy != nil {
		auth, _ = hy["auth"].(string)
	}
	if auth == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     name,
		"type":     "hysteria2",
		"server":   fmt.Sprint(settings["address"]),
		"port":     clashInt(settings["port"]),
		"password": auth,
		"udp":      true,
	}
	if tls, _ := stream["tlsSettings"].(map[string]any); tls != nil {
		if sni, _ := tls["serverName"].(string); sni != "" {
			proxy["sni"] = sni
		}
		if alpn := clashStringList(tls["alpn"]); len(alpn) > 0 {
			proxy["alpn"] = alpn
		}
		if fp, _ := tls["fingerprint"].(string); fp != "" {
			proxy["client-fingerprint"] = fp
		}
	}
	return proxy
}

func clashWireguardFromExternal(settings map[string]any, name string) map[string]any {
	peers, _ := settings["peers"].([]any)
	if len(peers) == 0 {
		return nil
	}
	peer, _ := peers[0].(map[string]any)
	if peer == nil {
		return nil
	}
	host, port := splitClashHostPort(fmt.Sprint(peer["endpoint"]))
	if host == "" || port == 0 {
		return nil
	}
	proxy := map[string]any{
		"name":   name,
		"type":   "wireguard",
		"server": host,
		"port":   port,
		"udp":    true,
	}
	if sk, _ := settings["secretKey"].(string); sk != "" {
		proxy["private-key"] = sk
	}
	if pk, _ := peer["publicKey"].(string); pk != "" {
		proxy["public-key"] = pk
	}
	if psk, _ := peer["preSharedKey"].(string); psk != "" {
		proxy["pre-shared-key"] = psk
	}
	for _, addr := range clashStringList(settings["address"]) {
		ip := stripCIDR(addr)
		if strings.Contains(ip, ":") {
			proxy["ipv6"] = ip
		} else {
			proxy["ip"] = ip
		}
	}
	return proxy
}

func clashInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func clashStringList(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	case string:
		if x == "" {
			return nil
		}
		return strings.Split(x, ",")
	default:
		return nil
	}
}

func stripCIDR(addr string) string {
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		return addr[:i]
	}
	return addr
}

func splitClashHostPort(endpoint string) (string, int) {
	endpoint = strings.TrimSpace(endpoint)
	i := strings.LastIndex(endpoint, ":")
	if i < 0 {
		return endpoint, 0
	}
	host := strings.Trim(endpoint[:i], "[]")
	port, _ := strconv.Atoi(endpoint[i+1:])
	return host, port
}
