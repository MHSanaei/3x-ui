package sub

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// TestBuildProxy_VLESSRealityFieldsForClash locks the reality field mapping in
// applySecurity (clash_service.go ~488): a regression that drops servername,
// public-key, short-id, or client-fingerprint would hand mihomo a broken reality
// proxy. The existing clash tests don't assert any of these.
func TestBuildProxy_VLESSRealityFieldsForClash(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{remarkModel: "-i"}}
	inbound := &model.Inbound{Listen: "203.0.113.1", Port: 443, Protocol: model.VLESS, Remark: "r", Settings: `{"encryption":"none"}`}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555"}
	stream := map[string]any{
		"network":         "tcp",
		"security":        "reality",
		"tcpSettings":     map[string]any{"header": map[string]any{"type": "none"}},
		"realitySettings": map[string]any{"serverName": "reality.example.com", "publicKey": "PBKvalue", "shortId": "ab12", "fingerprint": "chrome"},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, "")
	if proxy == nil {
		t.Fatal("buildProxy returned nil for a valid reality stream")
	}
	if proxy["tls"] != true {
		t.Fatalf("tls = %v, want true", proxy["tls"])
	}
	if proxy["servername"] != "reality.example.com" {
		t.Fatalf("servername = %v, want reality.example.com", proxy["servername"])
	}
	if proxy["client-fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome", proxy["client-fingerprint"])
	}
	opts, _ := proxy["reality-opts"].(map[string]any)
	if opts == nil {
		t.Fatal("reality-opts missing")
	}
	if opts["public-key"] != "PBKvalue" {
		t.Fatalf("public-key = %v, want PBKvalue", opts["public-key"])
	}
	if opts["short-id"] != "ab12" {
		t.Fatalf("short-id = %v, want ab12", opts["short-id"])
	}
}

// TestApplyTransport_TCPHeader pins the tcp-header validation (clash_service.go ~359):
// plain tcp and a "none" header are representable in clash; a non-none obfs header is
// not, so applyTransport must reject it (returning false drops it from the YAML).
func TestApplyTransport_TCPHeader(t *testing.T) {
	svc := &SubClashService{}
	if !svc.applyTransport(map[string]any{}, "tcp", map[string]any{}) {
		t.Fatal("plain tcp must be buildable")
	}
	noneStream := map[string]any{"tcpSettings": map[string]any{"header": map[string]any{"type": "none"}}}
	if !svc.applyTransport(map[string]any{}, "tcp", noneStream) {
		t.Fatal("tcp + header type none must be buildable")
	}
	httpStream := map[string]any{"tcpSettings": map[string]any{"header": map[string]any{"type": "http"}}}
	if svc.applyTransport(map[string]any{}, "tcp", httpStream) {
		t.Fatal("tcp + non-none (http) header is not representable in clash and must be rejected")
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

func TestBuildProxy_VLESSPostQuantumEncryptionUsesMihomoEncryptionField(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{remarkModel: "-i"}}
	encryption := "mlkem768x25519plus.native.0rtt.client"
	inbound := &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "pq",
		Settings: `{"encryption":"` + encryption + `"}`,
	}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555"}
	stream := map[string]any{
		"network": "xhttp",
		"xhttpSettings": map[string]any{
			"path": "/",
			"mode": "auto",
		},
		"security": "reality",
		"realitySettings": map[string]any{
			"publicKey":  "pub",
			"serverName": "example.com",
			"shortId":    "abcd",
		},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, "")

	if proxy["encryption"] != encryption {
		t.Fatalf("encryption = %v, want %q", proxy["encryption"], encryption)
	}
}

func TestBuildProxy_VLESSFlowXhttpRealityVlessenc(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{remarkModel: "-i"}}
	encryption := "mlkem768x25519plus.native.0rtt.client"
	inbound := &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "pq-flow",
		Settings: `{"encryption":"` + encryption + `"}`,
	}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555", Flow: "xtls-rprx-vision"}
	stream := map[string]any{
		"network": "xhttp",
		"xhttpSettings": map[string]any{
			"path": "/",
			"mode": "auto",
		},
		"security": "reality",
		"realitySettings": map[string]any{
			"publicKey":  "pub",
			"serverName": "example.com",
			"shortId":    "abcd",
		},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, "")

	if proxy["flow"] != "xtls-rprx-vision" {
		t.Fatalf("xhttp+reality+vlessenc Clash proxy must carry the vision flow (#5232): %#v", proxy)
	}
}

func TestBuildProxy_VLESSFlowDroppedWithoutVisionSupport(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{remarkModel: "-i"}}
	inbound := &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "plain-flow",
		Settings: `{"encryption":"none"}`,
	}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555", Flow: "xtls-rprx-vision"}
	stream := map[string]any{
		"network":  "tcp",
		"security": "none",
		"tcpSettings": map[string]any{
			"header": map[string]any{"type": "none"},
		},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, "")

	if _, ok := proxy["flow"]; ok {
		t.Fatalf("tcp without tls/reality must not carry a flow: %#v", proxy)
	}
}

func TestBuildProxy_VLESSNoneEncryptionOmittedForClash(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{remarkModel: "-i"}}
	inbound := &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "plain",
		Settings: `{"encryption":"none"}`,
	}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555"}
	stream := map[string]any{
		"network":  "tcp",
		"security": "none",
		"tcpSettings": map[string]any{
			"header": map[string]any{"type": "none"},
		},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, "")

	if _, ok := proxy["encryption"]; ok {
		t.Fatalf("plain vless encryption should be omitted for mihomo: %#v", proxy)
	}
	// The rest of the proxy must still be well-formed — otherwise a mutant that
	// drops encryption *and* corrupts a core field passes the absence check alone.
	if proxy["type"] != "vless" {
		t.Fatalf("type = %v, want vless", proxy["type"])
	}
	if proxy["server"] != "203.0.113.1" {
		t.Fatalf("server = %v, want 203.0.113.1", proxy["server"])
	}
	if proxy["port"] != 443 {
		t.Fatalf("port = %v, want 443", proxy["port"])
	}
	if proxy["uuid"] != client.ID {
		t.Fatalf("uuid = %v, want %v", proxy["uuid"], client.ID)
	}
}
