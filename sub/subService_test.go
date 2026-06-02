package sub

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestSubscriptionExpiryFromClient(t *testing.T) {
	const now = int64(1_700_000_000_000)
	const oneDayMs = int64(86_400_000)
	if got := subscriptionExpiryFromClient(now, 0); got != 0 {
		t.Fatalf("zero expiry should stay zero, got %d", got)
	}
	if got := subscriptionExpiryFromClient(now, 1_700_000_000_000); got != 1_700_000_000_000 {
		t.Fatalf("positive expiry should pass through, got %d", got)
	}
	if got := subscriptionExpiryFromClient(now, -oneDayMs); got != now+oneDayMs {
		t.Fatalf("delayed-start expiry should be now+|value|, got %d, want %d", got, now+oneDayMs)
	}
	if a, b := subscriptionExpiryFromClient(now, -oneDayMs), subscriptionExpiryFromClient(now, -oneDayMs); a != b {
		t.Fatalf("same now+value should be deterministic across calls, got %d vs %d (#4545 review)", a, b)
	}
}

func TestFindClientIndex(t *testing.T) {
	clients := []model.Client{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
		{Email: "c@example.com"},
	}
	if got := findClientIndex(clients, "b@example.com"); got != 1 {
		t.Fatalf("findClientIndex middle = %d, want 1", got)
	}
	if got := findClientIndex(clients, "a@example.com"); got != 0 {
		t.Fatalf("findClientIndex first = %d, want 0", got)
	}
	if got := findClientIndex(clients, "missing@example.com"); got != -1 {
		t.Fatalf("findClientIndex missing = %d, want -1", got)
	}
	if got := findClientIndex(nil, "x"); got != -1 {
		t.Fatalf("findClientIndex on nil slice = %d, want -1", got)
	}
}

func TestIsRoutableHost(t *testing.T) {
	routable := []string{"example.com", "sub.example.com", "10.0.0.1", "192.168.1.5", "1.2.3.4", "2001:db8::1"}
	for _, v := range routable {
		if !isRoutableHost(v) {
			t.Fatalf("isRoutableHost(%q) = false, want true", v)
		}
	}
	notRoutable := []string{"", "0.0.0.0", "::", "::0", "127.0.0.1", "127.0.0.2", "::1", "[::1]"}
	for _, v := range notRoutable {
		if isRoutableHost(v) {
			t.Fatalf("isRoutableHost(%q) = true, want false", v)
		}
	}
}

func TestResolveInboundAddress(t *testing.T) {
	const reqHost = "sub.example.com"

	// A subscriber reaches the panel through reqHost; the inbound's own
	// bind Listen IP (loopback, private, or even a public secondary IP) is
	// a server-side detail and must never become the link's connect host.
	t.Run("bind listen IP must not leak into the link host", func(t *testing.T) {
		s := &SubService{address: reqHost}
		for _, listen := range []string{"127.0.0.1", "10.0.0.5", "192.168.1.10", "1.2.3.4", "0.0.0.0", "::", "::0", ""} {
			ib := &model.Inbound{Listen: listen}
			if got := s.resolveInboundAddress(ib); got != reqHost {
				t.Fatalf("listen %q: address = %q, want %q (subscriber host, not bind IP)", listen, got, reqHost)
			}
		}
	})

	t.Run("node-managed inbound uses the node address", func(t *testing.T) {
		id := 7
		s := &SubService{
			address:   reqHost,
			nodesByID: map[int]*model.Node{7: {Id: 7, Address: "node7.example.com"}},
		}
		ib := &model.Inbound{NodeID: &id, Listen: "1.2.3.4"}
		if got := s.resolveInboundAddress(ib); got != "node7.example.com" {
			t.Fatalf("node-managed address = %q, want node7.example.com", got)
		}
	})

	t.Run("node id with no known node falls back to subscriber host", func(t *testing.T) {
		id := 9
		s := &SubService{address: reqHost, nodesByID: map[int]*model.Node{}}
		ib := &model.Inbound{NodeID: &id, Listen: "10.0.0.1"}
		if got := s.resolveInboundAddress(ib); got != reqHost {
			t.Fatalf("unknown-node address = %q, want subscriber host %q", got, reqHost)
		}
	})
}

func TestUnmarshalStreamSettings(t *testing.T) {
	got := unmarshalStreamSettings(`{"network":"ws","wsSettings":{"path":"/api"}}`)
	if got["network"] != "ws" {
		t.Fatalf("network = %v, want ws", got["network"])
	}
	ws, ok := got["wsSettings"].(map[string]any)
	if !ok || ws["path"] != "/api" {
		t.Fatalf("wsSettings = %v, want map with path=/api", got["wsSettings"])
	}
}

func TestUnmarshalStreamSettings_InvalidJSON(t *testing.T) {
	if got := unmarshalStreamSettings("not json"); got != nil {
		t.Fatalf("invalid JSON should produce nil map, got %#v", got)
	}
}

func TestSearchHost_StringValue(t *testing.T) {
	headers := map[string]any{"Host": "example.com"}
	if got := searchHost(headers); got != "example.com" {
		t.Fatalf("searchHost = %q, want example.com", got)
	}
}

func TestSearchHost_CaseInsensitiveKey(t *testing.T) {
	headers := map[string]any{"host": "example.com"}
	if got := searchHost(headers); got != "example.com" {
		t.Fatalf("searchHost = %q, want example.com", got)
	}
	headers2 := map[string]any{"HOST": "example.com"}
	if got := searchHost(headers2); got != "example.com" {
		t.Fatalf("searchHost uppercase = %q, want example.com", got)
	}
}

func TestSearchHost_ArrayValue(t *testing.T) {
	headers := map[string]any{"Host": []any{"first.example.com", "second.example.com"}}
	if got := searchHost(headers); got != "first.example.com" {
		t.Fatalf("searchHost array = %q, want first.example.com", got)
	}
}

func TestSearchHost_EmptyArray(t *testing.T) {
	headers := map[string]any{"Host": []any{}}
	if got := searchHost(headers); got != "" {
		t.Fatalf("searchHost empty array = %q, want empty", got)
	}
}

func TestSearchHost_NoHostKey(t *testing.T) {
	headers := map[string]any{"X-Other": "value"}
	if got := searchHost(headers); got != "" {
		t.Fatalf("searchHost no host = %q, want empty", got)
	}
}

func TestSearchHost_NotAMap(t *testing.T) {
	if got := searchHost("not a map"); got != "" {
		t.Fatalf("searchHost non-map = %q, want empty", got)
	}
	if got := searchHost(nil); got != "" {
		t.Fatalf("searchHost nil = %q, want empty", got)
	}
}

func TestSearchKey_FoundAtTopLevel(t *testing.T) {
	data := map[string]any{"foo": 42, "bar": "x"}
	got, ok := searchKey(data, "foo")
	if !ok {
		t.Fatal("expected to find foo")
	}
	if got != 42 {
		t.Fatalf("got %v, want 42", got)
	}
}

func TestSearchKey_FoundInNested(t *testing.T) {
	data := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"target": "hit",
			},
		},
	}
	got, ok := searchKey(data, "target")
	if !ok {
		t.Fatal("expected to find target in nested map")
	}
	if got != "hit" {
		t.Fatalf("got %v, want hit", got)
	}
}

func TestSearchKey_FoundInsideArray(t *testing.T) {
	data := map[string]any{
		"list": []any{
			map[string]any{"other": 1},
			map[string]any{"needle": "found"},
		},
	}
	got, ok := searchKey(data, "needle")
	if !ok {
		t.Fatal("expected to find needle in array element")
	}
	if got != "found" {
		t.Fatalf("got %v, want found", got)
	}
}

func TestSearchKey_NotFound(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	if _, ok := searchKey(data, "missing"); ok {
		t.Fatal("expected ok=false for missing key")
	}
}

func TestSearchKey_OnScalar(t *testing.T) {
	if _, ok := searchKey(42, "anything"); ok {
		t.Fatal("expected ok=false searching on a scalar")
	}
}

func TestBuildXhttpExtra_IncludesClientSideFieldsWhenPresent(t *testing.T) {
	extra := buildXhttpExtra(map[string]any{
		"path":                 "/xhttp",
		"host":                 "example.com",
		"mode":                 "packet-up",
		"xPaddingBytes":        "100-1000",
		"uplinkHTTPMethod":     "GET",
		"uplinkChunkSize":      float64(4096),
		"noGRPCHeader":         true,
		"scMinPostsIntervalMs": "20-40",
		"xmux": map[string]any{
			"maxConcurrency":   "16-32",
			"hMaxRequestTimes": "600-900",
			"hMaxReusableSecs": "1800-3000",
			"hKeepAlivePeriod": float64(15),
		},
		"downloadSettings": map[string]any{
			"network": "xhttp",
		},
		"headers": map[string]any{
			"Host":         "ignored.example.com",
			"X-Forwarded":  "1",
			"X-Test-Empty": "",
		},
	})

	if extra["path"] != nil || extra["host"] != nil {
		t.Fatalf("path/host should stay top-level, got extra %#v", extra)
	}
	for _, key := range []string{
		"xPaddingBytes",
		"uplinkHTTPMethod",
		"uplinkChunkSize",
		"noGRPCHeader",
		"scMinPostsIntervalMs",
		"xmux",
		"downloadSettings",
	} {
		if _, ok := extra[key]; !ok {
			t.Fatalf("extra missing %q: %#v", key, extra)
		}
	}
	if _, ok := extra["mode"]; ok {
		t.Fatalf("mode should stay as a top-level query parameter, got extra %#v", extra)
	}

	headers, ok := extra["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %#v, want map", extra["headers"])
	}
	if _, ok := headers["Host"]; ok {
		t.Fatalf("headers should not include Host: %#v", headers)
	}
	if headers["X-Forwarded"] != "1" {
		t.Fatalf("headers[X-Forwarded] = %#v, want 1", headers["X-Forwarded"])
	}
}

func TestBuildXhttpExtra_LeavesDefaultClientSideFieldsOut(t *testing.T) {
	extra := buildXhttpExtra(map[string]any{
		"uplinkHTTPMethod": "",
		"uplinkChunkSize":  float64(0),
		"noGRPCHeader":     false,
		"xmux":             map[string]any{},
		"downloadSettings": map[string]any{},
	})
	if extra != nil {
		t.Fatalf("default-only xhttp extra = %#v, want nil", extra)
	}
}

func TestCloneStringMap(t *testing.T) {
	src := map[string]string{"a": "1", "b": "2"}
	dst := cloneStringMap(src)
	if len(dst) != len(src) {
		t.Fatalf("clone length = %d, want %d", len(dst), len(src))
	}
	for k, v := range src {
		if dst[k] != v {
			t.Fatalf("clone[%q] = %q, want %q", k, dst[k], v)
		}
	}
	dst["a"] = "changed"
	if src["a"] == "changed" {
		t.Fatal("modifying clone leaked into source")
	}
}

func TestCloneStringMap_Empty(t *testing.T) {
	dst := cloneStringMap(map[string]string{})
	if dst == nil {
		t.Fatal("clone of empty map should not be nil")
	}
	if len(dst) != 0 {
		t.Fatalf("clone of empty map should be empty, got %v", dst)
	}
}

func TestGetHostFromXFH_HostOnly(t *testing.T) {
	got, err := getHostFromXFH("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "example.com" {
		t.Fatalf("got %q, want example.com", got)
	}
}

func TestGetHostFromXFH_HostWithPort(t *testing.T) {
	got, err := getHostFromXFH("example.com:8443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "example.com" {
		t.Fatalf("got %q, want example.com", got)
	}
}

func TestGetHostFromXFH_IPv6WithPort(t *testing.T) {
	got, err := getHostFromXFH("[2606:4700::1111]:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2606:4700::1111" {
		t.Fatalf("got %q, want 2606:4700::1111", got)
	}
}

func TestGetHostFromXFH_BadHostPort(t *testing.T) {
	if _, err := getHostFromXFH("example.com:8443:9999"); err == nil {
		t.Fatal("expected error for malformed host:port")
	}
}

func TestReadPositiveInt(t *testing.T) {
	cases := []struct {
		name    string
		in      any
		wantVal int
		wantOk  bool
	}{
		{"int_positive", int(5), 5, true},
		{"int_zero", int(0), 0, false},
		{"int_negative", int(-3), -3, false},
		{"int32_positive", int32(7), 7, true},
		{"int64_positive", int64(99), 99, true},
		{"float64_positive", float64(12), 12, true},
		{"float64_zero", float64(0.0), 0, false},
		{"float64_negative", float64(-1.5), -1, false},
		{"float32_positive", float32(3), 3, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotVal, gotOk := readPositiveInt(c.in)
			if gotVal != c.wantVal || gotOk != c.wantOk {
				t.Fatalf("readPositiveInt(%v) = (%d, %v), want (%d, %v)", c.in, gotVal, gotOk, c.wantVal, c.wantOk)
			}
		})
	}
}

func TestSetStringParam(t *testing.T) {
	p := map[string]string{"existing": "value"}

	setStringParam(p, "new", "hello")
	if p["new"] != "hello" {
		t.Fatalf("missing key after set: %v", p)
	}

	setStringParam(p, "existing", "")
	if _, ok := p["existing"]; ok {
		t.Fatalf("empty value should delete the key, got %v", p)
	}
}

func TestSetIntParam(t *testing.T) {
	p := map[string]string{"existing": "10"}

	setIntParam(p, "n", 42)
	if p["n"] != "42" {
		t.Fatalf("set positive int: got %v", p)
	}

	setIntParam(p, "existing", 0)
	if _, ok := p["existing"]; ok {
		t.Fatalf("zero value should delete the key, got %v", p)
	}

	p["other"] = "5"
	setIntParam(p, "other", -1)
	if _, ok := p["other"]; ok {
		t.Fatalf("negative value should delete the key, got %v", p)
	}
}

func TestSetStringField(t *testing.T) {
	f := map[string]any{"existing": "value"}

	setStringField(f, "new", "hello")
	if f["new"] != "hello" {
		t.Fatalf("missing key after set: %v", f)
	}

	setStringField(f, "existing", "")
	if _, ok := f["existing"]; ok {
		t.Fatalf("empty value should delete the key, got %v", f)
	}
}

func TestSetIntField(t *testing.T) {
	f := map[string]any{"existing": 10}

	setIntField(f, "n", 7)
	if f["n"] != 7 {
		t.Fatalf("set positive int: got %v", f)
	}

	setIntField(f, "existing", 0)
	if _, ok := f["existing"]; ok {
		t.Fatalf("zero value should delete the key, got %v", f)
	}
}

func TestBuildVmessLink(t *testing.T) {
	obj := map[string]any{
		"v":    "2",
		"ps":   "remark",
		"add":  "example.com",
		"port": 443,
		"net":  "tcp",
	}
	link := buildVmessLink(obj)
	if !strings.HasPrefix(link, "vmess://") {
		t.Fatalf("missing vmess:// prefix: %q", link)
	}
	payload := strings.TrimPrefix(link, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(decoded, &roundTrip); err != nil {
		t.Fatalf("decoded payload is not JSON: %v\n%s", err, decoded)
	}
	if roundTrip["add"] != "example.com" {
		t.Fatalf("round-trip add = %v, want example.com", roundTrip["add"])
	}
	if roundTrip["ps"] != "remark" {
		t.Fatalf("round-trip ps = %v, want remark", roundTrip["ps"])
	}
}

func TestCloneVmessShareObj_CopiesEverythingByDefault(t *testing.T) {
	base := map[string]any{
		"v":    "2",
		"sni":  "example.com",
		"alpn": "h2",
		"fp":   "chrome",
		"net":  "tcp",
	}
	out := cloneVmessShareObj(base, "tls")
	for _, key := range []string{"sni", "alpn", "fp", "net", "v"} {
		if _, ok := out[key]; !ok {
			t.Fatalf("expected key %q to be preserved when security=tls, got %v", key, out)
		}
	}
}

func TestCloneVmessShareObj_NoneStripsTLSOnlyKeys(t *testing.T) {
	base := map[string]any{
		"v":    "2",
		"sni":  "example.com",
		"alpn": "h2",
		"fp":   "chrome",
		"net":  "tcp",
	}
	out := cloneVmessShareObj(base, "none")
	for _, key := range []string{"sni", "alpn", "fp"} {
		if _, ok := out[key]; ok {
			t.Fatalf("security=none should strip %q, got %v", key, out)
		}
	}
	if out["v"] != "2" || out["net"] != "tcp" {
		t.Fatalf("non-TLS keys should remain, got %v", out)
	}
}

func TestApplyExternalProxyTLSParams_UsesProxyDomainAndOverrides(t *testing.T) {
	params := map[string]string{
		"security": "tls",
		"sni":      "origin.example.com",
		"fp":       "firefox",
		"alpn":     "h2",
	}
	ep := map[string]any{
		"dest":        "proxy.example.com",
		"sni":         "tls.example.com",
		"fingerprint": "chrome",
		"alpn":        []any{"h3", "h2"},
	}

	applyExternalProxyTLSParams(ep, params, "tls")

	if params["sni"] != "tls.example.com" {
		t.Fatalf("sni = %q, want tls.example.com", params["sni"])
	}
	if params["fp"] != "chrome" {
		t.Fatalf("fp = %q, want chrome", params["fp"])
	}
	if params["alpn"] != "h3,h2" {
		t.Fatalf("alpn = %q, want h3,h2", params["alpn"])
	}
}

func TestApplyExternalProxyTLSParams_PreservesUpstreamSNI(t *testing.T) {
	// External-proxy entry has no SNI of its own; its dest must not
	// clobber the upstream tlsSettings.serverName already written into
	// params. Regression: the dest fallback used to overwrite "222" with
	// "111" whenever an operator set forceTls=same and left the proxy's
	// SNI field blank.
	params := map[string]string{"security": "tls", "sni": "real.example.com"}
	ep := map[string]any{"dest": "proxy.example.com"}

	applyExternalProxyTLSParams(ep, params, "tls")

	if params["sni"] != "real.example.com" {
		t.Fatalf("sni = %q, want upstream sni preserved (real.example.com)", params["sni"])
	}
}

func TestApplyExternalProxyTLSParams_ExplicitSNIOverridesUpstream(t *testing.T) {
	params := map[string]string{"security": "tls", "sni": "real.example.com"}
	ep := map[string]any{"dest": "proxy.example.com", "sni": "edge.example.com"}

	applyExternalProxyTLSParams(ep, params, "tls")

	if params["sni"] != "edge.example.com" {
		t.Fatalf("sni = %q, want edge.example.com", params["sni"])
	}
}

func TestApplyExternalProxyTLSToStream_DoesNotLeakAcrossProxies(t *testing.T) {
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName": "upstream.example.com",
		},
	}
	proxies := []map[string]any{
		{"dest": "a.example.com", "sni": "a-sni.example.com", "fingerprint": "chrome", "alpn": []any{"h3"}},
		{"dest": "b.example.com"},
	}

	results := make([]map[string]any, 0, len(proxies))
	for _, ep := range proxies {
		working := cloneStreamForExternalProxy(stream)
		applyExternalProxyTLSToStream(ep, working, "tls")
		ts := working["tlsSettings"].(map[string]any)
		snapshot := map[string]any{
			"serverName":  ts["serverName"],
			"fingerprint": ts["fingerprint"],
			"alpn":        ts["alpn"],
		}
		results = append(results, snapshot)
	}

	if results[0]["serverName"] != "a-sni.example.com" || results[0]["fingerprint"] != "chrome" {
		t.Fatalf("proxy A snapshot = %v", results[0])
	}
	// Proxy B has no SNI of its own — the upstream tlsSettings serverName
	// must remain in place (no dest fallback) and no fingerprint/alpn
	// must leak from proxy A.
	if results[1]["serverName"] != "upstream.example.com" {
		t.Fatalf("proxy B serverName = %v, want upstream.example.com preserved", results[1]["serverName"])
	}
	if results[1]["fingerprint"] != nil {
		t.Fatalf("proxy B should inherit no fingerprint, got %v (leaked from A)", results[1]["fingerprint"])
	}
	if results[1]["alpn"] != nil {
		t.Fatalf("proxy B should inherit no alpn, got %v (leaked from A)", results[1]["alpn"])
	}
}

func TestApplyExternalProxyTLSParams_DoesNotApplyForNone(t *testing.T) {
	params := map[string]string{
		"security": "none",
		"sni":      "origin.example.com",
	}
	ep := map[string]any{
		"dest":        "proxy.example.com",
		"fingerprint": "chrome",
		"alpn":        []any{"h3"},
	}

	applyExternalProxyTLSParams(ep, params, "none")

	if params["sni"] != "origin.example.com" {
		t.Fatalf("sni should not change for security=none, got %q", params["sni"])
	}
	if _, ok := params["fp"]; ok {
		t.Fatalf("fp should not be set for security=none, got %v", params)
	}
	if _, ok := params["alpn"]; ok {
		t.Fatalf("alpn should not be set for security=none, got %v", params)
	}
}

func TestExtractKcpShareFields_Defaults(t *testing.T) {
	stream := map[string]any{}
	got := extractKcpShareFields(stream)
	if got.headerType != "none" {
		t.Fatalf("default headerType = %q, want none", got.headerType)
	}
	if got.seed != "" || got.mtu != 0 || got.tti != 0 {
		t.Fatalf("default kcpShareFields should be zero except headerType, got %+v", got)
	}
}

func TestExtractKcpShareFields_ReadsAllFields(t *testing.T) {
	stream := map[string]any{
		"kcpSettings": map[string]any{
			"header": map[string]any{"type": "wechat-video"},
			"seed":   "secret-seed",
			"mtu":    float64(1350),
			"tti":    float64(50),
		},
	}
	got := extractKcpShareFields(stream)
	if got.headerType != "wechat-video" {
		t.Fatalf("headerType = %q, want wechat-video", got.headerType)
	}
	if got.seed != "secret-seed" {
		t.Fatalf("seed = %q, want secret-seed", got.seed)
	}
	if got.mtu != 1350 {
		t.Fatalf("mtu = %d, want 1350", got.mtu)
	}
	if got.tti != 50 {
		t.Fatalf("tti = %d, want 50", got.tti)
	}
}

func TestExtractKcpShareFields_FinalMaskLegacyHeader(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{
					"type":     "mkcp-legacy",
					"settings": map[string]any{"header": "wechat", "value": ""},
				},
			},
		},
	}
	got := extractKcpShareFields(stream)
	if got.headerType != "wechat-video" {
		t.Fatalf("headerType = %q, want wechat-video", got.headerType)
	}
	if got.seed != "" {
		t.Fatalf("seed = %q, want empty for header mask", got.seed)
	}
}

func TestExtractKcpShareFields_FinalMaskLegacySeed(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{
					"type":     "mkcp-legacy",
					"settings": map[string]any{"header": "", "value": "obfs-pass"},
				},
			},
		},
	}
	got := extractKcpShareFields(stream)
	if got.headerType != "none" {
		t.Fatalf("headerType = %q, want none for empty-header legacy mask", got.headerType)
	}
	if got.seed != "obfs-pass" {
		t.Fatalf("seed = %q, want obfs-pass", got.seed)
	}
}

func TestKcpShareFields_ApplyToParams(t *testing.T) {
	params := map[string]string{}
	kcpShareFields{headerType: "wechat-video", seed: "s", mtu: 1350, tti: 50}.applyToParams(params)
	if params["headerType"] != "wechat-video" {
		t.Fatalf("headerType param = %q", params["headerType"])
	}
	if params["seed"] != "s" {
		t.Fatalf("seed param = %q", params["seed"])
	}
	if params["mtu"] != "1350" {
		t.Fatalf("mtu param = %q", params["mtu"])
	}
	if params["tti"] != "50" {
		t.Fatalf("tti param = %q", params["tti"])
	}
}

func TestKcpShareFields_ApplyToParams_NoneHeaderNotAdded(t *testing.T) {
	params := map[string]string{}
	kcpShareFields{headerType: "none"}.applyToParams(params)
	if _, ok := params["headerType"]; ok {
		t.Fatalf("headerType=none should not be added, got %v", params)
	}
}

func TestMarshalFinalMask_EmptyReturnsFalse(t *testing.T) {
	if _, ok := marshalFinalMask(map[string]any{}); ok {
		t.Fatal("expected ok=false for empty finalmask")
	}
	if _, ok := marshalFinalMask(nil); ok {
		t.Fatal("expected ok=false for nil finalmask")
	}
}

func TestMarshalFinalMask_WithContent(t *testing.T) {
	fm := map[string]any{
		"tcp": []any{
			map[string]any{"type": "fragment"},
		},
	}
	out, ok := marshalFinalMask(fm)
	if !ok {
		t.Fatal("expected ok=true for finalmask with valid tcp mask")
	}
	if !strings.Contains(out, `"tcp"`) {
		t.Fatalf("marshaled finalmask missing tcp key: %s", out)
	}
	if !strings.Contains(out, "fragment") {
		t.Fatalf("marshaled finalmask missing mask type: %s", out)
	}
}

func TestMarshalFinalMask_UnknownTypeIsDropped(t *testing.T) {
	fm := map[string]any{
		"tcp": []any{
			map[string]any{"type": "not-a-real-mask"},
		},
	}
	if _, ok := marshalFinalMask(fm); ok {
		t.Fatal("unknown mask types should be dropped, leaving nothing to marshal")
	}
}

func TestHasFinalMaskContent(t *testing.T) {
	if hasFinalMaskContent(nil) {
		t.Fatal("nil should not count as content")
	}
	if hasFinalMaskContent(map[string]any{}) {
		t.Fatal("empty map should not count as content")
	}
	if !hasFinalMaskContent(map[string]any{"x": 1}) {
		t.Fatal("non-empty map should count as content")
	}
}

func TestHysteriaHopPorts(t *testing.T) {
	withHop := func(ports any) map[string]any {
		return map[string]any{
			"finalmask": map[string]any{
				"quicParams": map[string]any{
					"udpHop": map[string]any{"ports": ports, "interval": "5-10"},
				},
			},
		}
	}

	cases := []struct {
		name   string
		stream map[string]any
		want   string
	}{
		{"range", withHop("20000-50000"), "20000-50000"},
		{"trimmed", withHop("  443,20000-50000  "), "443,20000-50000"},
		{"empty string", withHop(""), ""},
		{"non-string", withHop(float64(443)), ""},
		{"no udpHop", map[string]any{"finalmask": map[string]any{"quicParams": map[string]any{}}}, ""},
		{"no finalmask", map[string]any{}, ""},
		{"nil stream", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hysteriaHopPorts(tc.stream); got != tc.want {
				t.Fatalf("hysteriaHopPorts() = %q, want %q", got, tc.want)
			}
		})
	}
}
