package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// N1 — externalProxyToEndpoint maps the scalar fields and carries the source
// entry so delegated TLS application reproduces the legacy presence-tracked
// overrides (absent key never clobbers an upstream value).
func TestExternalProxyToEndpoint(t *testing.T) {
	ep := map[string]any{
		"forceTls": "tls",
		"dest":     "cdn.example.com",
		"port":     float64(8443),
		"remark":   "R",
		"sni":      "s.example.com",
	}
	e := externalProxyToEndpoint(ep)
	if e.Address != "cdn.example.com" {
		t.Fatalf("Address = %q, want cdn.example.com", e.Address)
	}
	if e.Port != 8443 {
		t.Fatalf("Port = %d, want 8443", e.Port)
	}
	if e.ForceTls != "tls" {
		t.Fatalf("ForceTls = %q, want tls", e.ForceTls)
	}
	if e.Remark != "R" {
		t.Fatalf("Remark = %q, want R", e.Remark)
	}
	if e.ep == nil {
		t.Fatalf("ep not carried; delegated TLS application would lose the source entry")
	}
	// Delegation preserves the sni override and does not invent absent fields.
	params := map[string]string{}
	applyEndpointTLSParams(e, params, "tls")
	if params["sni"] != "s.example.com" {
		t.Fatalf("delegated sni = %q, want s.example.com", params["sni"])
	}
	if _, ok := params["fp"]; ok {
		t.Fatalf("absent fingerprint must not be set, got fp=%q", params["fp"])
	}
}

// N2 — inboundDefaultEndpoint reproduces the no-externalProxy default: resolved
// address + inbound port, forceTls "same", empty remark, no source entry.
func TestInboundDefaultEndpoint(t *testing.T) {
	in := &model.Inbound{Listen: "198.51.100.7", Port: 8080}
	s := &SubService{}
	e := s.inboundDefaultEndpoint(in)
	if e.Address != "198.51.100.7" {
		t.Fatalf("Address = %q, want 198.51.100.7", e.Address)
	}
	if e.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", e.Port)
	}
	if e.ForceTls != "same" {
		t.Fatalf("ForceTls = %q, want same", e.ForceTls)
	}
	if e.Remark != "" {
		t.Fatalf("Remark = %q, want empty", e.Remark)
	}
	if e.ep != nil {
		t.Fatalf("default endpoint must not carry a source externalProxy entry")
	}
}

// N3 — buildEndpointLinks renders the param-form path: one link per endpoint,
// TLS override applied for tls, fields stripped + security overridden for none,
// joined by "\n", in order.
func TestBuildEndpointLinks_ParamForm(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	params := map[string]string{"type": "tcp", "security": "tls", "sni": "base.sni", "fp": "chrome"}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "tls", "dest": "a.example.com", "port": float64(8443), "remark": "A", "sni": "a.sni"}),
		externalProxyToEndpoint(map[string]any{"forceTls": "none", "dest": "b.example.com", "port": float64(80), "remark": "B"}),
	}
	got := s.buildEndpointLinks(eps, params, "tls",
		func(e ShareEndpoint) string { return fmt.Sprintf("vless://uid@%s", joinHostPort(e.Address, e.Port)) },
		func(e ShareEndpoint) string { return s.genRemark(in, "user", e.Remark, "") },
	)
	want := "vless://uid@a.example.com:8443?fp=chrome&security=tls&sni=a.sni&type=tcp#ib-A-user\n" +
		"vless://uid@b.example.com:80?security=none&type=tcp#ib-B-user"
	if got != want {
		t.Fatalf("N3 mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// N4 — buildEndpointVmessLinks renders the object-form path: base obj cloned per
// endpoint, add/port/tls rewritten, sni override applied, none-strip honored.
func TestBuildEndpointVmessLinks(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	baseObj := map[string]any{
		"v": "2", "add": "base.example.com", "port": 443, "type": "none",
		"id": "uid", "scy": "auto", "net": "tcp",
		"tls": "tls", "sni": "base.sni", "alpn": "h2", "fp": "chrome",
	}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "same", "dest": "a.example.com", "port": float64(8443), "remark": "A", "sni": "a.sni"}),
		externalProxyToEndpoint(map[string]any{"forceTls": "none", "dest": "b.example.com", "port": float64(80), "remark": "B"}),
	}
	got := s.buildEndpointVmessLinks(eps, baseObj, in, "user", "tcp")
	want := "vmess://ewogICJhZGQiOiAiYS5leGFtcGxlLmNvbSIsCiAgImFscG4iOiAiaDIiLAogICJmcCI6ICJjaHJvbWUiLAogICJpZCI6ICJ1aWQiLAogICJuZXQiOiAidGNwIiwKICAicG9ydCI6IDg0NDMsCiAgInBzIjogImliLUEtdXNlciIsCiAgInNjeSI6ICJhdXRvIiwKICAic25pIjogImEuc25pIiwKICAidGxzIjogInRscyIsCiAgInR5cGUiOiAibm9uZSIsCiAgInYiOiAiMiIKfQ==\n" +
		"vmess://ewogICJhZGQiOiAiYi5leGFtcGxlLmNvbSIsCiAgImlkIjogInVpZCIsCiAgIm5ldCI6ICJ0Y3AiLAogICJwb3J0IjogODAsCiAgInBzIjogImliLUItdXNlciIsCiAgInNjeSI6ICJhdXRvIiwKICAidGxzIjogIm5vbmUiLAogICJ0eXBlIjogIm5vbmUiLAogICJ2IjogIjIiCn0="
	if got != want {
		t.Fatalf("N4 mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// happ.su documents serverDescription for VMess as a JSON object key
// {"add":"...","ps":"...","serverDescription":"Happ the best"}.
func TestBuildEndpointVmessLinks_HostServerDescription(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	baseObj := map[string]any{"v": "2", "add": "base.example.com", "port": 443, "type": "none", "id": "uid", "scy": "auto", "net": "tcp", "tls": "none"}
	host := &model.Host{Address: "a.example.com", Port: 8443, ServerDescription: "Berlin premium"}
	eps := []ShareEndpoint{externalProxyToEndpoint(hostToExternalProxyMap(host, "a.example.com", 8443))}

	got := s.buildEndpointVmessLinks(eps, baseObj, in, "user", "tcp")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess link: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal vmess object: %v", err)
	}
	if obj["add"] != "a.example.com" {
		t.Fatalf("host endpoint not applied: add = %v", obj["add"])
	}
	if obj["serverDescription"] != "Berlin premium" {
		t.Fatalf("VMess object missing serverDescription: got %v, want Berlin premium", obj["serverDescription"])
	}
}

// N5 — a host's Final Mask is appended to the inbound's own fm param (#5831).
func TestBuildEndpointLinks_HostFinalMaskMerge(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	params := map[string]string{"type": "tcp", "security": "tls", "fm": `{"tcp":[{"type":"sudoku"}]}`}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "same", "dest": "a.example.com", "port": float64(8443), "remark": "A", "isHost": true, "finalMask": `{"tcp":[{"type":"fragment"}]}`}),
	}
	got := s.buildEndpointLinks(eps, params, "tls",
		func(e ShareEndpoint) string { return fmt.Sprintf("vless://uid@%s", joinHostPort(e.Address, e.Port)) },
		func(e ShareEndpoint) string { return s.genRemark(in, "user", e.Remark, "") },
	)
	wantFm := "fm=" + url.QueryEscape(`{"tcp":[{"type":"sudoku"},{"type":"fragment"}]}`)
	if !strings.Contains(got, wantFm) {
		t.Fatalf("host finalMask not merged into the fm param.\n got: %q\nwant substring: %q", got, wantFm)
	}
}

// N6 — same for the VMess object form: the host mask lands in obj["fm"].
func TestBuildEndpointVmessLinks_HostFinalMask(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	baseObj := map[string]any{"v": "2", "add": "base.example.com", "port": 443, "type": "none", "id": "uid", "scy": "auto", "net": "tcp", "tls": "tls"}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "same", "dest": "a.example.com", "port": float64(8443), "remark": "A", "isHost": true, "finalMask": `{"udp":[{"type":"salamander"}]}`}),
	}
	got := s.buildEndpointVmessLinks(eps, baseObj, in, "user", "tcp")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess link: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal vmess obj: %v", err)
	}
	if fm, _ := obj["fm"].(string); fm != `{"udp":[{"type":"salamander"}]}` {
		t.Fatalf("vmess fm = %q, want the host mask", obj["fm"])
	}
}

// An endpoint address is mirrored from the node as reported: whatever it holds, one
// endpoint stays one line of the subscription body, inside its own link's authority.
func TestSub_EndpointAddressCannotAddLines(t *testing.T) {
	seedSubDB(t)
	dest, _ := json.Marshal("::1]:443#a\nincy://routing/onadd/e30=\r\nvless://x@[::2")
	seedSubInbound(t, "s1", "n", 4490, 1, fmt.Sprintf(
		`{"network":"tcp","security":"none","externalProxy":[{"forceTls":"same","dest":%s,"port":443,"remark":""}]}`, dest))
	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	want := "vless://11111111-2222-4333-8444-000000004490@" +
		"[::1%5D:443%23a%0Aincy:%2F%2Frouting%2Fonadd%2Fe30=%0D%0Avless:%2F%2Fx%40%5B::2]:443" +
		"?encryption=none&security=none&type=tcp#n-n%40e"
	if len(links) != 1 || links[0] != want {
		t.Fatalf("links = %q, want [%q]", links, want)
	}
}

func TestJoinHostPortEscapesNonHostCharacters(t *testing.T) {
	for host, want := range map[string]string{
		"cdn.example.com": "cdn.example.com:443",
		"203.0.113.5":     "203.0.113.5:443",
		"[2001:db8::1]":   "[2001:db8::1]:443",
		"fe80::1%eth0":    "[fe80::1%eth0]:443",
		"пример.рф":       "пример.рф:443",
		"a b\tc":          "a%20b%09c:443",
		"a\u0085b\u2028c": "a%C2%85b%E2%80%A8c:443",
		"h?x=1#f":         "h%3Fx=1%23f:443",
		"u@h/p":           "u%40h%2Fp:443",
	} {
		if got := joinHostPort(host, 443); got != want {
			t.Errorf("joinHostPort(%q) = %q, want %q", host, got, want)
		}
	}
}

// The VLESS id is the userinfo of its link and is escaped like every other protocol's.
func TestGenVlessLinkEscapesID(t *testing.T) {
	in := &model.Inbound{
		Listen: "203.0.113.5", Port: 443, Protocol: model.VLESS, Remark: "n",
		Settings:       `{"clients":[{"id":"a@b#c\nd","email":"user"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	got := NewSubService("").genVlessLink(in, "user")
	if !strings.HasPrefix(got, "vless://a%40b%23c%0Ad@203.0.113.5:443?") {
		t.Fatalf("vless id not escaped: %q", got)
	}
}

// The panel builds its own link and QR through the browser's URL parser, which escapes
// userinfo with the WHATWG userinfo set; a subscription escaping more serves another id.
func TestGenVlessLinkEscapesIDLikeThePanel(t *testing.T) {
	for id, want := range map[string]string{
		"team+1":     "team+1",
		"a=b":        "a%3Db",
		"x:y;z":      "x%3Ay%3Bz",
		"ok!$,'()*~": "ok!$,'()*~",
		"é":          "%C3%A9",
	} {
		in := &model.Inbound{
			Listen: "203.0.113.5", Port: 443, Protocol: model.VLESS, Remark: "n",
			Settings:       `{"clients":[{"id":` + strconv.Quote(id) + `,"email":"user"}],"decryption":"none"}`,
			StreamSettings: `{"network":"tcp","security":"none"}`,
		}
		got := NewSubService("").genVlessLink(in, "user")
		if !strings.HasPrefix(got, "vless://"+want+"@203.0.113.5:443?") {
			t.Errorf("id %q: link %q, want userinfo %q", id, got, want)
		}
	}
}
