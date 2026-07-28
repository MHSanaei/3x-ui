package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func settingsVersion(t *testing.T, settings string) any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return parsed["version"]
}

func TestHealHysteriaVersion(t *testing.T) {
	tests := []struct {
		name        string
		settings    string
		wantChanged bool
		wantVersion any
	}{
		{
			name:        "legacy v1 row",
			settings:    `{"version":1,"clients":[{"auth":"tok","email":"a@x"}]}`,
			wantChanged: true,
			wantVersion: float64(2),
		},
		{
			name:        "no version at all",
			settings:    `{"clients":[{"auth":"tok","email":"a@x"}]}`,
			wantChanged: true,
			wantVersion: float64(2),
		},
		{
			name:        "already v2",
			settings:    `{"version":2,"clients":[]}`,
			wantChanged: false,
			wantVersion: float64(2),
		},
		{
			name:        "version as a string",
			settings:    `{"version":"1","clients":[]}`,
			wantChanged: true,
			wantVersion: float64(2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healed, changed := HealHysteriaVersion(tt.settings)
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if got := settingsVersion(t, healed); got != tt.wantVersion {
				t.Fatalf("version = %#v, want %#v", got, tt.wantVersion)
			}
		})
	}
}

func TestHealHysteriaVersionKeepsClients(t *testing.T) {
	healed, changed := HealHysteriaVersion(`{"version":1,"clients":[{"auth":"tok","email":"a@x"}]}`)
	if !changed {
		t.Fatal("a v1 row must be healed")
	}
	if !strings.Contains(healed, `"auth": "tok"`) || !strings.Contains(healed, `"email": "a@x"`) {
		t.Fatalf("healing dropped client data: %s", healed)
	}
}

func TestHealHysteriaVersionLeavesUnparsableSettings(t *testing.T) {
	const broken = `{"version":1,`
	healed, changed := HealHysteriaVersion(broken)
	if changed || healed != broken {
		t.Fatalf("unparsable settings must be left alone, got changed=%v %q", changed, healed)
	}
	if healed, changed := HealHysteriaVersion(""); changed || healed != "" {
		t.Fatalf("empty settings must be left alone, got changed=%v %q", changed, healed)
	}
}

func TestHealHysteriaStreamVersion(t *testing.T) {
	healed, changed := HealHysteriaStreamVersion(`{"network":"hysteria","hysteriaSettings":{"version":1,"udpIdleTimeout":60}}`)
	if !changed {
		t.Fatal("a v1 transport must be healed")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(healed), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	hysteria, _ := parsed["hysteriaSettings"].(map[string]any)
	if hysteria["version"] != float64(2) {
		t.Fatalf("version = %#v, want 2", hysteria["version"])
	}
	if hysteria["udpIdleTimeout"] != float64(60) {
		t.Fatalf("healing dropped transport settings: %#v", hysteria)
	}
}

func TestHealHysteriaStreamVersionWithoutHysteriaSettings(t *testing.T) {
	const stream = `{"network":"tcp","tcpSettings":{}}`
	healed, changed := HealHysteriaStreamVersion(stream)
	if changed || healed != stream {
		t.Fatalf("a stream without hysteriaSettings must be left alone, got changed=%v %q", changed, healed)
	}
}

// TestGenXrayInboundConfigHealsHysteriaVersion is the regression for a stored
// v1 row: xray-core answers "version != 2" and rejects the whole config, so
// every other inbound on the server stays offline until the row is fixed.
func TestGenXrayInboundConfigHealsHysteriaVersion(t *testing.T) {
	in := Inbound{
		Protocol:       Hysteria,
		Port:           36715,
		Listen:         "127.0.0.1",
		Tag:            "in-hysteria",
		Settings:       `{"version":1,"clients":[{"auth":"tok","email":"a@x"}]}`,
		StreamSettings: `{"network":"hysteria","hysteriaSettings":{"version":1,"udpIdleTimeout":60}}`,
	}
	cfg := in.GenXrayInboundConfig()

	if got := settingsVersion(t, string(cfg.Settings)); got != float64(2) {
		t.Fatalf("generated settings.version = %#v, want 2", got)
	}
	var stream map[string]any
	if err := json.Unmarshal(cfg.StreamSettings, &stream); err != nil {
		t.Fatalf("unmarshal generated streamSettings: %v", err)
	}
	hysteria, _ := stream["hysteriaSettings"].(map[string]any)
	if hysteria["version"] != float64(2) {
		t.Fatalf("generated hysteriaSettings.version = %#v, want 2", hysteria["version"])
	}

	if !strings.Contains(in.Settings, `"version":1`) {
		t.Fatal("the stored row must keep its own value; only the generated config is healed")
	}
}

func TestGenXrayInboundConfigLeavesOtherProtocolsAlone(t *testing.T) {
	in := Inbound{
		Protocol: VLESS,
		Port:     443,
		Tag:      "in-vless",
		Settings: `{"clients":[],"decryption":"none"}`,
	}
	if got := settingsVersion(t, string(in.GenXrayInboundConfig().Settings)); got != nil {
		t.Fatalf("a non-hysteria inbound must not gain a version key, got %#v", got)
	}
}
