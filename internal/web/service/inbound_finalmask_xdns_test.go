package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// legacyXdnsStream carries the 26.9.30 xdns objects, whose "name" and typed resolvers
// xray-core 26.10.10 ignores: the mask would load with no domain at all.
const legacyXdnsStream = `{"network":"kcp","security":"none","kcpSettings":{"mtu":900},"finalmask":{"udp":[{"type":"xdns","settings":{"domains":[{"name":"t.example.com","types":[16],"edns0":1232}],"resolvers":[{"type":"udp","settings":{"addr":"8.8.8.8:53"}}]}}]}}`

const upgradedXdnsSettings = `{"domains":[{"edns0":1232,"names":["t.example.com"],"types":[16]}],"resolvers":[{"addrs":["udp://8.8.8.8:53"]}]}`

func assertXdnsSettingsUpgraded(t *testing.T, stream map[string]any) {
	t.Helper()
	finalmask, _ := stream["finalmask"].(map[string]any)
	udp, _ := finalmask["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("finalmask.udp = %v, want one mask", finalmask["udp"])
	}
	mask, _ := udp[0].(map[string]any)
	got, err := json.Marshal(mask["settings"])
	if err != nil {
		t.Fatalf("marshal xdns settings: %v", err)
	}
	if string(got) != upgradedXdnsSettings {
		t.Fatalf("xdns settings\n got: %s\nwant: %s", got, upgradedXdnsSettings)
	}
}

// An API client or an older panel's export can still post the 26.9.30 shape; stored as
// sent it would reach the sub links and the form in a shape the core silently empties.
func TestAddInbound_StoresXdnsMaskInNamesShape(t *testing.T) {
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
	assertXdnsSettingsUpgraded(t, stream)
}

// A row that never went through the save path (restored backup, node sync, direct DB
// edit) must still reach the core with its xdns domains, or the mask serves nothing.
func TestGetXrayConfig_UpgradesLegacyXdnsMask(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-45301-kcp", "127.0.0.1", 45301, model.VLESS,
		legacyXdnsStream, `{"clients":[],"decryption":"none"}`)

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
		stream, _ := emitted["streamSettings"].(map[string]any)
		assertXdnsSettingsUpgraded(t, stream)
		assertXrayAccepts(t, "the healed xdns inbound", buildGoldenInbound(t, emitted))
		return
	}
	t.Fatal("inbound in-45301-kcp not found in the generated config")
}

// A template pasted from a 26.9.30 panel after the seeder ran passes the save check,
// since the core loads that shape without error; only the build-time heal catches it.
func TestGetXrayConfig_UpgradesTemplateOutboundXdnsMask(t *testing.T) {
	setupConflictDB(t)
	template := `{"outbounds":[{"protocol":"freedom","tag":"direct"},{"protocol":"vless","tag":"dns-tunnel",
		"settings":{"vnext":[{"address":"t.example.com","port":53,"users":[{"id":"c9f0c2d0-0000-4000-8000-000000000000","encryption":"none"}]}]},
		"streamSettings":` + legacyXdnsStream + `}]}`
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", template); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("decode emitted outbounds: %v", err)
	}
	for _, outbound := range outbounds {
		if outbound["tag"] == "dns-tunnel" {
			stream, _ := outbound["streamSettings"].(map[string]any)
			assertXdnsSettingsUpgraded(t, stream)
			return
		}
	}
	t.Fatal("outbound dns-tunnel not found in the generated config")
}

// A template outbound pasted from before 26.9.9 carries quicParams.udpHop, which the core ignores,
// and a hand-ordered udphop not last makes it refuse every dial; the build heals both.
func TestGetXrayConfig_HealsTemplateOutboundUDPHop(t *testing.T) {
	const hop = `{"settings":{"interval":"5-10","mode":"intervalRemote","remotePorts":"20000-30000"},"type":"udphop"}`
	const salamander = `{"settings":{"password":"s3cr3t"},"type":"salamander"}`
	tests := []struct{ name, finalmask, want string }{
		{"legacy quicParams.udpHop", `{"quicParams":{"udpHop":{"ports":"20000-30000","interval":"5-10"}}}`, `{"udp":[` + hop + `]}`},
		{"udphop listed first", `{"udp":[` + hop + `,` + salamander + `]}`, `{"udp":[` + salamander + `,` + hop + `]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			template := `{"outbounds":[{"protocol":"freedom","tag":"direct"},{"protocol":"hysteria","tag":"hy-out",
				"settings":{"version":2,"address":"hy.example.com","port":443},
				"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"a"},
					"finalmask":` + tc.finalmask + `}}]}`
			if err := (&SettingService{}).saveSetting("xrayTemplateConfig", template); err != nil {
				t.Fatalf("seed template: %v", err)
			}

			cfg, err := (&XrayService{}).GetXrayConfig()
			if err != nil {
				t.Fatalf("GetXrayConfig: %v", err)
			}
			var outbounds []map[string]any
			if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
				t.Fatalf("decode emitted outbounds: %v", err)
			}
			for _, outbound := range outbounds {
				if outbound["tag"] != "hy-out" {
					continue
				}
				stream, _ := outbound["streamSettings"].(map[string]any)
				got, _ := json.Marshal(stream["finalmask"])
				if string(got) != tc.want {
					t.Fatalf("emitted finalmask\n got: %s\nwant: %s", got, tc.want)
				}
				return
			}
			t.Fatal("outbound hy-out not found in the generated config")
		})
	}
}
