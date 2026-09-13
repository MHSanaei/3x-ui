package sub

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

// The vmess object carries the certificate checks the panel exported, so
// importing that same link has to rebuild them instead of dropping them.
func TestVmessTLSVerifyFieldsSurviveExportImport(t *testing.T) {
	in := &model.Inbound{
		Id: 940002, Listen: "203.0.113.1", Port: 8443, Protocol: model.VMESS,
		Settings: `{"clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"user"}]}`,
		StreamSettings: `{"network":"tcp","security":"tls","tcpSettings":{"header":{"type":"none"}},` +
			`"tlsSettings":{"serverName":"vmess.example.com","alpn":["h2"],"settings":{` +
			`"fingerprint":"chrome","echConfigList":"AEX+DQBB","verifyPeerCertByName":"vcn.example.com",` +
			`"pinnedPeerCertSha256":["AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="]}}}`,
	}
	exported := (&SubService{}).genVmessLink(in, "user")
	if exported == "" {
		t.Fatal("genVmessLink produced nothing")
	}

	parsed, err := link.ParseLink(exported)
	if err != nil {
		t.Fatalf("ParseLink: %v", err)
	}
	raw, err := json.Marshal(parsed.Outbound["streamSettings"])
	if err != nil {
		t.Fatalf("marshal stream: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal(raw, &stream); err != nil {
		t.Fatalf("stream json: %v", err)
	}
	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		t.Fatalf("no tlsSettings: %s", raw)
	}

	// The core reads these joined strings in its config, as applySecurity writes them.
	for field, want := range map[string]string{
		"serverName":           "vmess.example.com",
		"fingerprint":          "chrome",
		"echConfigList":        "AEX+DQBB",
		"verifyPeerCertByName": "vcn.example.com",
		"pinnedPeerCertSha256": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	} {
		if tlsSettings[field] != want {
			t.Errorf("tlsSettings[%q] = %v, want %q", field, tlsSettings[field], want)
		}
	}
	alpn, _ := tlsSettings["alpn"].([]any)
	if len(alpn) != 1 || alpn[0] != "h2" {
		t.Errorf("alpn = %v, want [h2]", tlsSettings["alpn"])
	}
}
