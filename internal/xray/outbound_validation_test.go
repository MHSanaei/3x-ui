package xray

import (
	"strings"
	"testing"
)

// TestValidateOutboundConfig_RejectsUnencryptedPublicVless covers xray-core
// v26.7.11's refusal to build an unencrypted vless outbound to a public
// address — the check now runs in-process, so the panel can surface it before
// a config reaches the core and bricks startup. A private-address outbound and
// a TLS outbound stay valid.
func TestValidateOutboundConfig_RejectsUnencryptedPublicVless(t *testing.T) {
	publicPlaintext := `{
		"protocol": "vless",
		"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "none"}
	}`
	if err := ValidateOutboundConfig([]byte(publicPlaintext)); err == nil {
		t.Fatal("expected a public unencrypted vless outbound to be rejected")
	} else if !strings.Contains(err.Error(), "prohibited") {
		t.Fatalf("expected a prohibition error, got: %v", err)
	}

	privatePlaintext := `{
		"protocol": "vless",
		"settings": {"address": "10.0.0.1", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "none"}
	}`
	if err := ValidateOutboundConfig([]byte(privatePlaintext)); err != nil {
		t.Fatalf("a private-address plaintext vless outbound must stay valid, got: %v", err)
	}

	publicTLS := `{
		"protocol": "vless",
		"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "tls", "tlsSettings": {"serverName": "example.com"}}
	}`
	if err := ValidateOutboundConfig([]byte(publicTLS)); err != nil {
		t.Fatalf("a TLS-secured public vless outbound must stay valid, got: %v", err)
	}
}
