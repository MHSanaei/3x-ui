package singbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateXrayVLESSWebSocketTLS(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "vless-443",
		"listen": "0.0.0.0",
		"port": 443,
		"settings": map[string]any{
			"clients": []any{
				map[string]any{
					"id": "11111111-1111-1111-1111-111111111111",
					"email": "alice",
				},
			},
		},
		"streamSettings": map[string]any{
			"network": "ws",
			"security": "tls",
			"tlsSettings": map[string]any{
				"serverName": "example.com",
			},
			"wsSettings": map[string]any{
				"path": "/ws",
			},
		},
	}

	got, err := TranslateXrayInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["type"] != "vless" || got["listen_port"] != 443 {
		t.Fatalf("unexpected base config: %#v", got)
	}
	users, ok := got["users"].([]map[string]any)
	if !ok || len(users) != 1 || users[0]["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected users: %#v", got["users"])
	}
	tls, ok := got["tls"].(map[string]any)
	if !ok || tls["enabled"] != true || tls["server_name"] != "example.com" {
		t.Fatalf("unexpected tls: %#v", got["tls"])
	}
	transport, ok := got["transport"].(map[string]any)
	if !ok || transport["type"] != "ws" || transport["path"] != "/ws" {
		t.Fatalf("unexpected transport: %#v", got["transport"])
	}
}

func TestTranslateXrayInboundHTTPUpgrade(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "httpupgrade",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "httpupgrade",
			"httpupgradeSettings": map[string]any{
				"host": "example.com",
				"path": "/upgrade",
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	transport, ok := got["transport"].(map[string]any)
	if !ok || transport["type"] != "httpupgrade" || transport["host"] != "example.com" || transport["path"] != "/upgrade" {
		t.Fatalf("unexpected transport: %#v", got["transport"])
	}
}

func TestTranslateXrayInboundHTTPAndQUIC(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "http",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "http",
			"httpSettings": map[string]any{
				"host": []any{"example.com"},
				"path": "/",
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	transport := got["transport"].(map[string]any)
	if transport["type"] != "http" { t.Fatalf("unexpected HTTP transport: %#v", transport) }

	raw["tag"] = "quic"
	raw["streamSettings"] = map[string]any{
		"network": "quic",
		"quicSettings": map[string]any{},
	}
	got, err = TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	transport = got["transport"].(map[string]any)
	if transport["type"] != "quic" { t.Fatalf("unexpected QUIC transport: %#v", transport) }
}
func TestTranslateXrayInboundShadowsocksMethod(t *testing.T) {
	raw := map[string]any{
		"protocol": "shadowsocks", "tag": "ss-in",
		"settings": map[string]any{
			"method": "aes-256-gcm",
			"clients": []any{map[string]any{"email": "alice", "password": "secret"}},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	if got["method"] != "aes-256-gcm" { t.Fatalf("method = %#v", got["method"]) }
	if _, err := TranslateXrayInbound(map[string]any{"protocol":"shadowsocks","tag":"bad","settings":map[string]any{"clients":[]any{map[string]any{"password":"secret"}}}}); err == nil || !strings.Contains(err.Error(), "Shadowsocks method") {
		t.Fatalf("expected missing method error, got %v", err)
	}
}

func TestTranslateXrayRoutingMatchers(t *testing.T) {
	got, err := TranslateXrayRouting(map[string]any{
		"domainStrategy": "AsIs",
		"rules": []any{map[string]any{
			"domain": []any{"example.com", "domain:example.org", "keyword:ads", "regexp:^cdn\\.", "full:exact.example"},
			"ip": []any{"1.2.3.4", "10.0.0.0/8"},
			"network": "tcp", "outboundTag": "direct",
		}},
	})
	if err != nil { t.Fatal(err) }
	rule := got["rules"].([]map[string]any)[0]
	if rule["action"] != "route" || rule["outbound"] != "direct" { t.Fatalf("unexpected action: %#v", rule) }
	if len(rule["domain"].([]string)) != 2 || len(rule["domain_suffix"].([]string)) != 1 || len(rule["domain_keyword"].([]string)) != 1 || len(rule["domain_regex"].([]string)) != 1 {
		t.Fatalf("unexpected domain translation: %#v", rule)
	}
	if len(rule["ip_cidr"].([]string)) != 2 { t.Fatalf("unexpected IP translation: %#v", rule["ip_cidr"]) }
}

func TestTranslateXrayRoutingRejectsUnrepresentableMatchers(t *testing.T) {
	for _, rule := range []map[string]any{
		{"domain": []any{"geosite:cn"}, "outboundTag": "direct"},
		{"ip": []any{"geoip:private"}, "outboundTag": "direct"},
		{"domain": []any{"example.com"}, "sourceIP": []any{"10.0.0.0/8"}, "outboundTag": "direct"},
	} {
		if _, err := TranslateXrayRouting(map[string]any{"rules": []any{rule}}); err == nil {
			t.Fatalf("expected unsupported matcher error for %#v", rule)
		}
	}
}

func TestTranslateXrayRoutingRejectsNonAsIsStrategy(t *testing.T) {
	for _, strategy := range []string{"IPIfNonMatch", "IPOnDemand"} {
		if _, err := TranslateXrayRouting(map[string]any{"domainStrategy": strategy}); err == nil {
			t.Fatalf("expected unsupported domainStrategy error for %s", strategy)
		}
	}
}

func TestTranslateXrayDNSClientIP(t *testing.T) {
	got, err := TranslateXrayDNS(map[string]any{"clientIp":"1.2.3.4","servers":[]any{"1.1.1.1"}})
	if err != nil { t.Fatal(err) }
	if got["client_subnet"] != "1.2.3.4" { t.Fatalf("client_subnet = %#v", got["client_subnet"]) }
}

func TestTranslateHysteriaProtocolSettings(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria", "tag": "hy", "port": 443,
		"settings": map[string]any{
			"up_mbps": float64(100), "down_mbps": float64(200),
			"obfs": "secret",
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	if got["up_mbps"] != 100 || got["down_mbps"] != 200 || got["obfs"] != "secret" {
		t.Fatalf("unexpected Hysteria settings: %#v", got)
	}
}

func TestTranslateHysteria2ProtocolSettings(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria2", "tag": "hy2", "port": 443,
		"settings": map[string]any{
			"up_mbps": float64(100), "down_mbps": float64(200),
			"ignore_client_bandwidth": true,
			"masquerade": "https://example.com",
			"bbr_profile": "aggressive",
			"obfs": map[string]any{"type": "salamander", "password": "secret"},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	if got["up_mbps"] != 100 || got["down_mbps"] != 200 || got["ignore_client_bandwidth"] != true ||
		got["masquerade"] != "https://example.com" || got["bbr_profile"] != "aggressive" {
		t.Fatalf("unexpected Hysteria2 settings: %#v", got)
	}
	obfs, ok := got["obfs"].(map[string]any)
	if !ok || obfs["type"] != "salamander" || obfs["password"] != "secret" {
		t.Fatalf("unexpected Hysteria2 obfs: %#v", got["obfs"])
	}
}

func TestTranslateTUICProtocolSettings(t *testing.T) {
	raw := map[string]any{
		"protocol": "tuic", "tag": "tuic", "port": 443,
		"settings": map[string]any{
			"congestion_control": "bbr", "auth_timeout": "5s",
			"zero_rtt_handshake": true, "heartbeat": "15s",
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	if got["congestion_control"] != "bbr" || got["auth_timeout"] != "5s" ||
		got["zero_rtt_handshake"] != true || got["heartbeat"] != "15s" {
		t.Fatalf("unexpected TUIC settings: %#v", got)
	}
}

func TestTranslateQUICSettings(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria2", "tag": "hy2", "port": 443,
		"settings": map[string]any{},
		"streamSettings": map[string]any{
			"network": "quic",
			"quicSettings": map[string]any{
				"initialPacketSize": float64(1200),
				"disablePathMTUDiscovery": true,
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	transport, ok := got["transport"].(map[string]any)
	if !ok || transport["type"] != "quic" || transport["initial_packet_size"] != 1200 || transport["disable_path_mtu_discovery"] != true {
		t.Fatalf("unexpected QUIC settings: %#v", got["transport"])
	}
}

func TestTranslateHysteriaTLSCertificateFiles(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria2", "tag": "hy2", "port": 443,
		"settings": map[string]any{},
		"streamSettings": map[string]any{
			"security": "tls",
			"tlsSettings": map[string]any{
				"serverName": "example.com",
				"alpn": []any{"h3"},
				"certificates": []any{
					map[string]any{"certificateFile": "/etc/cert.pem", "keyFile": "/etc/key.pem"},
				},
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	tls, ok := got["tls"].(map[string]any)
	if !ok || tls["server_name"] != "example.com" || tls["certificate_path"] != "/etc/cert.pem" || tls["key_path"] != "/etc/key.pem" {
		t.Fatalf("unexpected TLS: %#v", got["tls"])
	}
}

func TestTranslateHysteriaTLSCertificatePEM(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria", "tag": "hy", "port": 443,
		"settings": map[string]any{},
		"streamSettings": map[string]any{
			"security": "tls",
			"tlsSettings": map[string]any{
				"certificates": []any{
					map[string]any{"certificate": "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----", "key": "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----"},
				},
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	tls := got["tls"].(map[string]any)
	if _, ok := tls["certificate"].([]string); !ok {
		t.Fatalf("expected PEM certificate: %#v", tls["certificate"])
	}
	if _, ok := tls["key"].([]string); !ok {
		t.Fatalf("expected PEM key: %#v", tls["key"])
	}
}

func TestTranslateHysteriaOutbound(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria", "tag": "hy-out",
		"settings": map[string]any{
			"servers": []any{map[string]any{"address": "example.com", "port": 443, "auth": "secret"}},
		},
	}
	got, err := TranslateXrayOutbound(raw)
	if err != nil { t.Fatal(err) }
	if got["type"] != "hysteria" || got["server"] != "example.com" || got["server_port"] != 443 || got["auth_str"] != "secret" {
		t.Fatalf("unexpected Hysteria outbound: %#v", got)
	}
}

func TestTranslateHysteria2Outbound(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria2", "tag": "hy2-out",
		"settings": map[string]any{
			"servers": []any{map[string]any{"address": "example.com", "port": 443, "password": "secret"}},
			"obfs": map[string]any{"type": "salamander", "password": "obfs"},
			"up_mbps": float64(100), "down_mbps": float64(200),
			"bbr_profile": "aggressive",
		},
	}
	got, err := TranslateXrayOutbound(raw)
	if err != nil { t.Fatal(err) }
	if got["type"] != "hysteria2" || got["password"] != "secret" || got["up_mbps"] != 100 || got["down_mbps"] != 200 || got["bbr_profile"] != "aggressive" {
		t.Fatalf("unexpected Hysteria2 outbound: %#v", got)
	}
}

func TestTranslateTUICOutbound(t *testing.T) {
	raw := map[string]any{
		"protocol": "tuic", "tag": "tuic-out",
		"settings": map[string]any{
			"servers": []any{map[string]any{"address": "example.com", "port": 443, "uuid": "u", "password": "p"}},
			"congestion_control": "bbr", "udp_relay_mode": "quic", "zero_rtt_handshake": false,
		},
	}
	got, err := TranslateXrayOutbound(raw)
	if err != nil { t.Fatal(err) }
	if got["type"] != "tuic" || got["uuid"] != "u" || got["password"] != "p" || got["congestion_control"] != "bbr" || got["udp_relay_mode"] != "quic" {
		t.Fatalf("unexpected TUIC outbound: %#v", got)
	}
}

func TestTranslateXrayDNSModernServers(t *testing.T) {
	raw := map[string]any{
		"queryStrategy": "UseIPv4",
		"servers": []any{"8.8.8.8", "https://dns.google/dns-query", "tls://1.1.1.1:853"},
	}
	got, err := TranslateXrayDNS(raw)
	if err != nil { t.Fatal(err) }
	servers, ok := got["servers"].([]map[string]any)
	if !ok || len(servers) != 3 { t.Fatalf("servers = %#v", got["servers"]) }
	if servers[0]["type"] != "udp" || servers[0]["server"] != "8.8.8.8" || servers[0]["server_port"] != 53 {
		t.Fatalf("unexpected UDP DNS server: %#v", servers[0])
	}
	if servers[1]["type"] != "https" || servers[1]["server"] != "dns.google" || servers[1]["path"] != "/dns-query" {
		t.Fatalf("unexpected DoH server: %#v", servers[1])
	}
	if servers[2]["type"] != "tls" || servers[2]["server_port"] != 853 {
		t.Fatalf("unexpected DoT server: %#v", servers[2])
	}
	if got["final"] != "dns-1" || got["strategy"] != "ipv4_only" {
		t.Fatalf("unexpected DNS defaults: %#v", got)
	}
}

func TestTranslateXrayRoutingUsesModernDomainStrategy(t *testing.T) {
	raw := map[string]any{
		"domainStrategy": "IPIfNonMatch",
		"rules": []any{
			map[string]any{"type":"field","domain":[]any{"example.com"},"outboundTag":"proxy"},
		},
	}
	got, err := TranslateXrayRouting(raw)
	if err != nil { t.Fatal(err) }
	if got["default_domain_strategy"] != "prefer_ipv4" {
		t.Fatalf("unexpected domain strategy: %#v", got["default_domain_strategy"])
	}
	rules, ok := got["rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["outbound"] != "proxy" {
		t.Fatalf("unexpected rules: %#v", got["rules"])
	}
}

func TestTranslateHysteriaUsers(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria",
		"tag": "hy",
		"port": 443,
		"settings": map[string]any{
			"clients": []any{
				map[string]any{"email": "alice", "auth": "secret"},
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	users, ok := got["users"].([]map[string]any)
	if !ok || len(users) != 1 || users[0]["name"] != "alice" || users[0]["auth_str"] != "secret" {
		t.Fatalf("unexpected Hysteria users: %#v", got["users"])
	}
}

func TestTranslateHysteria2Users(t *testing.T) {
	raw := map[string]any{
		"protocol": "hysteria2",
		"tag": "hy2",
		"port": 443,
		"settings": map[string]any{
			"clients": []any{
				map[string]any{"email": "alice", "password": "secret"},
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil { t.Fatal(err) }
	users, ok := got["users"].([]map[string]any)
	if !ok || len(users) != 1 || users[0]["name"] != "alice" || users[0]["password"] != "secret" {
		t.Fatalf("unexpected Hysteria2 users: %#v", got["users"])
	}
}

func TestTranslateXrayInboundReality(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "reality",
		"port": 443,
		"settings": map[string]any{
			"clients": []any{map[string]any{
				"id": "11111111-1111-1111-1111-111111111111",
				"email": "alice",
			}},
		},
		"streamSettings": map[string]any{
			"network": "tcp",
			"security": "reality",
			"tlsSettings": map[string]any{
				"serverName": "www.example.com",
				"realitySettings": map[string]any{
					"dest": "www.example.com:443",
					"privateKey": "private-key",
					"shortIds": []any{"0123456789abcdef"},
				},
			},
		},
	}
	got, err := TranslateXrayInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	tls, ok := got["tls"].(map[string]any)
	if !ok {
		t.Fatalf("missing tls: %#v", got)
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok || reality["enabled"] != true || reality["private_key"] != "private-key" {
		t.Fatalf("unexpected reality: %#v", tls["reality"])
	}
	handshake, ok := reality["handshake"].(map[string]any)
	if !ok || handshake["server"] != "www.example.com" || handshake["server_port"] != 443 {
		t.Fatalf("unexpected reality handshake: %#v", reality["handshake"])
	}
}

func TestTranslateXrayInboundRejectsRealityWithoutSettings(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "reality",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "tcp",
			"security": "reality",
		},
	}
	if _, err := TranslateXrayInbound(raw); err == nil {
		t.Fatal("expected REALITY settings error")
	}
}

func TestTranslateXrayInboundRejectsUnknownTransport(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "xhttp",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "xhttp",
		},
	}
	if _, err := TranslateXrayInbound(raw); err == nil {
		t.Fatal("expected unsupported transport error")
	}
}


func TestTranslateXrayOutboundFreedomAndVLESS(t *testing.T) {
	direct, err := TranslateXrayOutbound(map[string]any{
		"protocol": "freedom",
		"tag": "direct",
		"settings": map[string]any{},
	})
	if err != nil || direct["type"] != "direct" {
		t.Fatalf("unexpected direct outbound: %#v, err=%v", direct, err)
	}

	vless, err := TranslateXrayOutbound(map[string]any{
		"protocol": "vless",
		"tag": "proxy",
		"settings": map[string]any{
			"vnext": []any{map[string]any{
				"address": "example.com",
				"port": 443,
				"users": []any{map[string]any{"id": "11111111-1111-1111-1111-111111111111"}},
			}},
		},
		"streamSettings": map[string]any{
			"network": "grpc",
			"security": "tls",
			"tlsSettings": map[string]any{"serverName": "example.com"},
			"grpcSettings": map[string]any{"serviceName": "proxy"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if vless["type"] != "vless" || vless["server"] != "example.com" || vless["server_port"] != 443 {
		t.Fatalf("unexpected vless outbound: %#v", vless)
	}
}


func TestTranslateXrayRoutingAndDNS(t *testing.T) {
	route, err := TranslateXrayRouting(map[string]any{
		"domainStrategy": "AsIs",
		"rules": []any{map[string]any{
			"inboundTag": []any{"vless-in"},
			"domain": []any{"domain:example.com"},
			"outboundTag": "proxy",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules, ok := route["rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["outbound"] != "proxy" {
		t.Fatalf("unexpected routing translation: %#v", route)
	}

	dns, err := TranslateXrayDNS(map[string]any{
		"servers": []any{"1.1.1.1", map[string]any{"address": "8.8.8.8"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	servers, ok := dns["servers"].([]map[string]any)
	if !ok || len(servers) != 2 {
		t.Fatalf("unexpected DNS translation: %#v", dns)
	}
}


func TestV2RayStatsCodecRoundTrip(t *testing.T) {
	codec := v2rayProtoCodec{}
	req := &v2rayQueryRequest{
		Patterns: []string{"user>>>alice>>>traffic>>>uplink", "user>>>alice>>>traffic>>>downlink"},
		Reset:    true,
	}
	raw, err := codec.Marshal(req)
	if err != nil { t.Fatal(err) }
	if len(raw) == 0 || raw[0] != 0x12 {
		t.Fatalf("unexpected protobuf request: %x", raw)
	}

	statPayload := appendStringField(nil, 1, "user>>>alice>>>traffic>>>uplink")
	statPayload = appendVarintField(statPayload, 2, uint64(123))
	response := appendStringField(nil, 1, string(statPayload))
	var decoded v2rayQueryResponse
	if err := codec.Unmarshal(response, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Stats) != 1 || decoded.Stats[0].Name != "user>>>alice>>>traffic>>>uplink" || decoded.Stats[0].Value != 123 {
		t.Fatalf("unexpected decoded response: %#v", decoded.Stats)
	}
}


func TestClashConnectionDecode(t *testing.T) {
	raw := []byte(`{"connections":[{"id":"1","upload":10,"download":20,"chains":["direct"],"metadata":{"network":"tcp","type":"vless/vless-in","sourceIP":"203.0.113.10","sourcePort":"1234","destinationIP":"1.1.1.1","destinationPort":"443","host":"example.com"}}]}`)
	var payload clashConnectionsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Connections) != 1 {
		t.Fatalf("expected one connection, got %d", len(payload.Connections))
	}
	if payload.Connections[0].Metadata.SourceIP != "203.0.113.10" {
		t.Fatalf("unexpected source IP: %q", payload.Connections[0].Metadata.SourceIP)
	}
}


func TestConnectionAPIProtoDecode(t *testing.T) {
	connection := []byte{}
	connection = appendStringField(connection, 1, "conn-1")
	connection = appendStringField(connection, 2, "vless-in")
	connection = appendStringField(connection, 6, "203.0.113.10:54321")
	connection = appendStringField(connection, 10, "alice@example")
	connection = appendVarintField(connection, 12, uint64(1700000000))
	connection = appendStringField(connection, 21, "direct")

	event := []byte{}
	event = appendVarintField(event, 1, 0)
	event = appendStringField(event, 2, "conn-1")
	event = appendStringField(event, 3, string(connection))

	payload := []byte{}
	payload = appendStringField(payload, 1, string(event))
	payload = appendVarintField(payload, 2, 1)

	var response connectionEvents
	if err := (connectionAPIProtoCodec{}).Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Reset || len(response.Events) != 1 {
		t.Fatalf("unexpected response: reset=%v events=%d", response.Reset, len(response.Events))
	}
	got := response.Events[0].Connection
	if got == nil || got.User != "alice@example" || got.Source != "203.0.113.10:54321" {
		t.Fatalf("unexpected connection: %+v", got)
	}
}
