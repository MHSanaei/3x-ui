package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func wgTestSecretKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func wgInboundEmittedSettings(t *testing.T, tag string) map[string]any {
	t.Helper()
	svc := &XrayService{}
	cfg, err := svc.GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	for i := range cfg.InboundConfigs {
		ic := cfg.InboundConfigs[i]
		if ic.Tag != tag {
			continue
		}
		var s map[string]any
		if err := json.Unmarshal([]byte(ic.Settings), &s); err != nil {
			t.Fatalf("unmarshal emitted settings: %v", err)
		}
		return s
	}
	t.Fatalf("inbound %q not found in generated config", tag)
	return nil
}

func seedWGInbound(t *testing.T, tag string, port int, clients []model.Client) {
	t.Helper()
	setupSettingTestDB(t)
	db := database.GetDB()
	in := &model.Inbound{
		Tag:      tag,
		Enable:   true,
		Port:     port,
		Protocol: model.WireGuard,
		Settings: `{"secretKey":"` + wgTestSecretKey() + `","mtu":1420}`,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create wg inbound: %v", err)
	}
	svc := ClientService{}
	if err := svc.SyncInbound(nil, in.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
}

func wgPeerList(t *testing.T, settings map[string]any) []map[string]any {
	t.Helper()
	if _, ok := settings["clients"]; ok {
		t.Fatalf("wireguard inbound must not emit a clients[] key: %v", settings["clients"])
	}
	rawPeers, ok := settings["peers"].([]any)
	if !ok {
		t.Fatalf("settings.peers is not an array: %T", settings["peers"])
	}
	out := make([]map[string]any, 0, len(rawPeers))
	for _, p := range rawPeers {
		m, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("peer is not an object: %T", p)
		}
		out = append(out, m)
	}
	return out
}

func TestGetXrayConfigWireGuardPeers(t *testing.T) {
	clients := []model.Client{
		{Email: "alice@wg.test", Enable: true, PublicKey: "pub-alice", AllowedIPs: []string{"10.0.0.2/32"}, KeepAlive: 25},
		{Email: "bob@wg.test", Enable: true, PublicKey: "pub-bob", AllowedIPs: []string{"10.0.0.3/32"}},
	}
	seedWGInbound(t, "wg-multi", 51820, clients)

	settings := wgInboundEmittedSettings(t, "wg-multi")
	if settings["secretKey"] != wgTestSecretKey() {
		t.Errorf("secretKey not preserved: %v", settings["secretKey"])
	}
	if settings["mtu"] != float64(1420) {
		t.Errorf("mtu not preserved: %v", settings["mtu"])
	}

	peers := wgPeerList(t, settings)
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d: %v", len(peers), peers)
	}
	ips := map[string]bool{}
	for _, p := range peers {
		if p["email"] == nil || p["email"] == "" {
			t.Errorf("peer missing email: %v", p)
		}
		if p["publicKey"] == nil || p["publicKey"] == "" {
			t.Errorf("peer missing publicKey: %v", p)
		}
		if p["level"] != float64(0) {
			t.Errorf("peer level = %v, want 0 (needed for per-user stats)", p["level"])
		}
		allowed, ok := p["allowedIPs"].([]any)
		if !ok || len(allowed) == 0 {
			t.Fatalf("peer missing allowedIPs: %v", p)
		}
		ips[allowed[0].(string)] = true
	}
	if len(ips) != 2 {
		t.Errorf("peers must have distinct allowedIPs, got %v", ips)
	}
}

func TestGetXrayConfigWireGuardDisabledClientExcluded(t *testing.T) {
	clients := []model.Client{
		{Email: "on@wg.test", Enable: true, PublicKey: "pub-on", AllowedIPs: []string{"10.0.0.2/32"}},
		{Email: "off@wg.test", Enable: true, PublicKey: "pub-off", AllowedIPs: []string{"10.0.0.3/32"}},
	}
	seedWGInbound(t, "wg-disabled", 51821, clients)

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", "off@wg.test").Update("enable", false).Error; err != nil {
		t.Fatalf("disable client: %v", err)
	}

	peers := wgPeerList(t, wgInboundEmittedSettings(t, "wg-disabled"))
	if len(peers) != 1 {
		t.Fatalf("expected 1 enabled peer, got %d: %v", len(peers), peers)
	}
	if peers[0]["email"] != "on@wg.test" {
		t.Errorf("wrong peer kept: %v", peers[0])
	}
}

func TestGetXrayConfigWireGuardNoClientsEmitsEmptyPeers(t *testing.T) {
	seedWGInbound(t, "wg-empty", 51822, nil)

	settings := wgInboundEmittedSettings(t, "wg-empty")
	if _, ok := settings["clients"]; ok {
		t.Fatalf("clients key must be absent")
	}
	peers, ok := settings["peers"].([]any)
	if !ok {
		t.Fatalf("peers must be an (empty) array, got %T", settings["peers"])
	}
	if len(peers) != 0 {
		t.Fatalf("expected empty peers, got %v", peers)
	}
}
