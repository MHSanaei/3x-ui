package xray

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func testWGKey(seed byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestValidateOutboundConfig_TwoUserspaceWireGuard(t *testing.T) {
	build := func(tag, endpoint string) []byte {
		ob := map[string]any{
			"tag":      tag,
			"protocol": "wireguard",
			"settings": map[string]any{
				"secretKey":   testWGKey(1),
				"address":     []string{"10.0.0.2/32"},
				"mtu":         1420,
				"noKernelTun": true,
				"peers": []any{
					map[string]any{
						"publicKey":  testWGKey(2),
						"endpoint":   endpoint,
						"allowedIPs": []string{"0.0.0.0/0"},
						"keepAlive":  25,
					},
				},
			},
		}
		raw, err := json.Marshal(ob)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	a := build("pia-aaaa1111", "198.51.100.10:51820")
	b := build("pia-bbbb2222", "198.51.100.20:51820")
	if err := ValidateOutboundConfig(a); err != nil {
		t.Fatalf("first wireguard outbound: %v", err)
	}
	if err := ValidateOutboundConfig(b); err != nil {
		t.Fatalf("second wireguard outbound: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(a, &parsed); err != nil {
		t.Fatal(err)
	}
	settings := parsed["settings"].(map[string]any)
	addr, ok := settings["address"].([]any)
	if !ok || len(addr) != 1 || addr[0] != "10.0.0.2/32" {
		t.Fatalf("address must be a string array, got %#v", settings["address"])
	}
	if settings["noKernelTun"] != true {
		t.Fatalf("noKernelTun: %#v", settings["noKernelTun"])
	}
	peers := settings["peers"].([]any)
	peer := peers[0].(map[string]any)
	if peer["keepAlive"] != float64(25) {
		t.Fatalf("keepAlive: %#v", peer["keepAlive"])
	}
	ips := peer["allowedIPs"].([]any)
	if len(ips) != 1 || ips[0] != "0.0.0.0/0" {
		t.Fatalf("allowedIPs must be IPv4-only, got %#v", ips)
	}
	for _, ip := range ips {
		if strings.Contains(ip.(string), ":") {
			t.Fatalf("IPv6 allowedIP leaked into PIA fixture: %v", ip)
		}
	}
}
