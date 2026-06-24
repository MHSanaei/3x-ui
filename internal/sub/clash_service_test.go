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
	svc := &SubClashService{SubService: &SubService{}}
	inbound := &model.Inbound{Listen: "203.0.113.1", Port: 443, Protocol: model.VLESS, Remark: "r", Settings: `{"encryption":"none"}`}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555"}
	stream := map[string]any{
		"network":         "tcp",
		"security":        "reality",
		"tcpSettings":     map[string]any{"header": map[string]any{"type": "none"}},
		"realitySettings": map[string]any{"serverName": "reality.example.com", "publicKey": "PBKvalue", "shortId": "ab12", "fingerprint": "chrome"},
	}

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, nil)
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

func TestApplyTransport_XHTTP_NoSettings(t *testing.T) {
	svc := &SubClashService{}
	proxy := map[string]any{}
	stream := map[string]any{}

	if !svc.applyTransport(proxy, "xhttp", stream) {
		t.Fatalf("applyTransport returned false for xhttp with no xhttpSettings")
	}
	if proxy["network"] != "xhttp" {
		t.Fatalf("network = %v, want xhttp", proxy["network"])
	}
	if _, exists := proxy["xhttp-opts"]; exists {
		t.Fatalf("xhttp-opts should be absent when xhttpSettings is missing, got %#v", proxy["xhttp-opts"])
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
	svc := &SubClashService{SubService: &SubService{}}
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

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, nil)

	if proxy["encryption"] != encryption {
		t.Fatalf("encryption = %v, want %q", proxy["encryption"], encryption)
	}
}

func TestBuildProxy_VLESSFlowXhttpRealityVlessenc(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{}}
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

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, nil)

	if proxy["flow"] != "xtls-rprx-vision" {
		t.Fatalf("xhttp+reality+vlessenc Clash proxy must carry the vision flow (#5232): %#v", proxy)
	}
}

func TestBuildProxy_VLESSFlowDroppedWithoutVisionSupport(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{}}
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

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, nil)

	if _, ok := proxy["flow"]; ok {
		t.Fatalf("tcp without tls/reality must not carry a flow: %#v", proxy)
	}
}

func TestBuildProxy_VLESSNoneEncryptionOmittedForClash(t *testing.T) {
	svc := &SubClashService{SubService: &SubService{}}
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

	proxy := svc.buildProxy(svc.SubService, inbound, client, stream, nil)

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

func TestBuildXhttpClashOpts_FullFieldMapping(t *testing.T) {
	xhttp := map[string]any{
		"path":                 "/api/v1",
		"mode":                 "stream-up",
		"host":                 "example.com",
		"xPaddingBytes":        "100-1000",
		"xPaddingObfsMode":     true,
		"xPaddingKey":          "mykey",
		"xPaddingHeader":       "X-Trace-ID",
		"xPaddingPlacement":    "queryInHeader",
		"xPaddingMethod":       "tokenish",
		"uplinkHTTPMethod":     "POST",
		"sessionIDPlacement":   "query",
		"sessionIDKey":         "sess",
		"sessionIDTable":       "Base62",
		"sessionIDLength":      "16-32",
		"seqPlacement":         "header",
		"seqKey":               "seq",
		"uplinkDataPlacement":  "body",
		"uplinkDataKey":        "udata",
		"uplinkChunkSize":      "64-256",
		"noGRPCHeader":         true,
		"scMaxEachPostBytes":   "500000",
		"scMinPostsIntervalMs": "50",
		"xmux": map[string]any{
			"maxConcurrency":   "16-32",
			"maxConnections":   "4",
			"cMaxReuseTimes":   "8",
			"hMaxRequestTimes": "600-900",
			"hMaxReusableSecs": "1800-3000",
			"hKeepAlivePeriod": float64(60),
		},
		"headers": map[string]any{
			"User-Agent": "chrome",
			"Host":       "should-be-dropped.com",
		},
	}

	opts := buildXhttpClashOpts(xhttp)
	if opts == nil {
		t.Fatal("expected non-nil opts for full field mapping")
	}

	// Direct fields
	if opts["path"] != "/api/v1" {
		t.Errorf("path = %v, want /api/v1", opts["path"])
	}
	if opts["mode"] != "stream-up" {
		t.Errorf("mode = %v, want stream-up", opts["mode"])
	}
	if opts["host"] != "example.com" {
		t.Errorf("host = %v, want example.com", opts["host"])
	}

	// String fields
	if opts["x-padding-bytes"] != "100-1000" {
		t.Errorf("x-padding-bytes = %v", opts["x-padding-bytes"])
	}
	if opts["uplink-http-method"] != "POST" {
		t.Errorf("uplink-http-method = %v", opts["uplink-http-method"])
	}
	if opts["session-id-placement"] != "query" {
		t.Errorf("session-id-placement = %v", opts["session-id-placement"])
	}
	if opts["session-id-key"] != "sess" {
		t.Errorf("session-id-key = %v", opts["session-id-key"])
	}
	if opts["session-id-table"] != "Base62" {
		t.Errorf("session-id-table = %v", opts["session-id-table"])
	}
	if opts["session-id-length"] != "16-32" {
		t.Errorf("session-id-length = %v", opts["session-id-length"])
	}
	if opts["seq-placement"] != "header" {
		t.Errorf("seq-placement = %v", opts["seq-placement"])
	}
	if opts["seq-key"] != "seq" {
		t.Errorf("seq-key = %v", opts["seq-key"])
	}
	if opts["uplink-data-placement"] != "body" {
		t.Errorf("uplink-data-placement = %v", opts["uplink-data-placement"])
	}
	if opts["uplink-data-key"] != "udata" {
		t.Errorf("uplink-data-key = %v", opts["uplink-data-key"])
	}

	// DPI-filtered fields (non-default values should pass)
	if opts["sc-max-each-post-bytes"] != "500000" {
		t.Errorf("sc-max-each-post-bytes = %v", opts["sc-max-each-post-bytes"])
	}
	if opts["sc-min-posts-interval-ms"] != "50" {
		t.Errorf("sc-min-posts-interval-ms = %v", opts["sc-min-posts-interval-ms"])
	}

	// Bool fields
	if opts["no-grpc-header"] != true {
		t.Errorf("no-grpc-header = %v, want true", opts["no-grpc-header"])
	}
	if opts["x-padding-obfs-mode"] != true {
		t.Errorf("x-padding-obfs-mode = %v, want true", opts["x-padding-obfs-mode"])
	}

	// Padding obfs gated fields
	if opts["x-padding-key"] != "mykey" {
		t.Errorf("x-padding-key = %v", opts["x-padding-key"])
	}
	if opts["x-padding-header"] != "X-Trace-ID" {
		t.Errorf("x-padding-header = %v", opts["x-padding-header"])
	}
	if opts["x-padding-placement"] != "queryInHeader" {
		t.Errorf("x-padding-placement = %v", opts["x-padding-placement"])
	}
	if opts["x-padding-method"] != "tokenish" {
		t.Errorf("x-padding-method = %v", opts["x-padding-method"])
	}

	// Non-zero value fields
	if opts["uplink-chunk-size"] != "64-256" {
		t.Errorf("uplink-chunk-size = %v", opts["uplink-chunk-size"])
	}

	// Reuse-settings (xmux)
	reuse, ok := opts["reuse-settings"].(map[string]any)
	if !ok {
		t.Fatalf("reuse-settings missing or wrong type: %#v", opts["reuse-settings"])
	}
	if reuse["max-concurrency"] != "16-32" {
		t.Errorf("max-concurrency = %v", reuse["max-concurrency"])
	}
	if reuse["max-connections"] != "4" {
		t.Errorf("max-connections = %v", reuse["max-connections"])
	}
	if reuse["c-max-reuse-times"] != "8" {
		t.Errorf("c-max-reuse-times = %v", reuse["c-max-reuse-times"])
	}
	if reuse["h-max-request-times"] != "600-900" {
		t.Errorf("h-max-request-times = %v", reuse["h-max-request-times"])
	}
	if reuse["h-max-reusable-secs"] != "1800-3000" {
		t.Errorf("h-max-reusable-secs = %v", reuse["h-max-reusable-secs"])
	}
	if reuse["h-keep-alive-period"] != float64(60) {
		t.Errorf("h-keep-alive-period = %v, want 60", reuse["h-keep-alive-period"])
	}

	// Headers (Host should be dropped)
	headers, ok := opts["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing or wrong type: %#v", opts["headers"])
	}
	if headers["User-Agent"] != "chrome" {
		t.Errorf("headers[User-Agent] = %v", headers["User-Agent"])
	}
	if _, has := headers["Host"]; has {
		t.Error("headers should not contain Host key")
	}
	if _, has := headers["host"]; has {
		t.Error("headers should not contain host key (case-insensitive)")
	}
}

func TestBuildXhttpClashOpts_DPIDefaultsFiltered(t *testing.T) {
	xhttp := map[string]any{
		"path":                 "/",
		"mode":                 "stream-up",
		"scMaxEachPostBytes":   "1000000",
		"scMinPostsIntervalMs": "30",
	}
	opts := buildXhttpClashOpts(xhttp)
	if opts == nil {
		t.Fatal("expected non-nil opts (path and mode should be present)")
	}
	if _, has := opts["sc-max-each-post-bytes"]; has {
		t.Error("sc-max-each-post-bytes should be filtered when value is 1000000")
	}
	if _, has := opts["sc-min-posts-interval-ms"]; has {
		t.Error("sc-min-posts-interval-ms should be filtered when value is 30")
	}
}

func TestBuildXhttpClashOpts_PaddingObfsGate(t *testing.T) {
	// Sub-test 1: obfs mode false — gated fields should not appear
	t.Run("ObfsModeFalse", func(t *testing.T) {
		xhttp := map[string]any{
			"path":             "/",
			"xPaddingObfsMode": false,
			"xPaddingKey":      "should-not-appear",
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if _, has := opts["x-padding-obfs-mode"]; has {
			t.Error("x-padding-obfs-mode should not appear when false")
		}
		if _, has := opts["x-padding-key"]; has {
			t.Error("x-padding-key should not appear when obfs mode is false")
		}
	})

	// Sub-test 2: obfs mode absent — gated fields should not appear
	t.Run("ObfsModeAbsent", func(t *testing.T) {
		xhttp := map[string]any{
			"path":        "/",
			"xPaddingKey": "should-not-appear",
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if _, has := opts["x-padding-key"]; has {
			t.Error("x-padding-key should not appear when obfs mode is absent")
		}
	})

	// Sub-test 3: obfs mode true with no gated fields — only x-padding-obfs-mode appears
	t.Run("ObfsModeTrueNoGatedFields", func(t *testing.T) {
		xhttp := map[string]any{
			"path":             "/",
			"xPaddingObfsMode": true,
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if opts["x-padding-obfs-mode"] != true {
			t.Errorf("x-padding-obfs-mode = %v, want true", opts["x-padding-obfs-mode"])
		}
		if _, has := opts["x-padding-key"]; has {
			t.Error("x-padding-key should not appear when not set")
		}
	})
}

func TestBuildXhttpClashOpts_XmuxMapsToReuseSettings(t *testing.T) {
	// Sub-test 1: full xmux mapping
	t.Run("FullXmux", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
			"xmux": map[string]any{
				"maxConcurrency":   "16-32",
				"maxConnections":   "4",
				"cMaxReuseTimes":   "8",
				"hMaxRequestTimes": "600-900",
				"hMaxReusableSecs": "1800-3000",
				"hKeepAlivePeriod": float64(60),
			},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		reuse, ok := opts["reuse-settings"].(map[string]any)
		if !ok {
			t.Fatalf("reuse-settings missing or wrong type: %#v", opts["reuse-settings"])
		}
		if reuse["max-concurrency"] != "16-32" {
			t.Errorf("max-concurrency = %v", reuse["max-concurrency"])
		}
		if reuse["max-connections"] != "4" {
			t.Errorf("max-connections = %v", reuse["max-connections"])
		}
		if reuse["c-max-reuse-times"] != "8" {
			t.Errorf("c-max-reuse-times = %v", reuse["c-max-reuse-times"])
		}
		if reuse["h-max-request-times"] != "600-900" {
			t.Errorf("h-max-request-times = %v", reuse["h-max-request-times"])
		}
		if reuse["h-max-reusable-secs"] != "1800-3000" {
			t.Errorf("h-max-reusable-secs = %v", reuse["h-max-reusable-secs"])
		}
		if reuse["h-keep-alive-period"] != float64(60) {
			t.Errorf("h-keep-alive-period = %v, want 60", reuse["h-keep-alive-period"])
		}
	})

	// Sub-test 2: empty xmux map — no reuse-settings key
	t.Run("EmptyXmux", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
			"xmux": map[string]any{},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts (path is present)")
		}
		if _, has := opts["reuse-settings"]; has {
			t.Error("reuse-settings should not appear for empty xmux")
		}
	})

	// Sub-test 3: hKeepAlivePeriod as int (not float64)
	t.Run("IntKeepAlivePeriod", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
			"xmux": map[string]any{
				"hKeepAlivePeriod": int(60),
			},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		reuse, ok := opts["reuse-settings"].(map[string]any)
		if !ok {
			t.Fatalf("reuse-settings missing: %#v", opts["reuse-settings"])
		}
		if reuse["h-keep-alive-period"] != int(60) {
			t.Errorf("h-keep-alive-period = %v (%T), want 60 (int)", reuse["h-keep-alive-period"], reuse["h-keep-alive-period"])
		}
	})

	// Sub-test 4: hKeepAlivePeriod=0 should be filtered
	t.Run("ZeroKeepAlivePeriod", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
			"xmux": map[string]any{
				"hKeepAlivePeriod": float64(0),
			},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if _, has := opts["reuse-settings"]; has {
			t.Error("reuse-settings should not appear when only hKeepAlivePeriod=0")
		}
	})
}

func TestBuildXhttpClashOpts_ServerOnlyFieldsExcluded(t *testing.T) {
	xhttp := map[string]any{
		"path":                 "/",
		"noSSEHeader":          true,
		"scMaxBufferedPosts":   "100",
		"scStreamUpServerSecs": "5",
		"serverMaxHeaderBytes": "4096",
	}
	opts := buildXhttpClashOpts(xhttp)
	if opts == nil {
		t.Fatal("expected non-nil opts (path is present)")
	}
	if _, has := opts["no-sse-header"]; has {
		t.Error("noSSEHeader should not appear in Clash output (server-only)")
	}
	if _, has := opts["sc-max-buffered-posts"]; has {
		t.Error("scMaxBufferedPosts should not appear in Clash output (server-only)")
	}
	if _, has := opts["sc-stream-up-server-secs"]; has {
		t.Error("scStreamUpServerSecs should not appear in Clash output (server-only)")
	}
	if _, has := opts["server-max-header-bytes"]; has {
		t.Error("serverMaxHeaderBytes should not appear in Clash output (not in Mihomo)")
	}
}

func TestBuildXhttpClashOpts_NilInput(t *testing.T) {
	opts := buildXhttpClashOpts(nil)
	if opts != nil {
		t.Fatalf("expected nil for nil input, got %#v", opts)
	}
}

func TestBuildXhttpClashOpts_EmptyInput(t *testing.T) {
	opts := buildXhttpClashOpts(map[string]any{})
	if opts != nil {
		t.Fatalf("expected nil for empty input, got %#v", opts)
	}
}

func TestBuildXhttpClashOpts_HostFallbackFromHeaders(t *testing.T) {
	// Sub-test 1: host from headers.Host
	t.Run("HostFromHeaders", func(t *testing.T) {
		xhttp := map[string]any{
			"path":    "/",
			"headers": map[string]any{"Host": "via-header.example.com"},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if opts["host"] != "via-header.example.com" {
			t.Errorf("host = %v, want via-header.example.com", opts["host"])
		}
	})

	// Sub-test 2: headers only contains Host — no headers key in output
	t.Run("HeadersOnlyHost", func(t *testing.T) {
		xhttp := map[string]any{
			"path":    "/",
			"headers": map[string]any{"Host": "only-host.example.com"},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if _, has := opts["headers"]; has {
			t.Error("headers key should not appear when only Host is present (Host is extracted to top-level)")
		}
	})

	// Sub-test 3: case-insensitive Host drop
	t.Run("CaseInsensitiveHostDrop", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
			"host": "explicit.example.com",
			"headers": map[string]any{
				"host":     "lowercase-host.example.com",
				"X-Custom": "value",
			},
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if opts["host"] != "explicit.example.com" {
			t.Errorf("host = %v, want explicit.example.com (explicit host wins)", opts["host"])
		}
		headers, ok := opts["headers"].(map[string]any)
		if !ok {
			t.Fatal("headers should be present (X-Custom remains)")
		}
		if _, has := headers["host"]; has {
			t.Error("lowercase 'host' should be dropped from headers")
		}
		if headers["X-Custom"] != "value" {
			t.Errorf("X-Custom = %v, want value", headers["X-Custom"])
		}
	})
}

func TestBuildXhttpClashOpts_NoGRPCHeaderFalsey(t *testing.T) {
	// Sub-test 1: noGRPCHeader: false
	t.Run("ExplicitFalse", func(t *testing.T) {
		xhttp := map[string]any{
			"path":         "/",
			"noGRPCHeader": false,
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts (path is present)")
		}
		if _, has := opts["no-grpc-header"]; has {
			t.Error("no-grpc-header should not appear when noGRPCHeader is false")
		}
	})

	// Sub-test 2: noGRPCHeader absent
	t.Run("Absent", func(t *testing.T) {
		xhttp := map[string]any{
			"path": "/",
		}
		opts := buildXhttpClashOpts(xhttp)
		if opts == nil {
			t.Fatal("expected non-nil opts")
		}
		if _, has := opts["no-grpc-header"]; has {
			t.Error("no-grpc-header should not appear when absent")
		}
	})
}
