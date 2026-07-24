package link

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"testing"
)

func TestDefaultPort(t *testing.T) {
	cases := []struct {
		in   string
		def  int
		want int
	}{
		{"", 443, 443},
		{"8080", 443, 8080},
		{"0", 443, 443},   // non-positive falls back
		{"-1", 443, 443},  // negative falls back
		{"abc", 443, 443}, // unparseable falls back
		{"65535", 443, 65535},
	}
	for _, c := range cases {
		if got := defaultPort(c.in, c.def); got != c.want {
			t.Errorf("defaultPort(%q,%d) = %d, want %d", c.in, c.def, got, c.want)
		}
	}
}

func TestFirstNonEmptyAndParam(t *testing.T) {
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("firstNonEmpty(a,b) = %q, want a", got)
	}
	if got := firstNonEmpty("", "b"); got != "b" {
		t.Errorf("firstNonEmpty(,b) = %q, want b", got)
	}
	p := url.Values{"x": {""}, "y": {"hit"}, "z": {"z"}}
	if got := firstParam(p, "x", "y", "z"); got != "hit" {
		t.Errorf("firstParam = %q, want hit (first non-empty)", got)
	}
	if got := firstParam(p, "x"); got != "" {
		t.Errorf("firstParam(only empty) = %q, want empty", got)
	}
}

func TestSplitComma(t *testing.T) {
	if got := splitComma(""); got != nil {
		t.Errorf("splitComma(empty) = %v, want nil", got)
	}
	if got := splitComma("a, ,b ,, c"); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("splitComma trim/skip = %v, want [a b c]", got)
	}
	if got := splitCommaOrDefault("", []string{"d"}); !reflect.DeepEqual(got, []string{"d"}) {
		t.Errorf("splitCommaOrDefault(empty) = %v, want [d]", got)
	}
	if got := splitCommaOrDefault("x,y", []string{"d"}); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("splitCommaOrDefault(x,y) = %v, want [x y]", got)
	}
}

func TestPadAndBase64DecodeFlexible(t *testing.T) {
	if got := padBase64("abc"); got != "abc=" {
		t.Errorf("padBase64(abc) = %q, want abc=", got)
	}
	if got := padBase64("abcd"); got != "abcd" {
		t.Errorf("padBase64(abcd) = %q, want unchanged", got)
	}
	std := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:secret"))
	if got, err := base64DecodeFlexible(std); err != nil || got != "aes-256-gcm:secret" {
		t.Errorf("base64DecodeFlexible(std) = (%q,%v), want (aes-256-gcm:secret,nil)", got, err)
	}
	rawURL := base64.RawURLEncoding.EncodeToString([]byte("m:p"))
	if got, err := base64DecodeFlexible(rawURL); err != nil || got != "m:p" {
		t.Errorf("base64DecodeFlexible(rawurl) = (%q,%v), want (m:p,nil)", got, err)
	}
	if _, err := base64DecodeFlexible("!!!not!!!"); err == nil {
		t.Error("base64DecodeFlexible(garbage) should error")
	}
}

func TestDecodeHash(t *testing.T) {
	if got := decodeHash(""); got != "" {
		t.Errorf("decodeHash(empty) = %q, want empty", got)
	}
	if got := decodeHash("a%20b"); got != "a b" {
		t.Errorf("decodeHash(a%%20b) = %q, want 'a b'", got)
	}
	if got := decodeHash("plain"); got != "plain" {
		t.Errorf("decodeHash(plain) = %q, want plain", got)
	}
}

func TestCanonicalQuery_SortsKeys(t *testing.T) {
	// unsorted input must come out key-sorted for a stable identity
	got := canonicalQuery(url.Values{"c": {"3"}, "a": {"1"}, "b": {"2"}})
	if got != "a=1&b=2&c=3" {
		t.Fatalf("canonicalQuery = %q, want a=1&b=2&c=3", got)
	}
}

// stream navigates res.Outbound["streamSettings"][key] as a map.
func streamSub(t *testing.T, res *ParseResult, key string) map[string]any {
	t.Helper()
	ss, _ := res.Outbound["streamSettings"].(map[string]any)
	m, ok := ss[key].(map[string]any)
	if !ok {
		t.Fatalf("streamSettings.%s missing/not a map: %#v", key, ss)
	}
	return m
}

func TestParse_RealitySecurityMapped(t *testing.T) {
	res, err := ParseLink("vless://uuid@h.com:443?type=tcp&security=reality&pbk=PBK&sid=SID&sni=SNI&fp=firefox&spx=%2Fspx&pqv=PQV")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	re := streamSub(t, res, "realitySettings")
	for k, want := range map[string]string{"publicKey": "PBK", "shortId": "SID", "serverName": "SNI", "fingerprint": "firefox", "spiderX": "/spx", "mldsa65Verify": "PQV"} {
		if re[k] != want {
			t.Errorf("realitySettings[%q] = %v, want %q", k, re[k], want)
		}
	}
}

func TestParse_TLSSecurityMapped(t *testing.T) {
	res, err := ParseLink("trojan://pw@h.com:443?type=tcp&security=tls&sni=SNI&fp=chrome&alpn=h2,http/1.1&ech=ECH&pcs=PCS")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tls := streamSub(t, res, "tlsSettings")
	if tls["serverName"] != "SNI" || tls["fingerprint"] != "chrome" || tls["echConfigList"] != "ECH" || tls["pinnedPeerCertSha256"] != "PCS" {
		t.Errorf("tlsSettings fields = %#v", tls)
	}
	if alpn, _ := tls["alpn"].([]string); !reflect.DeepEqual(alpn, []string{"h2", "http/1.1"}) {
		t.Errorf("alpn = %#v, want [h2 http/1.1]", tls["alpn"])
	}
}

func TestParse_WSAndGRPCTransport(t *testing.T) {
	ws, err := ParseLink("vless://uuid@h.com:443?type=ws&host=H&path=%2Fwspath")
	if err != nil {
		t.Fatalf("parse ws: %v", err)
	}
	wss := streamSub(t, ws, "wsSettings")
	if wss["host"] != "H" || wss["path"] != "/wspath" {
		t.Errorf("wsSettings = %#v, want host=H path=/wspath", wss)
	}

	grpc, err := ParseLink("vless://uuid@h.com:443?type=grpc&serviceName=svc&authority=auth&mode=multi")
	if err != nil {
		t.Fatalf("parse grpc: %v", err)
	}
	gs := streamSub(t, grpc, "grpcSettings")
	if gs["serviceName"] != "svc" || gs["authority"] != "auth" || gs["multiMode"] != true {
		t.Errorf("grpcSettings = %#v, want serviceName=svc authority=auth multiMode=true", gs)
	}
}

func TestParse_XhttpExtraAndSnakeCaseFields(t *testing.T) {
	q := url.Values{}
	q.Set("type", "xhttp")
	q.Set("encryption", "none")
	q.Set("security", "none")
	q.Set("mode", "auto")
	q.Set("x_padding_bytes", "1-50")
	q.Set("extra", `{"mode":"auto","xPaddingBytes":"1-50","scMaxEachPostBytes":"1000000"}`)
	res, err := ParseLink("vless://uuid@h.com:443?" + q.Encode() + "#r")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	xh := streamSub(t, res, "xhttpSettings")
	if xh["xPaddingBytes"] != "1-50" {
		t.Errorf("xPaddingBytes = %v, want 1-50 (dropped from the snake_case/extra payload the emitter writes)", xh["xPaddingBytes"])
	}
	if xh["scMaxEachPostBytes"] != "1000000" {
		t.Errorf("scMaxEachPostBytes = %v, want 1000000 (dropped from the extra blob)", xh["scMaxEachPostBytes"])
	}
}

func TestParse_VmessWSPathWithoutHostKey(t *testing.T) {
	inner := `{"v":"2","add":"h","port":443,"id":"11111111-2222-4333-8444-555555555555","net":"ws","path":"/api","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(inner))
	res, err := ParseLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wss := streamSub(t, res, "wsSettings")
	if wss["path"] != "/api" {
		t.Errorf("wsSettings path = %v, want /api (dropped when host key absent)", wss["path"])
	}
}

func TestParse_Hysteria2VerifyPeerCertByName(t *testing.T) {
	res, err := ParseLink("hysteria2://auth@h.com:443?security=tls&sni=decoy.com&vcn=real-cert.com#r")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tls := streamSub(t, res, "tlsSettings")
	if tls["verifyPeerCertByName"] != "real-cert.com" {
		t.Errorf("verifyPeerCertByName = %v, want real-cert.com (vcn param ignored)", tls["verifyPeerCertByName"])
	}
}

func TestParse_TCPHTTPHeader(t *testing.T) {
	res, err := ParseLink("vless://uuid@h.com:443?type=tcp&headerType=http&host=ex.com&path=%2F")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tcp := streamSub(t, res, "tcpSettings")
	header, _ := tcp["header"].(map[string]any)
	if header["type"] != "http" {
		t.Errorf("tcp header type = %v, want http", header["type"])
	}
}

func TestParseVless_CoreFields(t *testing.T) {
	res, err := ParseLink("vless://the-uuid@9.9.9.9:8443?type=tcp&security=none&flow=xtls-rprx-vision#tag1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	st, _ := res.Outbound["settings"].(map[string]any)
	if st["address"] != "9.9.9.9" || st["port"] != 8443 || st["id"] != "the-uuid" || st["flow"] != "xtls-rprx-vision" {
		t.Errorf("vless settings = %#v", st)
	}
}

func TestParseTrojanAndSS_CoreFields(t *testing.T) {
	tr, err := ParseLink("trojan://secret@t.com:443?type=tcp&security=tls#tj")
	if err != nil {
		t.Fatalf("parse trojan: %v", err)
	}
	srv := tr.Outbound["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if srv["address"] != "t.com" || srv["port"] != 443 || srv["password"] != "secret" {
		t.Errorf("trojan server = %#v", srv)
	}

	ssLink := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:sspass")) + "@s.com:8388#ss1"
	ss, err := ParseLink(ssLink)
	if err != nil {
		t.Fatalf("parse ss: %v", err)
	}
	ssrv := ss.Outbound["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if ssrv["address"] != "s.com" || ssrv["port"] != 8388 || ssrv["password"] != "sspass" || ssrv["method"] != "aes-256-gcm" {
		t.Errorf("ss server = %#v", ssrv)
	}
}
