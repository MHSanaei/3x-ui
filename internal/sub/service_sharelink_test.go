package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// shareLinkInbound builds a VLESS inbound with one client and the given stream
// settings, mirroring flowTestInbound but without forcing a flow.
func shareLinkInbound(streamSettings string) *model.Inbound {
	return &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.VLESS,
		Remark:         "sharelink",
		Settings:       `{"clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"user"}],"decryption":"none","encryption":"none"}`,
		StreamSettings: streamSettings,
	}
}

// TestGenVlessLink_TLSParamsMapped locks every field that applyShareTLSParams
// (service.go:1029) writes into a TLS share link. Without these assertions a mutant
// that drops `sni`, swaps a key, or skips `pcs`/`alpn`/`fp` survives the whole suite —
// the existing flow tests only check `flow=`.
func TestGenVlessLink_TLSParamsMapped(t *testing.T) {
	stream := `{
		"network":"tcp","security":"tls",
		"tcpSettings":{"header":{"type":"none"}},
		"tlsSettings":{
			"serverName":"sni.example.com",
			"alpn":["h2","http/1.1"],
			"settings":{"fingerprint":"chrome","pinnedPeerCertSha256":["YWJj"]}
		}
	}`
	s := &SubService{}
	link := s.genVlessLink(shareLinkInbound(stream), "user")

	// url.Values.Encode() percent-encodes values: "," -> %2C, "/" -> %2F.
	wants := []string{
		"security=tls",
		"sni=sni.example.com",
		"fp=chrome",
		"alpn=h2%2Chttp%2F1.1",
		"pcs=YWJj",
	}
	for _, w := range wants {
		if !strings.Contains(link, w) {
			t.Fatalf("TLS link missing %q\n got: %s", w, link)
		}
	}
}

// Locks the reality field mapping of applyShareRealityParams; distinct pbk/sid
// catch a swap mutant. spx is now a per-client derived value (#5718 / follow-up).
func TestGenVlessLink_RealityParamsMapped(t *testing.T) {
	stream := `{
		"network":"tcp","security":"reality",
		"tcpSettings":{"header":{"type":"none"}},
		"realitySettings":{
			"serverNames":["reality.example.com"],
			"shortIds":["ab12cd"],
			"settings":{"publicKey":"PBKvalue","fingerprint":"firefox","spiderX":"/mypath"}
		}
	}`
	s := &SubService{}
	link := s.genVlessLink(shareLinkInbound(stream), "user")

	wants := []string{
		"security=reality",
		"sni=reality.example.com",
		"pbk=PBKvalue",
		"sid=ab12cd",
		"fp=firefox",
		"spx=%2F",
	}
	for _, w := range wants {
		if !strings.Contains(link, w) {
			t.Fatalf("reality link missing %q\n got: %s", w, link)
		}
	}
	// A pbk<->sid swap must not silently pass: pbk must not carry the shortId.
	if strings.Contains(link, "pbk=ab12cd") || strings.Contains(link, "sid=PBKvalue") {
		t.Fatalf("reality pbk/sid mapping crossed: %s", link)
	}
}

// realityTwoClientInbound builds a reality VLESS inbound carrying two clients
// with distinct subIds so the per-client spx derivation can be exercised.
func realityTwoClientInbound() *model.Inbound {
	return &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "sharelink",
		Settings: `{"clients":[
			{"id":"11111111-2222-4333-8444-555555555555","email":"alice","subId":"subAlice"},
			{"id":"22222222-3333-4444-8555-666666666666","email":"bob","subId":"subBob"}
		],"decryption":"none","encryption":"none"}`,
		StreamSettings: `{
			"network":"tcp","security":"reality",
			"tcpSettings":{"header":{"type":"none"}},
			"realitySettings":{
				"serverNames":["reality.example.com"],
				"shortIds":["ab12cd"],
				"settings":{"publicKey":"PBKvalue","fingerprint":"firefox","spiderX":"/seed"}
			}
		}`,
	}
}

func spxParam(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link %q: %v", link, err)
	}
	spx := u.Query().Get("spx")
	if spx == "" || spx[0] != '/' {
		t.Fatalf("spx missing or not /-prefixed in %q", link)
	}
	return spx
}

// spx must be stable for a given client across repeated exports (the #5718
// complaint) yet differ between clients so the value can't be fingerprinted.
func TestGenVlessLink_RealitySpiderXPerClientStable(t *testing.T) {
	s := &SubService{}
	inbound := realityTwoClientInbound()

	aliceFirst := spxParam(t, s.genVlessLink(inbound, "alice"))
	aliceSecond := spxParam(t, s.genVlessLink(inbound, "alice"))
	bob := spxParam(t, s.genVlessLink(inbound, "bob"))

	if aliceFirst != aliceSecond {
		t.Fatalf("spx not stable for the same client: %q vs %q", aliceFirst, aliceSecond)
	}
	if aliceFirst == bob {
		t.Fatalf("spx identical across clients (fingerprintable): %q", aliceFirst)
	}
}

func TestDeriveSpiderX(t *testing.T) {
	if got := deriveSpiderX("seed", "clientA"); got != deriveSpiderX("seed", "clientA") {
		t.Fatalf("deriveSpiderX not deterministic: %q", got)
	}
	if deriveSpiderX("seed", "clientA") == deriveSpiderX("seed", "clientB") {
		t.Fatal("deriveSpiderX must differ per client")
	}
	if deriveSpiderX("seedA", "clientA") == deriveSpiderX("seedB", "clientA") {
		t.Fatal("rotating the seed must rotate a client's spx")
	}
	got := deriveSpiderX("seed", "clientA")
	if len(got) != 16 || got[0] != '/' {
		t.Fatalf("deriveSpiderX shape = %q, want /-prefixed 15-char path", got)
	}
	if fallback := deriveSpiderX("", ""); len(fallback) != 16 || fallback[0] != '/' {
		t.Fatalf("empty-input fallback = %q, want /-prefixed path", fallback)
	}
}

// Cross-language vectors shared with frontend/src/test/spider-x.test.ts: the
// panel builds these links in TS, so both derivations must agree byte-for-byte.
func TestDeriveSpiderXMatchesFrontendVectors(t *testing.T) {
	vectors := map[string]struct{ seed, clientKey, want string }{
		"seed and subId": {"/seed", "subAlice", "/c252fbc3ecd3e3c"},
		"seed only":      {"/", "", "/d08ed99bd9afc60"},
	}
	for name, v := range vectors {
		t.Run(name, func(t *testing.T) {
			if got := deriveSpiderX(v.seed, v.clientKey); got != v.want {
				t.Fatalf("deriveSpiderX(%q, %q) = %q, want %q (must match frontend/src/lib/xray/spider-x.ts)", v.seed, v.clientKey, got, v.want)
			}
		})
	}
}
