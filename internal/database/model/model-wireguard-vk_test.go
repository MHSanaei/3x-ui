package model

import (
	"strings"
	"testing"
)

func TestGenXrayInboundConfigRemovesVKTurnMetadata(t *testing.T) {
	for _, identity := range []string{
		`"clients":[{"publicKey":"peer","enable":true}]`,
		`"peers":[{"publicKey":"peer"}]`,
	} {
		t.Run(identity, func(t *testing.T) {
			settings := `{"secretKey":"server",` + identity + `,"vkTurn":{"enabled":true,"callLink":"https://vk.ru/call/join/example","port":56000}}`
			inbound := Inbound{Protocol: WireGuard, Port: 51820, Settings: settings}
			config := inbound.GenXrayInboundConfig()
			parsed := wgSettingsParsed(t, string(config.Settings))
			if _, exists := parsed["vkTurn"]; exists {
				t.Fatal("Xray configuration contains panel VK transport metadata")
			}
			peers, ok := parsed["peers"].([]any)
			if !ok || len(peers) != 1 || peers[0].(map[string]any)["publicKey"] != "peer" {
				t.Fatalf("WireGuard peers changed: %v", parsed["peers"])
			}
			if !strings.Contains(inbound.Settings, `"vkTurn"`) {
				t.Fatal("core config generation changed persisted transport settings")
			}
		})
	}
}
