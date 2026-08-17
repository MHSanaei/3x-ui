package model

import (
	"encoding/json"
	"testing"
)

// TestHealShadowsocksClientMethods_RewritesRemovedCipher covers the last-gate
// build-time heal for xray-core v26.7.11's removed "none"/"plain" ciphers: a
// row that survives to config generation (restored backup, direct DB edit)
// must be rewritten to a supported cipher on both the inbound method and its
// clients so one such inbound cannot keep xray from starting.
func TestHealShadowsocksClientMethods_RewritesRemovedCipher(t *testing.T) {
	settings := `{"method": "plain", "clients": [{"email": "a@x", "password": "p", "method": "plain"}]}`
	healed, ok := HealShadowsocksClientMethods(settings)
	if !ok {
		t.Fatal("expected heal to report a change for a removed cipher")
	}
	var parsed struct {
		Method  string           `json:"method"`
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(healed), &parsed); err != nil {
		t.Fatalf("parse healed settings: %v", err)
	}
	if parsed.Method != "chacha20-ietf-poly1305" {
		t.Fatalf("expected inbound method rewritten to a supported cipher, got %q", parsed.Method)
	}
	if parsed.Clients[0]["method"] != "chacha20-ietf-poly1305" {
		t.Fatalf("expected client method to match the healed cipher, got %v", parsed.Clients[0]["method"])
	}
}

func TestReplaceRemovedShadowsocksCipher(t *testing.T) {
	for _, method := range []string{"none", "plain"} {
		if got, removed := ReplaceRemovedShadowsocksCipher(method); !removed || got != "chacha20-ietf-poly1305" {
			t.Fatalf("ReplaceRemovedShadowsocksCipher(%q) = (%q, %v), want a supported replacement", method, got, removed)
		}
	}
	if got, removed := ReplaceRemovedShadowsocksCipher("aes-256-gcm"); removed || got != "aes-256-gcm" {
		t.Fatalf("ReplaceRemovedShadowsocksCipher(aes-256-gcm) = (%q, %v), want it left untouched", got, removed)
	}
}
