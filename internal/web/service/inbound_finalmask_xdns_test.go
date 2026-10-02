package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const legacyXdnsStream = `{"network":"kcp","security":"none","kcpSettings":{"mtu":900},"finalmask":{"udp":[{"type":"xdns","settings":{"domains":["t.example.com"]}}]}}`

func firstXdnsDomain(t *testing.T, stream map[string]any) any {
	t.Helper()
	finalmask, _ := stream["finalmask"].(map[string]any)
	udp, _ := finalmask["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("finalmask.udp = %v, want one mask", finalmask["udp"])
	}
	mask, _ := udp[0].(map[string]any)
	settings, _ := mask["settings"].(map[string]any)
	domains, _ := settings["domains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("xdns domains = %v, want one", settings["domains"])
	}
	return domains[0]
}

// An API client can still post the pre-26.9.30 string lists; stored as sent they would
// reach the sub links and the form in a shape the core no longer parses.
func TestAddInbound_StoresXdnsMaskInObjectShape(t *testing.T) {
	setupConflictDB(t)
	in := &model.Inbound{
		Tag: "in-45300-kcp", Enable: true, Listen: "0.0.0.0", Port: 45300, Protocol: model.VLESS,
		Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: legacyXdnsStream,
	}
	if _, _, err := (&InboundService{}).AddInbound(in); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	var stored model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-45300-kcp").First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(stored.StreamSettings), &stream); err != nil {
		t.Fatalf("stored stream is not JSON: %v", err)
	}
	domain, ok := firstXdnsDomain(t, stream).(map[string]any)
	if !ok || domain["name"] != "t.example.com" {
		t.Fatalf("stored xdns domain = %#v, want an object named t.example.com", firstXdnsDomain(t, stream))
	}
}

// A row that never went through the save path (restored backup, node sync, direct DB
// edit) must still reach the core in a shape it builds, or it keeps every inbound down.
func TestGetXrayConfig_UpgradesLegacyXdnsMask(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-45301-kcp", "127.0.0.1", 45301, model.VLESS,
		legacyXdnsStream, `{"clients":[],"decryption":"none"}`)

	var legacy map[string]any
	if err := json.Unmarshal([]byte(`{"tag":"in-45301-kcp","listen":"127.0.0.1","port":45301,"protocol":"vless",
		"settings":{"clients":[],"decryption":"none"},"streamSettings":`+legacyXdnsStream+`}`), &legacy); err != nil {
		t.Fatalf("decode legacy inbound: %v", err)
	}
	if err := buildGoldenInbound(t, legacy); err == nil {
		t.Fatal("xray-core accepted the legacy xdns lists; the heal is no longer needed")
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag != "in-45301-kcp" {
			continue
		}
		raw, err := json.Marshal(cfg.InboundConfigs[i])
		if err != nil {
			t.Fatalf("marshal emitted inbound: %v", err)
		}
		var emitted map[string]any
		if err := json.Unmarshal(raw, &emitted); err != nil {
			t.Fatalf("decode emitted inbound: %v", err)
		}
		assertXrayAccepts(t, "the healed xdns inbound", buildGoldenInbound(t, emitted))
		return
	}
	t.Fatal("inbound in-45301-kcp not found in the generated config")
}
