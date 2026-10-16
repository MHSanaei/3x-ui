package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestInjectPsiphonOutbounds(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	db := database.GetDB()
	_ = db.Create(&model.PsiphonInstance{
		Id:          "US",
		CountryCode: "US",
		SocksPort:   40001,
		Enabled:     true,
	})
	_ = db.Create(&model.PsiphonInstance{
		Id:          "DE",
		CountryCode: "DE",
		SocksPort:   40002,
		Enabled:     true,
	})
	_ = db.Create(&model.PsiphonInstance{
		Id:          "JP",
		CountryCode: "JP",
		SocksPort:   40003,
		Enabled:     false,
	})

	cfg := &xray.Config{
		OutboundConfigs: json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"blackhole","tag":"blocked"}]`),
		RouterConfig:    json_util.RawMessage(`{"rules":[],"balancers":[]}`),
	}

	injectPsiphonOutbounds(cfg)

	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds failed: %v", err)
	}

	tags := make(map[string]map[string]any)
	for _, o := range outbounds {
		if tag, ok := o["tag"].(string); ok {
			tags[tag] = o
		}
	}

	if _, ok := tags["psiphon-US"]; !ok {
		t.Errorf("missing psiphon-US outbound")
	}
	if _, ok := tags["psiphon-DE"]; !ok {
		t.Errorf("missing psiphon-DE outbound")
	}
	if _, ok := tags["psiphon-JP"]; ok {
		t.Errorf("disabled instance psiphon-JP should not be injected")
	}

	us := tags["psiphon-US"]
	if us["protocol"] != "socks" {
		t.Errorf("psiphon-US protocol = %v, want socks", us["protocol"])
	}

	var routing map[string]any
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatalf("unmarshal routing failed: %v", err)
	}

	balancers, _ := routing["balancers"].([]any)
	hasPsiphonBalancer := false
	for _, b := range balancers {
		bm, ok := b.(map[string]any)
		if ok && bm["tag"] == "psiphon-balancer" {
			hasPsiphonBalancer = true
			break
		}
	}
	if !hasPsiphonBalancer {
		t.Errorf("missing psiphon-balancer in routing")
	}

	rules, _ := routing["rules"].([]any)
	hasQuicBypass := false
	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if ok && rm["port"] == "443" && rm["network"] == "udp" && rm["outboundTag"] == "blocked" {
			hasQuicBypass = true
			break
		}
	}
	if !hasQuicBypass {
		t.Errorf("missing QUIC UDP 443 bypass rule")
	}
}
