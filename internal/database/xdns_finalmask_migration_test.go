package database

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const legacyXdnsFinalmask = `{"udp":[{"type":"xdns","settings":{"domains":["t.example.com"]}}]}`

// assertXdnsUpgraded fails unless the finalmask's xdns domains are objects, the only
// shape xray-core 26.9.30 parses.
func assertXdnsUpgraded(t *testing.T, where string, finalmask any) {
	t.Helper()
	fm, _ := finalmask.(map[string]any)
	udp, _ := fm["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("%s: udp masks = %v, want one", where, fm["udp"])
	}
	mask, _ := udp[0].(map[string]any)
	settings, _ := mask["settings"].(map[string]any)
	domains, _ := settings["domains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("%s: domains = %v, want one entry", where, settings["domains"])
	}
	domain, ok := domains[0].(map[string]any)
	if !ok || domain["name"] != "t.example.com" {
		t.Fatalf("%s: domain = %#v, want an object named t.example.com", where, domains[0])
	}
}

func TestXdnsFinalmaskSeederUpgradesEveryStoredMask(t *testing.T) {
	initMigrateDB(t)
	ib := seedInboundWithStream(t, "xdns-in", 5353,
		`{"network":"kcp","security":"none","finalmask":`+legacyXdnsFinalmask+`}`)
	host := &model.Host{InboundId: ib.Id, Remark: "h", Address: "cdn.example.com", Port: 53, FinalMask: legacyXdnsFinalmask}
	if err := GetDB().Create(host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	seedTemplate(t, `{"outbounds":[{"protocol":"vless","tag":"dns-tunnel","settings":{},"streamSettings":{"network":"kcp","finalmask":`+legacyXdnsFinalmask+`}}]}`)
	if err := GetDB().Create(&model.Setting{Key: "subJsonFinalMask", Value: legacyXdnsFinalmask}).Error; err != nil {
		t.Fatalf("seed subJsonFinalMask: %v", err)
	}
	sub := &model.OutboundSubscription{
		Remark: "donor", Url: "https://donor.example.com/sub",
		LastFetchedOutbounds: `[{"protocol":"vless","tag":"sub-1","settings":{},"streamSettings":{"network":"kcp","finalmask":` + legacyXdnsFinalmask + `}}]`,
	}
	if err := GetDB().Create(sub).Error; err != nil {
		t.Fatalf("create outbound subscription: %v", err)
	}
	if err := GetDB().Where("seeder_name = ?", "XdnsFinalmaskObjectsFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}

	var stored model.Inbound
	if err := GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(stored.StreamSettings), &stream); err != nil {
		t.Fatalf("inbound stream is not JSON: %v", err)
	}
	assertXdnsUpgraded(t, "inbound stream", stream["finalmask"])

	var storedHost model.Host
	if err := GetDB().First(&storedHost, host.Id).Error; err != nil {
		t.Fatalf("reload host: %v", err)
	}
	var hostMask any
	if err := json.Unmarshal([]byte(storedHost.FinalMask), &hostMask); err != nil {
		t.Fatalf("host finalMask is not JSON: %v", err)
	}
	assertXdnsUpgraded(t, "host finalMask", hostMask)

	var template struct {
		Outbounds []struct {
			StreamSettings struct {
				Finalmask any `json:"finalmask"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(storedTemplate(t)), &template); err != nil || len(template.Outbounds) != 1 {
		t.Fatalf("stored template unreadable (%v)", err)
	}
	assertXdnsUpgraded(t, "template outbound", template.Outbounds[0].StreamSettings.Finalmask)

	var subMask model.Setting
	if err := GetDB().Where("key = ?", "subJsonFinalMask").First(&subMask).Error; err != nil {
		t.Fatalf("reload subJsonFinalMask: %v", err)
	}
	var subFinalmask any
	if err := json.Unmarshal([]byte(subMask.Value), &subFinalmask); err != nil {
		t.Fatalf("subJsonFinalMask is not JSON: %v", err)
	}
	assertXdnsUpgraded(t, "subJsonFinalMask", subFinalmask)

	var storedSub model.OutboundSubscription
	if err := GetDB().First(&storedSub, sub.Id).Error; err != nil {
		t.Fatalf("reload outbound subscription: %v", err)
	}
	var cached []struct {
		StreamSettings struct {
			Finalmask any `json:"finalmask"`
		} `json:"streamSettings"`
	}
	if err := json.Unmarshal([]byte(storedSub.LastFetchedOutbounds), &cached); err != nil || len(cached) != 1 {
		t.Fatalf("cached subscription outbounds unreadable (%v)", err)
	}
	assertXdnsUpgraded(t, "cached subscription outbound", cached[0].StreamSettings.Finalmask)
}
