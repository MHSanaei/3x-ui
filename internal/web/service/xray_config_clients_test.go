package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedVlessInbound(t *testing.T, tag string, port int, clients []model.Client) {
	t.Helper()
	setupSettingTestDB(t)
	db := database.GetDB()
	in := &model.Inbound{
		Tag:      tag,
		Enable:   true,
		Port:     port,
		Protocol: model.VLESS,
		Settings: `{"clients":[],"decryption":"none"}`,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create vless inbound: %v", err)
	}
	svc := ClientService{}
	if err := svc.SyncInbound(nil, in.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
}

func disableClients(t *testing.T, emails ...string) {
	t.Helper()
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email IN ?", emails).
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable clients: %v", err)
	}
}

func emittedClients(t *testing.T, tag string) (any, bool) {
	t.Helper()
	svc := &XrayService{}
	cfg, err := svc.GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag != tag {
			continue
		}
		var s map[string]any
		if err := json.Unmarshal([]byte(cfg.InboundConfigs[i].Settings), &s); err != nil {
			t.Fatalf("unmarshal emitted settings: %v", err)
		}
		v, ok := s["clients"]
		return v, ok
	}
	t.Fatalf("inbound %q not found in generated config", tag)
	return nil, false
}

func TestGetXrayConfig_EmptyClientListIsArrayNotNull(t *testing.T) {
	seedVlessInbound(t, "vless-empty", 43101, []model.Client{
		{Email: "gone@x", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
	})
	disableClients(t, "gone@x")

	value, present := emittedClients(t, "vless-empty")
	if !present {
		t.Fatal("the clients key must be present on a vless inbound")
	}
	if value == nil {
		t.Fatal(`settings.clients must be [] when every client is filtered out, got null`)
	}
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("settings.clients must be an array, got %T", value)
	}
	if len(list) != 0 {
		t.Fatalf("a disabled client must not reach the config, got %d entries", len(list))
	}
}

func TestGetXrayConfig_EnabledClientsStillEmitted(t *testing.T) {
	seedVlessInbound(t, "vless-mixed", 43102, []model.Client{
		{Email: "live@x", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
		{Email: "off@x", ID: "33333333-3333-3333-3333-333333333333", Enable: true},
	})
	disableClients(t, "off@x")

	value, _ := emittedClients(t, "vless-mixed")
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("settings.clients must be an array, got %T", value)
	}
	if len(list) != 1 {
		t.Fatalf("expected only the enabled client, got %d entries: %#v", len(list), list)
	}
	entry, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("client entry must be an object, got %T", list[0])
	}
	if entry["email"] != "live@x" {
		t.Errorf("wrong client emitted: %#v", entry)
	}
	if entry["id"] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("client id not carried through: %#v", entry)
	}
}

// A flow kept from a TCP+REALITY stream makes xray demand Vision on a transport that cannot run it
// without VLESS encryption: "XTLS only supports TLS and REALITY directly".
func TestGetXrayConfig_OmitsVisionFlowOnIneligibleStream(t *testing.T) {
	cases := []struct {
		name   string
		stream string
	}{
		{"xdrive", `{"network":"xdrive","security":"none","xdriveSettings":{"service":"local","remoteFolder":"/tmp/shared"}}`},
		{"ws over tls", `{"network":"ws","security":"tls","wsSettings":{"path":"/"}}`},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tag := "stale-flow"
			seedVlessInbound(t, tag, 43110+i, []model.Client{
				{Email: "vision@x", ID: "44444444-4444-4444-4444-444444444444", Enable: true, Flow: "xtls-rprx-vision"},
			})
			if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", tag).
				Update("stream_settings", tc.stream).Error; err != nil {
				t.Fatalf("set stream: %v", err)
			}
			value, _ := emittedClients(t, tag)
			list, ok := value.([]any)
			if !ok || len(list) != 1 {
				t.Fatalf("settings.clients = %#v, want the one enabled client", value)
			}
			entry, _ := list[0].(map[string]any)
			if flow, present := entry["flow"]; present {
				t.Fatalf("flow %q emitted for a %s stream", flow, tc.name)
			}
		})
	}
}

// xray-core runs Vision over TLS/REALITY on RAW, or over VLESS encryption on any transport.
func TestGetXrayConfig_KeepsVisionFlowWhereTheCoreRunsIt(t *testing.T) {
	const vlessenc = `{"clients":[],"decryption":"mlkem768x25519plus.native.600s.WB3OymHLL_DJ1jdHiqcMC-u84TgvfNFlgbZIJE-NtUQ"}`
	cases := []struct {
		name     string
		stream   string
		settings string
	}{
		{"tcp reality", `{"network":"tcp","security":"reality","realitySettings":{}}`, `{"clients":[],"decryption":"none"}`},
		{"raw vlessenc", `{"network":"tcp","security":"none"}`, vlessenc},
		{"ws vlessenc", `{"network":"ws","security":"none","wsSettings":{"path":"/"}}`, vlessenc},
		{"xdrive vlessenc", `{"network":"xdrive","security":"none","xdriveSettings":{"service":"local","remoteFolder":"/tmp/shared"}}`, vlessenc},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tag := "vision-flow"
			seedVlessInbound(t, tag, 43120+i, []model.Client{
				{Email: "vision@x", ID: "55555555-5555-5555-5555-555555555555", Enable: true, Flow: "xtls-rprx-vision"},
			})
			if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", tag).
				Updates(map[string]any{"stream_settings": tc.stream, "settings": tc.settings}).Error; err != nil {
				t.Fatalf("set stream: %v", err)
			}
			value, _ := emittedClients(t, tag)
			list, _ := value.([]any)
			if len(list) != 1 {
				t.Fatalf("settings.clients = %#v, want one client", value)
			}
			if entry, _ := list[0].(map[string]any); entry["flow"] != "xtls-rprx-vision" {
				t.Fatalf("flow = %#v, want xtls-rprx-vision kept on %s", entry["flow"], tc.name)
			}
		})
	}
}
