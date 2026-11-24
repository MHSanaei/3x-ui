package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestGetXrayConfig_AggregatesExcludedClientLogs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clients  int
		inbounds int
		excluded int
		affected int
		shared   bool
	}{
		{"2000 excluded entries", 2000, 5, 2000, 5, false},
		{"mixed clients and inbounds", 8, 4, 3, 2, false},
		{"no excluded entries", 8, 4, 0, 0, false},
		{"shared client counts as two entries", 4, 2, 1, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSettingTestDB(t)
			ds := seedScaleDataset(t, tc.clients, tc.inbounds)
			excluded := make(map[string]bool, tc.excluded)
			for _, email := range ds.emails[:tc.excluded] {
				excluded[email] = true
			}
			if tc.shared {
				var record model.ClientRecord
				if err := database.GetDB().Where("email = ?", ds.emails[0]).First(&record).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.GetDB().Create(&model.ClientInbound{ClientId: record.Id, InboundId: ds.inboundIds[1]}).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Manual disablement is a separate filter and must not inflate the summary.
			if tc.excluded < tc.clients {
				disableClients(t, ds.emails[tc.clients-1])
			}
			svc := &XrayService{}
			want, err := svc.GetXrayConfig()
			if err != nil {
				t.Fatal(err)
			}
			for i, tag := range ds.tags {
				clients := make([]map[string]string, 0)
				for _, client := range ds.perInbound[i] {
					if !excluded[client.Email] && client.Email != ds.emails[tc.clients-1] {
						clients = append(clients, map[string]string{"email": client.Email, "id": client.ID})
					}
				}
				for k := range want.InboundConfigs {
					if want.InboundConfigs[k].Tag == tag {
						settings, err := json.MarshalIndent(map[string]any{"clients": clients}, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						want.InboundConfigs[k].Settings = settings
					}
				}
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if tc.excluded > 0 {
				if err := database.GetDB().Model(&xray.ClientTraffic{}).
					Where("email IN ?", ds.emails[:tc.excluded]).Update("enable", false).Error; err != nil {
					t.Fatal(err)
				}
			}
			for generation := range 2 {
				marker := fmt.Sprintf("%s generation %d", t.Name(), generation)
				logger.Info(marker)
				got, err := svc.GetXrayConfig()
				if err != nil {
					t.Fatal(err)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotJSON) != string(wantJSON) {
					t.Fatal("generated config changed beyond the expected client exclusions")
				}
				infoCount := 0
				foundMarker := false
				for _, line := range logger.GetLogs(10000, "DEBUG") {
					if strings.HasSuffix(line, " INFO - "+marker) {
						foundMarker = true
						break
					}
					if strings.Contains(line, "Remove Inbound User") || strings.Contains(line, "@scale") {
						t.Fatalf("per-client log emitted: %s", line)
					}
					for _, clients := range ds.perInbound {
						for _, client := range clients {
							if strings.Contains(line, client.ID) || strings.Contains(line, client.SubID) {
								t.Error("client credential or subscription ID leaked into logs")
							}
						}
					}
					if strings.Contains(line, " INFO - ") {
						infoCount++
						entries := tc.excluded
						if tc.shared {
							entries++
						}
						message := fmt.Sprintf(" INFO - Xray config generated: excluded %d disabled client entries across %d inbounds", entries, tc.affected)
						if !strings.HasSuffix(line, message) {
							t.Errorf("unexpected INFO entry: %s", line)
						}
					}
				}
				wantCount := 0
				if tc.excluded > 0 {
					wantCount = 1
				}
				if !foundMarker || infoCount != wantCount {
					t.Errorf("generation %d: marker found = %v, INFO entries = %d, want %d", generation, foundMarker, infoCount, wantCount)
				}
			}
		})
	}
}

func TestGetXrayConfig_FailureDoesNotLogExclusionSummary(t *testing.T) {
	setupSettingTestDB(t)
	ds := seedScaleDataset(t, 1, 1)
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", ds.emails[0]).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", ds.inboundIds[0]).
		Update("stream_settings", `{"network":"tcp","security":"tls","tlsSettings":{}}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.InboundFallback{}); err != nil {
		t.Fatal(err)
	}
	logger.Info(t.Name())
	cfg, err := (&XrayService{}).GetXrayConfig()
	if cfg != nil || err == nil || err.Error() != "no such table: inbound_fallbacks" {
		t.Fatalf("GetXrayConfig = %v, %v; want missing fallback table failure", cfg, err)
	}
	for _, line := range logger.GetLogs(100, "DEBUG") {
		if strings.HasSuffix(line, " INFO - "+t.Name()) {
			return
		}
		if strings.Contains(line, "Xray config generated:") || strings.Contains(line, "Remove Inbound User") {
			t.Fatalf("failed generation logged a successful summary or per-client removal: %s", line)
		}
	}
	t.Fatal("log boundary not found")
}

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
// without VLESS encryption: "XTLS only supports TLS and REALITY directly". XDRIVE refuses it even
// inside VLESS encryption since xray-core 26.10.17 (#7182).
func TestGetXrayConfig_OmitsVisionFlowOnIneligibleStream(t *testing.T) {
	cases := []struct {
		name     string
		stream   string
		settings string
	}{
		{
			"xdrive vlessenc", `{"network":"xdrive","security":"none","xdriveSettings":{"service":"local","remoteFolder":"/tmp/shared"}}`,
			`{"clients":[],"decryption":"` + goldenVlessDecryption + `"}`,
		},
		{"ws over tls", `{"network":"ws","security":"tls","wsSettings":{"path":"/"}}`, `{"clients":[],"decryption":"none"}`},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tag := "stale-flow"
			seedVlessInbound(t, tag, 43110+i, []model.Client{
				{Email: "vision@x", ID: "44444444-4444-4444-4444-444444444444", Enable: true, Flow: "xtls-rprx-vision"},
			})
			if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", tag).
				Updates(map[string]any{"stream_settings": tc.stream, "settings": tc.settings}).Error; err != nil {
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

// xray-core runs Vision over TLS/REALITY on RAW, or over VLESS encryption on any transport but XDRIVE.
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

// With an empty client list xray-core runs a Shadowsocks-2022 inbound as a single-user
// server, and the server key it then accepts alone is the first half of every client's password.
func TestShadowsocks2022WithoutEnabledClientsStaysLocked(t *testing.T) {
	setupSettingTestDB(t)
	key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	in := &model.Inbound{
		Tag: "ss-2022", Enable: true, Port: 43110, Protocol: model.Shadowsocks,
		Settings: `{"method":"2022-blake3-aes-256-gcm","password":"` + key + `","network":"tcp,udp","clients":[]}`,
	}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatalf("create shadowsocks inbound: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "only@x", Password: key, Enable: true}}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	disableClients(t, "only@x")

	onlyLocked := func(where string, clients any) {
		t.Helper()
		list, _ := clients.([]any)
		if len(list) != 1 {
			t.Fatalf("%s: clients = %v, want only the locked placeholder", where, clients)
		}
		if email, _ := list[0].(map[string]any)["email"].(string); email != xray.LockedShadowsocks2022Email {
			t.Fatalf("%s: client %v served, want only the locked placeholder", where, list[0])
		}
	}
	emitted, _ := emittedClients(t, in.Tag)
	onlyLocked("config", emitted)

	var stored model.Inbound
	if err := database.GetDB().First(&stored, in.Id).Error; err != nil {
		t.Fatalf("read inbound: %v", err)
	}
	runtimeInbound, err := (&InboundService{}).buildInboundForLocalRuntime(database.GetDB(), &stored)
	if err != nil {
		t.Fatalf("buildInboundForLocalRuntime: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(runtimeInbound.Settings), &settings); err != nil {
		t.Fatalf("unmarshal runtime settings: %v", err)
	}
	onlyLocked("local runtime", settings["clients"])
}
