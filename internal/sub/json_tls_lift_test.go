package sub

import "testing"

func TestLiftJSONTLSClientSettings_HostOverrides(t *testing.T) {
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":  "inbound.example.com",
			"fingerprint": "chrome",
		},
	}
	ep := map[string]any{
		"dest":                 "proxy.example.com",
		"echConfigList":        "cloudflare-ech.com+udp://1.1.1.1",
		"verifyPeerCertByName": "cert.example.com",
		"pinnedPeerCertSha256": []any{"aa", "bb"},
		"allowInsecure":        true,
	}
	applyExternalProxyTLSToStream(ep, stream, "tls")
	liftJSONTLSClientSettings(stream)

	tls := stream["tlsSettings"].(map[string]any)
	if _, ok := tls["settings"]; ok {
		t.Fatalf("tlsSettings.settings should be removed, got %v", tls["settings"])
	}
	want := map[string]any{
		"echConfigList":        "cloudflare-ech.com+udp://1.1.1.1",
		"verifyPeerCertByName": "cert.example.com",
		"pinnedPeerCertSha256": "aa,bb",
		"allowInsecure":        true,
		"fingerprint":          "chrome",
	}
	for k, v := range want {
		if tls[k] != v {
			t.Errorf("tlsSettings[%q] = %v, want %v", k, tls[k], v)
		}
	}
}

func TestLiftJSONTLSClientSettings_NoSettings(t *testing.T) {
	stream := map[string]any{"security": "tls", "tlsSettings": map[string]any{"serverName": "a"}}
	liftJSONTLSClientSettings(stream)
	if got := stream["tlsSettings"].(map[string]any)["serverName"]; got != "a" {
		t.Fatalf("serverName = %v", got)
	}
	liftJSONTLSClientSettings(map[string]any{"security": "none"})
}
