package sub

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Characterization snapshots (Phase 0 of the Hosts feature). These lock the
// CURRENT subscription-link output for the externalProxy paths so the Phase-1
// ShareEndpoint refactor can be proven behavior-preserving: they must pass on
// unchanged code and stay green, unedited, through every later phase. Assertions
// are exact (==) where output is deterministic and Contains where a value is
// randomized (reality spx) or hex-derived.

const charClient = `{"id":"11111111-2222-4333-8444-555555555555","email":"user"}`

// charVlessInbound builds a VLESS inbound with one client "user".
func charVlessInbound(stream string) *model.Inbound {
	return &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.VLESS,
		Remark:         "char",
		Settings:       `{"clients":[` + charClient + `],"decryption":"none","encryption":"none"}`,
		StreamSettings: stream,
	}
}

// C1 — VLESS, TLS base, 2 externalProxy entries (forceTls tls + none). Locks
// buildExternalProxyURLLinks, applyExternalProxyTLSParams, the none-strip path,
// per-entry ordering, and the "\n" join.
func TestChar_C1_VlessExternalProxy(t *testing.T) {
	stream := `{
		"network":"tcp","security":"tls",
		"tcpSettings":{"header":{"type":"none"}},
		"tlsSettings":{"serverName":"base.sni","alpn":["h2"],"settings":{"fingerprint":"chrome"}},
		"externalProxy":[
			{"forceTls":"tls","dest":"cdn1.example.com","port":8443,"remark":"R1","sni":"sni1.example.com","fingerprint":"firefox","alpn":["h3","h2"],"pinnedPeerCertSha256":["UElO"]},
			{"forceTls":"none","dest":"cdn2.example.com","port":80,"remark":"R2"}
		]
	}`
	s := &SubService{}
	got := s.genVlessLink(charVlessInbound(stream), "user")
	want := "vless://11111111-2222-4333-8444-555555555555@cdn1.example.com:8443?alpn=h3%2Ch2&encryption=none&fp=firefox&pcs=UElO&security=tls&sni=sni1.example.com&type=tcp#char-R1\n" +
		"vless://11111111-2222-4333-8444-555555555555@cdn2.example.com:80?encryption=none&security=none&type=tcp#char-R2"
	if got != want {
		t.Fatalf("C1 mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// C4 — VLESS reality base + 1 externalProxy with forceTls "same". Locks the
// "same keeps the base security (reality)" passthrough. spx is randomized so the
// fixed fields are asserted by Contains.
func TestChar_C4_VlessRealitySame(t *testing.T) {
	stream := `{
		"network":"tcp","security":"reality",
		"tcpSettings":{"header":{"type":"none"}},
		"realitySettings":{"serverNames":["reality.example.com"],"shortIds":["ab12cd"],"settings":{"publicKey":"PBKvalue","fingerprint":"firefox"}},
		"externalProxy":[{"forceTls":"same","dest":"cdn.example.com","port":2053,"remark":"RS"}]
	}`
	s := &SubService{}
	got := s.genVlessLink(charVlessInbound(stream), "user")
	wants := []string{
		"vless://11111111-2222-4333-8444-555555555555@cdn.example.com:2053",
		"security=reality",
		"sni=reality.example.com",
		"pbk=PBKvalue",
		"sid=ab12cd",
		"fp=firefox",
		"#char-RS",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("C4 missing %q\n got: %s", w, got)
		}
	}
	if strings.Count(got, "\n") != 0 {
		t.Fatalf("C4 expected a single link, got: %s", got)
	}
}

// C2 — VMess, TLS base, 2 externalProxy entries (forceTls same + none). Locks
// buildVmessExternalProxyLinks, cloneVmessShareObj strip, the obj["tls"] rewrite,
// and the int port. Asserts on the decoded JSON objects.
func TestChar_C2_VmessExternalProxy(t *testing.T) {
	stream := `{
		"network":"tcp","security":"tls",
		"tcpSettings":{"header":{"type":"none"}},
		"tlsSettings":{"serverName":"base.sni","alpn":["h2"],"settings":{"fingerprint":"chrome"}},
		"externalProxy":[
			{"forceTls":"same","dest":"vm1.example.com","port":8443,"remark":"V1","sni":"sni1.example.com"},
			{"forceTls":"none","dest":"vm2.example.com","port":80,"remark":"V2"}
		]
	}`
	in := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.VMESS,
		Remark:         "char",
		Settings:       `{"clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"user","security":"auto"}]}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	got := s.genVmessLink(in, "user")
	want := "vmess://ewogICJhZGQiOiAidm0xLmV4YW1wbGUuY29tIiwKICAiYWxwbiI6ICJoMiIsCiAgImZwIjogImNocm9tZSIsCiAgImlkIjogIjExMTExMTExLTIyMjItNDMzMy04NDQ0LTU1NTU1NTU1NTU1NSIsCiAgIm5ldCI6ICJ0Y3AiLAogICJwb3J0IjogODQ0MywKICAicHMiOiAiY2hhci1WMSIsCiAgInNjeSI6ICJhdXRvIiwKICAic25pIjogInNuaTEuZXhhbXBsZS5jb20iLAogICJ0bHMiOiAidGxzIiwKICAidHlwZSI6ICJub25lIiwKICAidiI6ICIyIgp9\n" +
		"vmess://ewogICJhZGQiOiAidm0yLmV4YW1wbGUuY29tIiwKICAiaWQiOiAiMTExMTExMTEtMjIyMi00MzMzLTg0NDQtNTU1NTU1NTU1NTU1IiwKICAibmV0IjogInRjcCIsCiAgInBvcnQiOiA4MCwKICAicHMiOiAiY2hhci1WMiIsCiAgInNjeSI6ICJhdXRvIiwKICAidGxzIjogIm5vbmUiLAogICJ0eXBlIjogIm5vbmUiLAogICJ2IjogIjIiCn0="
	if got != want {
		t.Fatalf("C2 mismatch.\n got: %q\nwant: %q", got, want)
	}
	// Sanity: decode both objects so a structural change is visible too.
	for i, part := range strings.Split(got, "\n") {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(part, "vmess://"))
		if err != nil {
			t.Fatalf("C2 link %d not base64: %v", i, err)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("C2 link %d not json: %v", i, err)
		}
	}
}

// C3a — Trojan, TLS base, 1 externalProxy entry. Locks userinfo encoding through
// the shared builder.
func TestChar_C3_TrojanExternalProxy(t *testing.T) {
	stream := `{
		"network":"tcp","security":"tls",
		"tcpSettings":{"header":{"type":"none"}},
		"tlsSettings":{"serverName":"base.sni","settings":{"fingerprint":"chrome"}},
		"externalProxy":[{"forceTls":"tls","dest":"tj.example.com","port":8443,"remark":"TJ","sni":"tj.sni"}]
	}`
	in := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.Trojan,
		Remark:         "char",
		Settings:       `{"clients":[{"password":"p@ss/w+rd=","email":"user"}]}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	got := s.genTrojanLink(in, "user")
	want := "trojan://p%40ss%2Fw%2Brd%3D@tj.example.com:8443?fp=chrome&security=tls&sni=tj.sni&type=tcp#char-TJ"
	if got != want {
		t.Fatalf("C3-Trojan mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// C3b — Shadowsocks 2022 (method[0]=='2'), TLS base, 1 externalProxy entry.
// Locks the ss-2022 triple-segment userinfo path through the shared builder.
func TestChar_C3_ShadowsocksExternalProxy(t *testing.T) {
	stream := `{
		"network":"tcp","security":"tls",
		"tcpSettings":{"header":{"type":"none"}},
		"tlsSettings":{"serverName":"base.sni","settings":{"fingerprint":"chrome"}},
		"externalProxy":[{"forceTls":"tls","dest":"ss.example.com","port":8443,"remark":"SS","sni":"ss.sni"}]
	}`
	in := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.Shadowsocks,
		Remark:         "char",
		Settings:       `{"method":"2022-blake3-aes-256-gcm","password":"inboundpw","clients":[{"password":"clientpw","email":"user"}]}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	got := s.genShadowsocksLink(in, "user")
	want := "ss://2022-blake3-aes-256-gcm:inboundpw:clientpw@ss.example.com:8443?fp=chrome&security=tls&sni=ss.sni&type=tcp#char-SS"
	if got != want {
		t.Fatalf("C3-SS mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// A TCP http header on Shadowsocks must be emitted as a SIP002 obfs-local
// plugin (what v2rayN parses), not the xray-native type/headerType/host/path
// params (which SIP002 clients silently ignore).
func TestShadowsocksTcpHttpHeaderUsesObfsLocalPlugin(t *testing.T) {
	stream := `{
		"network":"tcp","security":"none",
		"tcpSettings":{"header":{"type":"http","request":{"path":["/"],"headers":{"Host":["test"]}}}}
	}`
	in := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           38143,
		Protocol:       model.Shadowsocks,
		Remark:         "ss",
		Settings:       `{"method":"2022-blake3-aes-256-gcm","password":"inboundpw","clients":[{"password":"clientpw","email":"user"}]}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	got := s.genShadowsocksLink(in, "user")
	if !strings.Contains(got, "plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dtest") {
		t.Fatalf("expected obfs-local plugin param, got: %q", got)
	}
	for _, leak := range []string{"headerType=", "type=tcp", "host=test", "path="} {
		if strings.Contains(got, leak) {
			t.Fatalf("xray-native param %q must not leak into SS link: %q", leak, got)
		}
	}
}

// C6 — Hysteria2, TLS, 1 externalProxy entry with a cert pin. Guards that the
// Hysteria generator stays on its own path (hex pinSHA256, not pcs) and is NOT
// folded into the unified builder. Pin hex is derived, so Contains is used.
func TestChar_C6_HysteriaExternalProxy(t *testing.T) {
	// base64 of 32 zero bytes -> a valid pin shape for hysteriaPinHex.
	pin := base64.StdEncoding.EncodeToString(make([]byte, 32))
	stream := `{
		"security":"tls",
		"tlsSettings":{"serverName":"hy.sni","alpn":["h3"],"settings":{"fingerprint":"chrome"}},
		"externalProxy":[{"forceTls":"same","dest":"hop.example.com","port":9443,"remark":"H1","pinnedPeerCertSha256":["` + pin + `"]}]
	}`
	in := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.Hysteria,
		Remark:         "char",
		Settings:       `{"version":2,"clients":[{"auth":"hyauth","email":"user"}]}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	got := s.genHysteriaLink(in, "user")
	wants := []string{
		"hysteria2://hyauth@hop.example.com:9443",
		"security=tls",
		"sni=hy.sni",
		"pinSHA256=",
		"#char-H1",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("C6 missing %q\n got: %s", w, got)
		}
	}
	if strings.Contains(got, "pcs=") {
		t.Fatalf("C6 hysteria must not use pcs=, got: %s", got)
	}
}
