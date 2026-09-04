package tuic

import (
	"encoding/json"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	inst := Instance{
		Id:                    1,
		Port:                  8443,
		Listen:                "0.0.0.0",
		Certificate:           "/etc/ssl/cert.pem",
		PrivateKey:            "/etc/ssl/key.pem",
		CongestionControl:     "bbr",
		ALPN:                  []string{"h3", "spdy/3.1"},
		UDPRelayMode:          "native",
		ZeroRTTHandshake:      true,
		LogLevel:              "info",
		MaxIdleTime:           15,
		AuthenticationTimeout: 3,
		MaxUdpRelayPacketSize: 1500,
		Clients: []TuicClientSettings{
			{UUID: "uuid-1", Password: "pass-1", Email: "e1"},
			{UUID: "uuid-2", Password: "pass-2", Email: "e2"},
		},
	}

	data, err := GenerateConfig(inst)
	if err != nil {
		t.Fatalf("GenerateConfig error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["server"] != "0.0.0.0:8443" {
		t.Fatalf("expected server 0.0.0.0:8443, got %v", parsed["server"])
	}
	if parsed["certificate"] != "/etc/ssl/cert.pem" || parsed["private_key"] != "/etc/ssl/key.pem" {
		t.Fatalf("unexpected cert/key in json: %v", parsed)
	}

	users, ok := parsed["users"].(map[string]any)
	if !ok {
		t.Fatalf("expected users map, got %T", parsed["users"])
	}
	if users["uuid-1"] != "pass-1" || users["uuid-2"] != "pass-2" {
		t.Fatalf("unexpected users in json: %v", users)
	}
}

func TestGenerateConfigLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"info", "info"},
		{"warn", "info"},
		{"error", "info"},
		{"", "info"},
		{"debug", "debug"},
		{"trace", "trace"},
	}

	for _, tc := range tests {
		inst := Instance{
			Id:       1,
			Port:     8443,
			Listen:   "0.0.0.0",
			LogLevel: tc.input,
		}
		data, err := GenerateConfig(inst)
		if err != nil {
			t.Fatalf("GenerateConfig error for %s: %v", tc.input, err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error for %s: %v", tc.input, err)
		}
		if parsed["log_level"] != tc.expected {
			t.Fatalf("expected log_level %s for input %s, got %v", tc.expected, tc.input, parsed["log_level"])
		}
	}
}
