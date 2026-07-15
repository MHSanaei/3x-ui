package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func initNaiveInjectDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func TestInjectNaiveOutbounds_ReplacesStub(t *testing.T) {
	initNaiveInjectDB(t)
	if err := database.GetDB().Create(&model.NaiveOutbound{Tag: "naive-a", ProxyURL: "https://user:pass@example.com:443", LocalPort: 30001, Enabled: true}).Error; err != nil {
		t.Fatalf("create naive outbound: %v", err)
	}
	cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(`[{"tag":"naive-a","protocol":"naive","settings":{"proxy":"https://user:pass@example.com:443"}},{"tag":"direct","protocol":"freedom"}]`)}
	if err := injectNaiveOutbounds(cfg); err != nil {
		t.Fatalf("injectNaiveOutbounds: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds: %v", err)
	}
	for _, outbound := range outbounds {
		if outbound["tag"] != "naive-a" {
			continue
		}
		if outbound["protocol"] != "socks" {
			t.Fatalf("protocol = %v", outbound["protocol"])
		}
		settings := outbound["settings"].(map[string]any)
		servers := settings["servers"].([]any)
		server := servers[0].(map[string]any)
		if server["port"] != float64(30001) {
			t.Fatalf("port = %v", server["port"])
		}
		return
	}
	t.Fatal("naive outbound not found after injection")
}

func TestInjectNaiveOutbounds_AppendsWhenMissing(t *testing.T) {
	initNaiveInjectDB(t)
	if err := database.GetDB().Create(&model.NaiveOutbound{Tag: "naive-b", ProxyURL: "https://user:pass@example.com:443", LocalPort: 30002, Enabled: true}).Error; err != nil {
		t.Fatalf("create naive outbound: %v", err)
	}
	cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(`[{"tag":"direct","protocol":"freedom"}]`)}
	if err := injectNaiveOutbounds(cfg); err != nil {
		t.Fatalf("injectNaiveOutbounds: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("unmarshal outbounds: %v", err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("len(outbounds) = %d", len(outbounds))
	}
}
