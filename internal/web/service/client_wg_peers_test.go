package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRemovePeerFromSettingsMatchesPublicKeyWithoutComment(t *testing.T) {
	settings := `{
  "peers": [
    { "publicKey": "old-key", "allowedIPs": ["10.0.0.2/32"] },
    { "publicKey": "keep-key", "comment": "keep", "allowedIPs": ["10.0.0.3/32"] }
  ]
}`

	updated, err := removePeerFromSettings(settings, "wg-10-peer-1", "old-key")
	if err != nil {
		t.Fatalf("removePeerFromSettings: %v", err)
	}
	if strings.Contains(updated, "old-key") {
		t.Fatalf("old peer was not removed: %s", updated)
	}
	if !strings.Contains(updated, "keep-key") {
		t.Fatalf("unrelated peer was removed: %s", updated)
	}
}

func TestUpdatePeerInSettingsMatchesPublicKeyWithoutComment(t *testing.T) {
	settings := `{
  "peers": [
    { "publicKey": "old-key", "allowedIPs": ["10.0.0.2/32"] }
  ]
}`
	newPeer := map[string]any{
		"publicKey":  "new-key",
		"comment":    "renamed",
		"allowedIPs": []string{"10.0.0.4/32"},
	}

	updated, err := updatePeerInSettings(settings, "wg-10-peer-1", "old-key", newPeer, true)
	if err != nil {
		t.Fatalf("updatePeerInSettings: %v", err)
	}
	if strings.Contains(updated, "old-key") {
		t.Fatalf("old peer was not removed: %s", updated)
	}
	if !strings.Contains(updated, "new-key") {
		t.Fatalf("new peer was not added: %s", updated)
	}

	var parsed struct {
		Peers []map[string]any `json:"peers"`
	}
	if err := json.Unmarshal([]byte(updated), &parsed); err != nil {
		t.Fatalf("updated JSON: %v", err)
	}
	if len(parsed.Peers) != 1 {
		t.Fatalf("peer count = %d, want 1: %s", len(parsed.Peers), updated)
	}
}

func TestWgPeerToRecordFallbackEmailUsesInboundID(t *testing.T) {
	rec := wgPeerToRecord(map[string]any{"publicKey": "pk"}, 42, 2)
	if rec.Email != "wg-42-peer-3" {
		t.Fatalf("fallback email = %q, want wg-42-peer-3", rec.Email)
	}
}
