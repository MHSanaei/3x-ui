package sub

import (
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestTuicTLSOptionsReachLinksAndClash(t *testing.T) {
	for _, tc := range []struct {
		name, settings, zeroRTT string
		disableSNI              bool
	}{
		{"defaults", `"server":{"sni":"cert.example"}`, "1", false},
		{"nested", `"server":{"sni":"cert.example","disable_sni":true,"zero_rtt_handshake":false}`, "0", true},
		{"legacy", `"sni":"cert.example","disable_sni":true,"zero_rtt_handshake":false`, "0", true},
		{"nested false overrides legacy", `"sni":"cert.example","disable_sni":true,"zero_rtt_handshake":false,"server":{"disable_sni":false,"zero_rtt_handshake":true}`, "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Protocol: model.TUIC, Port: 8443, Listen: "198.51.100.1",
				Settings:       `{` + tc.settings + `,"clients":[{"uuid":"11111111-1111-1111-1111-111111111111","password":"secret","email":"user@test"}]}`,
				StreamSettings: `{"externalProxy":[{"dest":"edge.example","port":9443,"sni":"verify.example"}]}`,
			}
			svc := &SubService{}
			link, err := url.Parse(svc.genTuicLink(inbound, "user@test"))
			if err != nil {
				t.Fatal(err)
			}
			query := link.Query()
			if query.Get("zero_rtt_handshake") != tc.zeroRTT || (query.Get("disable_sni") == "1") != tc.disableSNI {
				t.Fatalf("client TLS options lost in link: %s", link)
			}
			if !tc.disableSNI && query.Has("disable_sni") {
				t.Fatalf("disabled option should remain absent for existing clients: %s", link)
			}
			if query.Get("sni") != "verify.example" || query.Get("allow_insecure") != "0" {
				t.Fatalf("SNI suppression changed certificate verification settings: %s", link)
			}
			clash := (&SubClashService{SubService: svc}).buildTuicProxy(svc, inbound, model.Client{Email: "user@test"}, map[string]any{"sni": "verify.example"})
			if clash["reduce-rtt"] != (tc.zeroRTT == "1") || (clash["disable-sni"] == true) != tc.disableSNI || clash["sni"] != "verify.example" {
				t.Fatalf("Clash lost client TLS options: %+v", clash)
			}
			if _, insecure := clash["skip-cert-verify"]; insecure {
				t.Fatal("disabling SNI must not disable certificate verification")
			}
			if tc.disableSNI && clash["name-cert-verify"] != "verify.example" {
				t.Fatalf("Mihomo needs explicit certificate verification when SNI is disabled: %+v", clash)
			}
		})
	}
}

func TestTuicClashCertificateNameFollowsEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint map[string]any
		wantName string
	}{
		{"inbound address", nil, "198.51.100.1"},
		{"host address", map[string]any{"dest": "edge.example"}, "edge.example"},
		{"host certificate", map[string]any{"dest": "edge.example", "sni": "verify.example"}, "verify.example"},
		{"explicit insecure host", map[string]any{"dest": "edge.example", "allowInsecure": true}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Protocol: model.TUIC, Port: 8443, Listen: "198.51.100.1",
				Settings: `{"server":{"disable_sni":true},"clients":[{"uuid":"11111111-1111-1111-1111-111111111111","password":"secret","email":"user@test"}]}`,
			}
			svc := &SubService{}
			proxy := (&SubClashService{SubService: svc}).buildTuicProxy(svc, inbound, model.Client{Email: "user@test"}, tc.endpoint)
			if tc.wantName == "" {
				if _, exists := proxy["name-cert-verify"]; exists || proxy["skip-cert-verify"] != true {
					t.Fatalf("explicit Host verification policy changed: %+v", proxy)
				}
			} else if proxy["name-cert-verify"] != tc.wantName {
				t.Fatalf("certificate name = %v, want %s", proxy["name-cert-verify"], tc.wantName)
			}
		})
	}
}
