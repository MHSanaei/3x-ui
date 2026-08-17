package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const realityFinalMaskStream = `{"network":"tcp","security":"reality","realitySettings":{},"finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`

func TestValidateFinalMaskRealityCombo(t *testing.T) {
	tests := []struct {
		name           string
		streamSettings string
		wantErr        bool
	}{
		{
			name:           "empty streamSettings",
			streamSettings: "",
			wantErr:        false,
		},
		{
			name:           "reality without finalmask",
			streamSettings: `{"security":"reality","realitySettings":{}}`,
			wantErr:        false,
		},
		{
			name:           "reality with empty finalmask",
			streamSettings: `{"security":"reality","finalmask":{"tcp":[],"udp":[]}}`,
			wantErr:        false,
		},
		{
			name:           "reality with tcp fragment finalmask",
			streamSettings: `{"security":"reality","finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantErr:        true,
		},
		{
			// UDP masks never touch the TCP accept path REALITY runs on —
			// TcpmaskManager (the thing that wraps the listener ahead of
			// REALITY's handshake) is only built when tcp masks are present,
			// so a udp-only config doesn't reproduce the panic and shouldn't
			// be rejected.
			name:           "reality with udp-only finalmask (does not reproduce the panic)",
			streamSettings: `{"security":"reality","finalmask":{"udp":[{"type":"salamander"}]}}`,
			wantErr:        false,
		},
		{
			name:           "non-reality security with finalmask",
			streamSettings: `{"security":"tls","finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFinalMaskRealityCombo(tt.streamSettings)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFinalMaskRealityCombo(%q) error = %v, wantErr %v", tt.streamSettings, err, tt.wantErr)
			}
		})
	}
}

// end-to-end: the guard must actually be wired into AddInbound, not just
// exist as a standalone helper a caller could forget to invoke.
func TestAddInbound_RejectsFinalMaskRealityCombo(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	in := &model.Inbound{
		Tag:            "in-44300-tcp",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           44300,
		Protocol:       model.VLESS,
		StreamSettings: realityFinalMaskStream,
		Settings:       `{"clients":[]}`,
	}
	if _, _, err := svc.AddInbound(in); err == nil {
		t.Fatal("AddInbound: want error for finalmask+reality, got nil")
	}

	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "in-44300-tcp").Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("AddInbound: rejected inbound was persisted anyway, row count = %d", count)
	}
}

func TestAddInbound_IgnoresBoundIdAndCreatesNewRow(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	first := &model.Inbound{Tag: "in-45100-tcp", Enable: true, Listen: "0.0.0.0", Port: 45100, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	created, _, err := svc.AddInbound(first)
	if err != nil {
		t.Fatalf("AddInbound first: %v", err)
	}

	second := &model.Inbound{Id: created.Id, Tag: "in-45101-tcp", Enable: true, Listen: "0.0.0.0", Port: 45101, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if _, _, err := svc.AddInbound(second); err != nil {
		t.Fatalf("AddInbound second: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 inbound rows, got %d: a bound id overwrote the first row instead of creating a new one", count)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, created.Id).Error; err != nil {
		t.Fatalf("reload first: %v", err)
	}
	if reloaded.Port != 45100 {
		t.Fatalf("first inbound port = %d, want 45100 (the second add overwrote it)", reloaded.Port)
	}
}

func TestAddInbound_AcceptsWireguardClientWithKey(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	settings := `{"secretKey":"` + wgTestSecretKey() + `","mtu":1420,"clients":[{"email":"wgimp@x","enable":true,"privateKey":"keep-priv","publicKey":"keep-pub","allowedIPs":["10.0.0.50/32"]}]}`
	in := &model.Inbound{
		Tag:      "in-45200-wg",
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     45200,
		Protocol: model.WireGuard,
		Settings: settings,
	}
	if _, _, err := svc.AddInbound(in); err != nil {
		t.Fatalf("AddInbound rejected a keyed WireGuard client: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "in-45200-wg").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("WireGuard inbound with a keyed client was not created, row count = %d", count)
	}
}

// end-to-end: same guard on the update path, on a row that was valid before
// the edit — the rejected StreamSettings must not overwrite the stored row.
func TestUpdateInbound_RejectsFinalMaskRealityCombo(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-44301-tcp", "0.0.0.0", 44301, model.VLESS,
		`{"network":"tcp","security":"reality","realitySettings":{}}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-44301-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	update := existing
	update.StreamSettings = realityFinalMaskStream
	if _, _, err := svc.UpdateInbound(&update); err == nil {
		t.Fatal("UpdateInbound: want error for finalmask+reality, got nil")
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.StreamSettings != existing.StreamSettings {
		t.Fatalf("UpdateInbound: rejected StreamSettings was persisted anyway\ngot:  %s\nwant: %s", reloaded.StreamSettings, existing.StreamSettings)
	}
}

// GetXrayConfig must heal a row that already carries finalmask+reality in the
// DB (saved before this guard existed - an upgrade, a node sync, a restored
// backup, or a direct DB edit) rather than handing xray-core a config that
// panics it on the first connection. Bypasses AddInbound/UpdateInbound
// entirely by writing the row directly, the same way a pre-existing bad row
// would already be sitting in a real database.
func TestGetXrayConfig_HealsFinalMaskRealityCombo(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-44302-tcp", "0.0.0.0", 44302, model.VLESS,
		realityFinalMaskStream, `{"clients":[]}`)

	svc := &XrayService{}
	cfg, err := svc.GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}

	for i := range cfg.InboundConfigs {
		ic := cfg.InboundConfigs[i]
		if ic.Tag != "in-44302-tcp" {
			continue
		}
		var stream map[string]any
		if err := json.Unmarshal(ic.StreamSettings, &stream); err != nil {
			t.Fatalf("unmarshal emitted streamSettings: %v", err)
		}
		if stream["security"] != "reality" {
			t.Fatalf("security = %v, want reality (test setup broken)", stream["security"])
		}
		if _, has := stream["finalmask"]; has {
			t.Fatalf("emitted config still carries finalmask alongside reality — this crashes Xray-core: %v", stream["finalmask"])
		}
		return
	}
	t.Fatalf("inbound in-44302-tcp not found in generated config")
}
