package database

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const legacyHopFinalmask = `{"quicParams":{"udpHop":{"ports":"20000-30000","interval":"5-10"}}}`

const upgradedHopFinalmask = `{"udp":[{"settings":{"interval":"5-10","mode":"intervalRemote","remotePorts":"20000-30000"},"type":"udphop"}]}`

func assertHopUpgraded(t *testing.T, where string, finalmask any) {
	t.Helper()
	got, err := json.Marshal(finalmask)
	if err != nil {
		t.Fatalf("%s: marshal finalmask: %v", where, err)
	}
	if string(got) != upgradedHopFinalmask {
		t.Fatalf("%s: finalmask\n got: %s\nwant: %s", where, got, upgradedHopFinalmask)
	}
}

// xray-core 26.9.9 ignores quicParams.udpHop, so every client-side finalmask the panel
// stored before then hops nowhere; inbounds keep it, since udphop refuses to run there.
func TestUDPHopSeederMovesClientHopsToTheMask(t *testing.T) {
	initMigrateDB(t)
	inboundStream := `{"network":"hysteria","security":"tls","finalmask":` + legacyHopFinalmask + `}`
	ib := seedInboundWithStream(t, "hy-in", 8443, inboundStream)
	host := &model.Host{InboundId: ib.Id, Remark: "h", Address: "cdn.example.com", Port: 8443, FinalMask: legacyHopFinalmask}
	if err := GetDB().Create(host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	seedTemplate(t, `{"inbounds":[{"tag":"tpl-in","streamSettings":{"finalmask":`+legacyHopFinalmask+`}}],
		"outbounds":[{"protocol":"hysteria","tag":"hy-out","settings":{},"streamSettings":{"network":"hysteria","finalmask":`+legacyHopFinalmask+`}}]}`)
	if err := GetDB().Create(&model.Setting{Key: "subJsonFinalMask", Value: legacyHopFinalmask}).Error; err != nil {
		t.Fatalf("seed subJsonFinalMask: %v", err)
	}
	sub := &model.OutboundSubscription{
		Remark: "donor", Url: "https://donor.example.com/sub",
		LastFetchedOutbounds: `[{"protocol":"hysteria","tag":"sub-1","settings":{},"streamSettings":{"network":"hysteria","finalmask":` + legacyHopFinalmask + `}}]`,
	}
	if err := GetDB().Create(sub).Error; err != nil {
		t.Fatalf("create outbound subscription: %v", err)
	}
	if err := GetDB().Where("seeder_name = ?", "UDPHopClientMaskFix").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}

	var storedHost model.Host
	if err := GetDB().First(&storedHost, host.Id).Error; err != nil {
		t.Fatalf("reload host: %v", err)
	}
	var hostMask any
	_ = json.Unmarshal([]byte(storedHost.FinalMask), &hostMask)
	assertHopUpgraded(t, "host finalMask", hostMask)

	var template struct {
		Inbounds []struct {
			StreamSettings struct {
				Finalmask json.RawMessage `json:"finalmask"`
			} `json:"streamSettings"`
		} `json:"inbounds"`
		Outbounds []struct {
			StreamSettings struct {
				Finalmask any `json:"finalmask"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(storedTemplate(t)), &template); err != nil || len(template.Outbounds) != 1 || len(template.Inbounds) != 1 {
		t.Fatalf("stored template unreadable (%v)", err)
	}
	assertHopUpgraded(t, "template outbound", template.Outbounds[0].StreamSettings.Finalmask)
	var inboundMask any
	_ = json.Unmarshal(template.Inbounds[0].StreamSettings.Finalmask, &inboundMask)
	var legacy any
	_ = json.Unmarshal([]byte(legacyHopFinalmask), &legacy)
	want, _ := json.Marshal(legacy)
	if got, _ := json.Marshal(inboundMask); string(got) != string(want) {
		t.Fatalf("template inbound finalmask = %s, want it untouched", got)
	}

	var subMask model.Setting
	if err := GetDB().Where("key = ?", "subJsonFinalMask").First(&subMask).Error; err != nil {
		t.Fatalf("reload subJsonFinalMask: %v", err)
	}
	var subFinalmask any
	_ = json.Unmarshal([]byte(subMask.Value), &subFinalmask)
	assertHopUpgraded(t, "subJsonFinalMask", subFinalmask)

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
	assertHopUpgraded(t, "cached subscription outbound", cached[0].StreamSettings.Finalmask)

	var stored model.Inbound
	if err := GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if stored.StreamSettings != inboundStream {
		t.Fatalf("inbound stream = %s, want it untouched", stored.StreamSettings)
	}
}
