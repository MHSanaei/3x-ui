package sub

import (
	"net/http"
	"testing"
	"time"
)

// TestClashExternalProxyFromHysteriaAndWireguard pins the two share-link kinds
// the Clash output builds itself: a library row of either kind has to render a
// usable proxy, and a link missing what the protocol needs has to render none
// rather than a broken entry.
func TestClashExternalProxyFromHysteriaAndWireguard(t *testing.T) {
	t.Run("hysteria2 with auth and tls", func(t *testing.T) {
		proxy := clashHysteriaFromExternal(
			map[string]any{"address": "hy.example.com", "port": float64(8443)},
			map[string]any{
				"hysteriaSettings": map[string]any{"auth": "s3cret"},
				"tlsSettings": map[string]any{
					"serverName":  "sni.example.com",
					"alpn":        []any{"h3", "h2"},
					"fingerprint": "chrome",
				},
			},
			"hy-node",
		)
		if proxy == nil {
			t.Fatal("a hysteria2 link with auth rendered no proxy")
		}
		want := map[string]any{
			"name":               "hy-node",
			"type":               "hysteria2",
			"server":             "hy.example.com",
			"port":               8443,
			"password":           "s3cret",
			"udp":                true,
			"sni":                "sni.example.com",
			"client-fingerprint": "chrome",
		}
		for key, value := range want {
			if proxy[key] != value {
				t.Errorf("proxy[%q] = %v, want %v", key, proxy[key], value)
			}
		}
		if alpn, ok := proxy["alpn"].([]string); !ok || len(alpn) != 2 || alpn[0] != "h3" {
			t.Errorf("proxy[alpn] = %v, want [h3 h2]", proxy["alpn"])
		}
	})

	t.Run("hysteria2 without auth is not rendered", func(t *testing.T) {
		proxy := clashHysteriaFromExternal(
			map[string]any{"address": "hy.example.com", "port": 443},
			map[string]any{"hysteriaSettings": map[string]any{}},
			"hy-node",
		)
		if proxy != nil {
			t.Fatalf("a hysteria2 link with no auth rendered %v", proxy)
		}
	})

	t.Run("wireguard with both address families", func(t *testing.T) {
		proxy := clashWireguardFromExternal(map[string]any{
			"secretKey": "priv-key",
			"address":   []any{"10.0.0.2/32", "fd00::2/128"},
			"peers": []any{map[string]any{
				"endpoint":     "wg.example.com:51820",
				"publicKey":    "pub-key",
				"preSharedKey": "psk",
			}},
		}, "wg-node")
		if proxy == nil {
			t.Fatal("a wireguard link rendered no proxy")
		}
		want := map[string]any{
			"name":           "wg-node",
			"type":           "wireguard",
			"server":         "wg.example.com",
			"port":           51820,
			"udp":            true,
			"private-key":    "priv-key",
			"public-key":     "pub-key",
			"pre-shared-key": "psk",
			"ip":             "10.0.0.2",
			"ipv6":           "fd00::2",
		}
		for key, value := range want {
			if proxy[key] != value {
				t.Errorf("proxy[%q] = %v, want %v", key, proxy[key], value)
			}
		}
	})

	t.Run("wireguard without a usable endpoint is not rendered", func(t *testing.T) {
		noPeers := clashWireguardFromExternal(map[string]any{"secretKey": "k"}, "wg-node")
		if noPeers != nil {
			t.Fatalf("a wireguard link with no peers rendered %v", noPeers)
		}
		noPort := clashWireguardFromExternal(map[string]any{
			"secretKey": "k",
			"peers":     []any{map[string]any{"endpoint": "wg.example.com", "publicKey": "p"}},
		}, "wg-node")
		if noPort != nil {
			t.Fatalf("a wireguard link whose endpoint has no port rendered %v", noPort)
		}
	})

	t.Run("host and port splitting", func(t *testing.T) {
		cases := []struct {
			endpoint string
			host     string
			port     int
		}{
			{"example.com:443", "example.com", 443},
			{" [2001:db8::1]:51820 ", "2001:db8::1", 51820},
			{"example.com", "example.com", 0},
			{"example.com:notaport", "example.com", 0},
		}
		for _, tc := range cases {
			host, port := splitClashHostPort(tc.endpoint)
			if host != tc.host || port != tc.port {
				t.Errorf("splitClashHostPort(%q) = (%q, %d), want (%q, %d)",
					tc.endpoint, host, port, tc.host, tc.port)
			}
		}
	})
}

// TestClashExternalProxyFromShareLinks drives the dispatcher on the kinds that
// borrow the inbound builder: a library row of any of these has to render the
// same proxy an inbound client of that protocol would, and one carrying a
// transport Clash cannot express has to render none rather than a wrong entry.
func TestClashExternalProxyFromShareLinks(t *testing.T) {
	svc := &SubClashService{}
	cases := []struct {
		name  string
		link  string
		want  map[string]any
		empty bool
	}{
		{
			name: "trojan over tls",
			link: "trojan://tpass@t.example.com:443?security=tls&type=tcp#node",
			want: map[string]any{
				"type": "trojan", "server": "t.example.com", "port": 443,
				"password": "tpass", "tls": true, "network": "tcp",
			},
		},
		{
			name: "shadowsocks",
			link: "ss://YWVzLTI1Ni1nY206c2VjcmV0@ss.example.com:8388#node",
			want: map[string]any{
				"type": "ss", "server": "ss.example.com", "port": 8388,
				"cipher": "aes-256-gcm", "password": "secret",
			},
		},
		{
			name: "vless over websocket",
			link: "vless://11111111-2222-3333-4444-555555555555@ws.example.com:8443" +
				"?security=tls&type=ws&path=%2Fws&host=cdn.example.com#node",
			want: map[string]any{
				"type": "vless", "server": "ws.example.com", "port": 8443,
				"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true,
			},
		},
		{
			name:  "a link Clash cannot express",
			link:  "vless://11111111-2222-3333-4444-555555555555@obfs.example.com:443?security=tls&type=tcp&headerType=http",
			empty: true,
		},
		{
			name:  "not a share link at all",
			link:  "just-some-text",
			empty: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := svc.clashProxyFromExternal(tc.link, "row-name")
			if tc.empty {
				if proxy != nil {
					t.Fatalf("a link Clash cannot represent rendered %v", proxy)
				}
				return
			}
			if proxy == nil {
				t.Fatalf("clashProxyFromExternal(%q) rendered nothing", tc.link)
			}
			if proxy["name"] != "row-name" || proxy["udp"] != true {
				t.Fatalf("proxy name/udp = %v/%v, want the row name and udp", proxy["name"], proxy["udp"])
			}
			for key, value := range tc.want {
				if proxy[key] != value {
					t.Errorf("proxy[%q] = %v, want %v", key, proxy[key], value)
				}
			}
		})
	}
}

// TestParseRetryAfter covers what a provider can actually send: a delay in
// seconds or an HTTP date, anything else ignored rather than turned into a
// negative or absurd wait.
func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "120", 120 * time.Second},
		{"zero seconds", "0", 0},
		{"negative seconds", "-30", 0},
		{"garbage", "soon", 0},
		{"past date", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.value); got != tc.want {
				t.Fatalf("parseRetryAfter(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}

	t.Run("a date in the future is honoured", func(t *testing.T) {
		got := parseRetryAfter(time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat))
		if got <= 60*time.Second || got > 90*time.Second {
			t.Fatalf("parseRetryAfter(a date 90s ahead) = %v, want about 90s", got)
		}
	})
}
