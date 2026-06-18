package sub

import (
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

// TestGenVlessLink_RealityParamsMapped locks the reality field mapping
// (applyShareRealityParams, service.go:1147). serverNames/shortIds are single-element
// so random.Num is deterministic (index 0); spx is random so it is asserted by prefix.
// Distinct pbk/sid values catch a pbk<->sid swap mutant.
func TestGenVlessLink_RealityParamsMapped(t *testing.T) {
	stream := `{
		"network":"tcp","security":"reality",
		"tcpSettings":{"header":{"type":"none"}},
		"realitySettings":{
			"serverNames":["reality.example.com"],
			"shortIds":["ab12cd"],
			"settings":{"publicKey":"PBKvalue","fingerprint":"firefox"}
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
		"spx=%2F", // "/" + random.Seq(15), percent-encoded leading slash
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
