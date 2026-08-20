package xray

import (
	"encoding/base64"
	"testing"
)

func testWGKey(seed byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestValidateOutboundConfig_PiaUserspaceWireGuard(t *testing.T) {
	piaOutbound := `{
		"tag": "pia-us-east-useast1",
		"piaHostname": "useast1",
		"protocol": "wireguard",
		"settings": {
			"secretKey": "` + testWGKey(1) + `",
			"address": ["10.0.0.2/32"],
			"mtu": 1420,
			"noKernelTun": true,
			"peers": [{
				"publicKey": "` + testWGKey(2) + `",
				"endpoint": "198.51.100.10:51820",
				"allowedIPs": ["0.0.0.0/0"],
				"keepAlive": 25
			}]
		}
	}`
	if err := ValidateOutboundConfig([]byte(piaOutbound)); err != nil {
		t.Fatalf("xray-core rejected the PIA WireGuard outbound the panel emits: %v", err)
	}
	second := `{
		"tag": "pia-us-west-uswest1",
		"piaHostname": "uswest1",
		"protocol": "wireguard",
		"settings": {
			"secretKey": "` + testWGKey(1) + `",
			"address": ["10.0.0.2/32"],
			"mtu": 1420,
			"noKernelTun": true,
			"peers": [{
				"publicKey": "` + testWGKey(2) + `",
				"endpoint": "198.51.100.20:51820",
				"allowedIPs": ["0.0.0.0/0"],
				"keepAlive": 25
			}]
		}
	}`
	if err := ValidateOutboundConfig([]byte(second)); err != nil {
		t.Fatalf("second PIA WireGuard outbound: %v", err)
	}
}
