package xray

import (
	"fmt"
	"testing"
)

func TestValidateOutboundConfig_MultipleNordUserspaceWireGuard(t *testing.T) {
	secretKey := testWGKey(11)
	tests := []struct {
		name      string
		tag       string
		publicKey string
		endpoint  string
	}{
		{name: "first", tag: "nord-us1.nordvpn.com", publicKey: testWGKey(12), endpoint: "198.51.100.10:51820"},
		{name: "second", tag: "nord-us2.nordvpn.com", publicKey: testWGKey(13), endpoint: "198.51.100.20:51820"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound := fmt.Sprintf(`{
				"tag": %q,
				"protocol": "wireguard",
				"settings": {
					"secretKey": %q,
					"address": ["10.5.0.2/32"],
					"peers": [{"publicKey": %q, "endpoint": %q}],
					"noKernelTun": true
				}
			}`, tt.tag, secretKey, tt.publicKey, tt.endpoint)
			if err := ValidateOutboundConfig([]byte(outbound)); err != nil {
				t.Fatalf("xray-core rejected NordVPN outbound %q: %v", tt.tag, err)
			}
		})
	}
}
