package sub

import (
	"reflect"
	"testing"
)

func TestEnsureUniqueProxyNames(t *testing.T) {
	proxies := []map[string]any{
		{"name": "", "type": "vless", "server": "a.com", "port": 443},
		{"name": "", "type": "vmess", "server": "b.com", "port": 8443},
		{"name": "node"},
		{"name": "node"},
		{"name": ""},
	}

	ensureUniqueProxyNames(proxies)

	seen := map[string]bool{}
	for i, p := range proxies {
		name, _ := p["name"].(string)
		if name == "" {
			t.Fatalf("proxy %d still has an empty name (mihomo would reject the config, #4641)", i)
		}
		if seen[name] {
			t.Fatalf("proxy %d has duplicate name %q (mihomo rejects the whole config, #4641)", i, name)
		}
		seen[name] = true
	}

	if got := proxies[0]["name"]; got != "vless-a.com-443" {
		t.Errorf("empty name fallback = %q, want vless-a.com-443", got)
	}
	if proxies[2]["name"] == proxies[3]["name"] {
		t.Errorf("duplicate %q was not disambiguated", proxies[2]["name"])
	}
	if got := proxies[4]["name"]; got != "proxy-5" {
		t.Errorf("typeless empty name fallback = %q, want proxy-5", got)
	}
}

func TestXrayDirectRulesToClash(t *testing.T) {
	raw := `[
		{"type":"field","outboundTag":"direct","domain":["geosite:cn","domain:example.com","full:exact.example","keyword:bank"],"ip":["geoip:cn","geoip:private","1.2.3.0/24","2001:db8::/32"]},
		{"type":"field","outboundTag":"proxy","domain":["geosite:google"],"ip":["geoip:us"]},
		{"type":"field","outboundTag":"direct","domain":["geosite:cn"],"ip":["geoip:cn"]}
	]`

	got := xrayDirectRulesToClash(raw)
	want := []string{
		"GEOSITE,cn,DIRECT",
		"DOMAIN-SUFFIX,example.com,DIRECT",
		"DOMAIN,exact.example,DIRECT",
		"DOMAIN-KEYWORD,bank,DIRECT",
		"GEOIP,CN,DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
		"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
		"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
		"IP-CIDR6,::1/128,DIRECT,no-resolve",
		"IP-CIDR,1.2.3.0/24,DIRECT,no-resolve",
		"IP-CIDR6,2001:db8::/32,DIRECT,no-resolve",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("xrayDirectRulesToClash() = %#v, want %#v", got, want)
	}
}

func TestXrayDirectRulesToClashIgnoresInvalidRules(t *testing.T) {
	cases := []string{
		"",
		"not-json",
		`[{"outboundTag":"direct","domain":["regexp:.*"]},{"outboundTag":"blocked","ip":["geoip:cn"]}]`,
	}

	for _, raw := range cases {
		if got := xrayDirectRulesToClash(raw); len(got) != 0 {
			t.Fatalf("xrayDirectRulesToClash(%q) = %#v, want empty", raw, got)
		}
	}
}

func TestApplyTransport_XHTTP(t *testing.T) {
	svc := &SubClashService{}
	proxy := map[string]any{}
	stream := map[string]any{
		"xhttpSettings": map[string]any{
			"path": "/xh",
			"host": "example.com",
			"mode": "auto",
		},
	}

	if !svc.applyTransport(proxy, "xhttp", stream) {
		t.Fatalf("applyTransport returned false for xhttp (#4531: would drop the inbound and yield an empty Clash YAML)")
	}
	if proxy["network"] != "xhttp" {
		t.Fatalf("network = %v, want xhttp", proxy["network"])
	}
	opts, ok := proxy["xhttp-opts"].(map[string]any)
	if !ok {
		t.Fatalf("xhttp-opts missing or wrong type: %#v", proxy["xhttp-opts"])
	}
	want := map[string]any{"path": "/xh", "host": "example.com", "mode": "auto"}
	if !reflect.DeepEqual(opts, want) {
		t.Fatalf("xhttp-opts = %#v, want %#v", opts, want)
	}
}

func TestApplyTransport_XHTTP_HostFromHeaders(t *testing.T) {
	svc := &SubClashService{}
	proxy := map[string]any{}
	stream := map[string]any{
		"xhttpSettings": map[string]any{
			"path":    "/xh",
			"headers": map[string]any{"Host": "via-header.example.com"},
		},
	}

	if !svc.applyTransport(proxy, "xhttp", stream) {
		t.Fatalf("applyTransport returned false for xhttp")
	}
	opts, _ := proxy["xhttp-opts"].(map[string]any)
	if opts["host"] != "via-header.example.com" {
		t.Fatalf("host should fall back to headers.Host, got %v", opts["host"])
	}
}

func TestApplyTransport_HTTPUpgrade(t *testing.T) {
	svc := &SubClashService{}
	proxy := map[string]any{}
	stream := map[string]any{
		"httpupgradeSettings": map[string]any{
			"path": "/hu",
			"host": "example.com",
		},
	}

	if !svc.applyTransport(proxy, "httpupgrade", stream) {
		t.Fatalf("applyTransport returned false for httpupgrade")
	}
	if proxy["network"] != "httpupgrade" {
		t.Fatalf("network = %v, want httpupgrade", proxy["network"])
	}
	opts, ok := proxy["http-upgrade-opts"].(map[string]any)
	if !ok {
		t.Fatalf("http-upgrade-opts missing: %#v", proxy["http-upgrade-opts"])
	}
	if opts["path"] != "/hu" {
		t.Fatalf("path = %v, want /hu", opts["path"])
	}
	headers, _ := opts["headers"].(map[string]any)
	if headers["Host"] != "example.com" {
		t.Fatalf("headers.Host = %v, want example.com", headers["Host"])
	}
}
