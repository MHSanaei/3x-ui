package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func amneziawgnetEgressPortForTest() int { return amneziawgnet.EgressBasePort }

func wgKeypairForTest() (priv, pub string, err error) {
	return wgutil.GenerateWireguardKeypair()
}

func makeAWGOutboundConfig(t *testing.T) *xray.Config {
	t.Helper()
	cfg := &xray.Config{}
	err := json.Unmarshal([]byte(`{
		"outbounds": [
			{"protocol": "freedom", "tag": "direct"},
			{"protocol": "amneziawg", "tag": "awg-hop", "settings": {"secretKey": "x"}}
		]
	}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestTransformAmneziaWGOutbounds(t *testing.T) {
	cfg := makeAWGOutboundConfig(t)
	transformAmneziaWGOutbounds(cfg)

	var outbounds []struct {
		Protocol string `json:"protocol"`
		Tag      string `json:"tag"`
		Settings struct {
			Addr string `json:"addr"`
			Port int    `json:"port"`
			User string `json:"user"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("outbound count = %d, want 2 (no additions or drops)", len(outbounds))
	}
	if outbounds[0].Protocol != "freedom" || outbounds[0].Tag != "direct" {
		t.Fatalf("first outbound disturbed: %+v", outbounds[0])
	}
	got := outbounds[1]
	if got.Protocol != "socks" {
		t.Fatalf("amneziawg outbound not swapped to socks: %q", got.Protocol)
	}
	if got.Tag != "awg-hop" {
		t.Fatalf("tag not preserved: %q", got.Tag)
	}
	if got.Settings.Addr != "127.0.0.1" {
		t.Fatalf("bridge addr = %q, want 127.0.0.1", got.Settings.Addr)
	}
	if got.Settings.Port != amneziawgnetEgressPortForTest() {
		t.Fatalf("bridge port = %d", got.Settings.Port)
	}
	if got.Settings.User != "awg-hop" {
		t.Fatalf("SOCKS username = %q, want the outbound tag", got.Settings.User)
	}
}

func TestTransformAmneziaWGOutbounds_NoopWithoutAWG(t *testing.T) {
	before := &xray.Config{}
	if err := json.Unmarshal([]byte(`{"outbounds":[{"protocol":"freedom","tag":"direct"}]}`), before); err != nil {
		t.Fatal(err)
	}
	cfg := &xray.Config{}
	if err := json.Unmarshal(before.OutboundConfigs, &cfg.OutboundConfigs); err != nil {
		t.Fatal(err)
	}
	orig := json_util.RawMessage(append([]byte(nil), cfg.OutboundConfigs...))
	transformAmneziaWGOutbounds(cfg)
	if string(cfg.OutboundConfigs) != string(orig) {
		t.Fatalf("config without amneziawg outbounds must stay byte-identical:\nbefore=%s\nafter=%s", orig, cfg.OutboundConfigs)
	}
}

func TestCheckXrayConfig_AcceptsValidAWGOutbound(t *testing.T) {
	// A syntactically valid AWG outbound must pass panel-side validation --
	// the Xray-core loader would reject the unknown protocol outright.
	priv, pub, err := wgKeypairForTest()
	if err != nil {
		t.Fatal(err)
	}
	template := `{
		"outbounds": [{
			"protocol": "amneziawg",
			"tag": "awg-hop",
			"settings": {
				"mtu": 1420,
				"secretKey": "` + priv + `",
				"address": ["10.8.0.2/32"],
				"jc": 4, "jmin": 40, "jmax": 100, "s1": 15, "s2": 80, "s3": 12, "s4": 12,
				"h1": "100-800", "h2": "900-1600", "h3": "1700-2400", "h4": "2500-3200",
				"peers": [{
					"publicKey": "` + pub + `",
					"allowedIPs": ["0.0.0.0/0"],
					"endpoint": "203.0.113.7:51820",
					"keepAlive": 25
				}]
			}
		}]
	}`
	svc := &XraySettingService{}
	if err := svc.CheckXrayConfig(template); err != nil {
		t.Fatalf("valid amneziawg outbound rejected: %v", err)
	}
}

func TestCheckXrayConfig_RejectsBrokenAWGOutbound(t *testing.T) {
	template := `{
		"outbounds": [{
			"protocol": "amneziawg",
			"tag": "awg-bad",
			"settings": {
				"secretKey": "not-a-key",
				"address": ["10.8.0.2/32"],
				"peers": [{"publicKey": "alsobad", "allowedIPs": ["0.0.0.0/0"], "endpoint": "203.0.113.7:51820"}]
			}
		}]
	}`
	svc := &XraySettingService{}
	if err := svc.CheckXrayConfig(template); err == nil {
		t.Fatal("broken amneziawg outbound accepted")
	}
}
