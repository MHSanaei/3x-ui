package naive

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func initInjectTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func TestInjectNaiveOutbounds_ReplacesNaiveProtocol(t *testing.T) {
	initInjectTestDB(t)
	if err := database.GetDB().Create(&model.NaiveOutbound{
		Tag:       "naive-test",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30001,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create naive outbound: %v", err)
	}

	cfg := &xray.Config{
		OutboundConfigs: json_util.RawMessage(`[
			{"tag":"naive-test","protocol":"naive","settings":{"proxy":"https://user:pass@example.com:443"}},
			{"tag":"direct","protocol":"freedom"}
		]`),
	}

	if err := InjectNaiveOutbounds(cfg); err != nil {
		t.Fatalf("InjectNaiveOutbounds: %v", err)
	}

	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds: %v", err)
	}

	for _, ob := range outbounds {
		if ob["tag"] != "naive-test" {
			continue
		}
		if ob["protocol"] != "socks" {
			t.Fatalf("expected protocol=socks, got %v", ob["protocol"])
		}
		settings := ob["settings"].(map[string]any)
		servers := settings["servers"].([]any)
		server := servers[0].(map[string]any)
		if server["port"] != float64(30001) {
			t.Fatalf("expected port=30001, got %v", server["port"])
		}
		if server["address"] != "127.0.0.1" {
			t.Fatalf("expected address=127.0.0.1, got %v", server["address"])
		}
		return
	}
	t.Fatal("naive outbound not found after injection")
}

func TestInjectNaiveOutbounds_AppendsWhenNotInConfig(t *testing.T) {
	initInjectTestDB(t)
	if err := database.GetDB().Create(&model.NaiveOutbound{
		Tag:       "naive-append",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30002,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create naive outbound: %v", err)
	}

	cfg := &xray.Config{
		OutboundConfigs: json_util.RawMessage(`[{"tag":"direct","protocol":"freedom"}]`),
	}

	if err := InjectNaiveOutbounds(cfg); err != nil {
		t.Fatalf("InjectNaiveOutbounds: %v", err)
	}

	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds: %v", err)
	}

	found := false
	for _, ob := range outbounds {
		if ob["tag"] == "naive-append" && ob["protocol"] == "socks" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected naive-append outbound to be appended as socks")
	}
}

func TestInjectNaiveOutbounds_EmptyConfig(t *testing.T) {
	initInjectTestDB(t)
	if err := database.GetDB().Create(&model.NaiveOutbound{
		Tag:       "naive-empty",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30004,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create naive outbound: %v", err)
	}

	cfg := &xray.Config{OutboundConfigs: nil}

	if err := InjectNaiveOutbounds(cfg); err != nil {
		t.Fatalf("InjectNaiveOutbounds: %v", err)
	}

	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds: %v", err)
	}

	if len(outbounds) != 1 {
		t.Fatalf("expected 1 outbound appended to empty config, got %d", len(outbounds))
	}
}

func TestBuildSocksOutbound(t *testing.T) {
	socks := BuildSocksOutbound("test-tag", 12345)

	if socks["tag"] != "test-tag" {
		t.Fatalf("expected tag=test-tag, got %v", socks["tag"])
	}
	if socks["protocol"] != "socks" {
		t.Fatalf("expected protocol=socks, got %v", socks["protocol"])
	}

	settings := socks["settings"].(map[string]any)
	servers := settings["servers"].([]map[string]any)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0]["address"] != "127.0.0.1" {
		t.Fatalf("expected address=127.0.0.1, got %v", servers[0]["address"])
	}
	if servers[0]["port"] != 12345 {
		t.Fatalf("expected port=12345, got %v", servers[0]["port"])
	}
}
