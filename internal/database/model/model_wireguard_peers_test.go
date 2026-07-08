package model

import (
	"encoding/json"
	"testing"
)

func wgSettingsParsed(t *testing.T, settings string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return parsed
}

func TestWireguardClientsToPeers(t *testing.T) {
	settings := `{
		"secretKey": "c2VydmVyLXNlY3JldC1rZXktYmFzZTY0LTMyYnl0ZXM=",
		"mtu": 1420,
		"clients": [
			{"email": "alice", "enable": true, "publicKey": "cHVi", "allowedIPs": ["10.0.0.2/32"], "preSharedKey": "cHNr", "keepAlive": 25},
			{"email": "bob", "enable": false, "publicKey": "cHViMg==", "allowedIPs": ["10.0.0.3/32"]}
		]
	}`

	out, ok := WireguardClientsToPeers(settings)
	if !ok {
		t.Fatal("WireguardClientsToPeers returned ok=false, want true")
	}
	parsed := wgSettingsParsed(t, out)

	if _, has := parsed["clients"]; has {
		t.Error("clients key must be removed after conversion")
	}
	if parsed["secretKey"] != "c2VydmVyLXNlY3JldC1rZXktYmFzZTY0LTMyYnl0ZXM=" {
		t.Errorf("secretKey not preserved: %v", parsed["secretKey"])
	}

	peers, ok := parsed["peers"].([]any)
	if !ok {
		t.Fatalf("peers not an array: %T", parsed["peers"])
	}
	if len(peers) != 1 {
		t.Fatalf("peers length = %d, want 1 (disabled client must be skipped)", len(peers))
	}

	peer := peers[0].(map[string]any)
	if peer["publicKey"] != "cHVi" {
		t.Errorf("peer publicKey = %v, want cHVi", peer["publicKey"])
	}
	if peer["preSharedKey"] != "cHNr" {
		t.Errorf("peer preSharedKey = %v, want cHNr", peer["preSharedKey"])
	}
	if peer["keepAlive"].(float64) != 25 {
		t.Errorf("peer keepAlive = %v, want 25", peer["keepAlive"])
	}
	ips, ok := peer["allowedIPs"].([]any)
	if !ok || len(ips) != 1 || ips[0] != "10.0.0.2/32" {
		t.Errorf("peer allowedIPs = %v, want [10.0.0.2/32]", peer["allowedIPs"])
	}
}

func TestWireguardClientsToPeersIdempotent(t *testing.T) {
	withPeers := `{"secretKey": "k", "peers": [{"publicKey": "cHVi"}]}`
	if out, ok := WireguardClientsToPeers(withPeers); ok || out != withPeers {
		t.Errorf("settings with peers must be a no-op: ok=%v out=%q", ok, out)
	}

	noClients := `{"secretKey": "k", "mtu": 1420}`
	if out, ok := WireguardClientsToPeers(noClients); ok || out != noClients {
		t.Errorf("settings without clients must be a no-op: ok=%v out=%q", ok, out)
	}
}

func TestGenXrayInboundConfigWireguardConvertsPeers(t *testing.T) {
	ib := &Inbound{
		Protocol: WireGuard,
		Port:     51820,
		Tag:      "wg-in",
		Settings: `{"secretKey": "k", "peers": [], "clients": [{"email": "alice", "enable": true, "publicKey": "cHVi", "allowedIPs": ["10.0.0.2/32"]}]}`,
	}

	cfg := ib.GenXrayInboundConfig()
	parsed := wgSettingsParsed(t, string(cfg.Settings))

	if _, has := parsed["clients"]; has {
		t.Error("GenXrayInboundConfig left clients in a wireguard inbound")
	}
	peers, ok := parsed["peers"].([]any)
	if !ok || len(peers) != 1 {
		t.Fatalf("GenXrayInboundConfig did not emit peers: %v", parsed["peers"])
	}
	if peers[0].(map[string]any)["publicKey"] != "cHVi" {
		t.Errorf("peer publicKey = %v, want cHVi", peers[0].(map[string]any)["publicKey"])
	}
}
