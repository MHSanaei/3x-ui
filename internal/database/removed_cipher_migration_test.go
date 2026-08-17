package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestMigrateShadowsocksRemovedCiphers_RewritesNoneAndPlain covers the
// xray-core v26.7.11 removal of the shadowsocks "none"/"plain" ciphers: one
// such row makes the generated config unbuildable, so startup must rewrite
// both the inbound method and any per-client method to a supported cipher and
// leave a valid inbound untouched.
func TestMigrateShadowsocksRemovedCiphers_RewritesNoneAndPlain(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	removed := `{"method": "none", "clients": [{"email": "a@x", "password": "p", "method": "plain"}]}`
	dirty := model.Inbound{UserId: 1, Port: 31001, Protocol: model.Shadowsocks, Tag: "ss-removed", Settings: removed}
	if err := db.Create(&dirty).Error; err != nil {
		t.Fatalf("create dirty inbound: %v", err)
	}

	valid := `{"method": "aes-256-gcm", "clients": [{"email": "b@x", "password": "p"}]}`
	clean := model.Inbound{UserId: 1, Port: 31002, Protocol: model.Shadowsocks, Tag: "ss-valid", Settings: valid}
	if err := db.Create(&clean).Error; err != nil {
		t.Fatalf("create clean inbound: %v", err)
	}

	if err := migrateShadowsocksRemovedCiphers(); err != nil {
		t.Fatalf("migrateShadowsocksRemovedCiphers: %v", err)
	}

	var gotDirty model.Inbound
	if err := db.First(&gotDirty, dirty.Id).Error; err != nil {
		t.Fatalf("reload dirty inbound: %v", err)
	}
	var parsed struct {
		Method  string           `json:"method"`
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(gotDirty.Settings), &parsed); err != nil {
		t.Fatalf("parse repaired settings: %v", err)
	}
	if parsed.Method != "chacha20-ietf-poly1305" {
		t.Fatalf("expected inbound method rewritten, got %q", parsed.Method)
	}
	if parsed.Clients[0]["method"] != "chacha20-ietf-poly1305" {
		t.Fatalf("expected client method rewritten, got %v", parsed.Clients[0]["method"])
	}

	var gotClean model.Inbound
	if err := db.First(&gotClean, clean.Id).Error; err != nil {
		t.Fatalf("reload clean inbound: %v", err)
	}
	if gotClean.Settings != valid {
		t.Fatalf("valid inbound was rewritten:\nbefore: %s\nafter:  %s", valid, gotClean.Settings)
	}
}

// TestMigrateVmessRemovedSecurities_RewritesNoneAndZero covers the v26.7.11
// removal of vmess "none"/"zero" security values: startup rewrites them to
// "auto" on both the clients column and each vmess inbound's settings JSON.
func TestMigrateVmessRemovedSecurities_RewritesNoneAndZero(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	settings := `{"clients": [{"id": "u1", "email": "a@x", "security": "none"},` +
		`{"id": "u2", "email": "b@x", "security": "zero"},` +
		`{"id": "u3", "email": "c@x", "security": "aes-128-gcm"}]}`
	inbound := model.Inbound{UserId: 1, Port: 32001, Protocol: model.VMESS, Tag: "vmess-removed", Settings: settings}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create vmess inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "a@x", Security: "zero", Enable: true}).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}

	if err := migrateVmessRemovedSecurities(); err != nil {
		t.Fatalf("migrateVmessRemovedSecurities: %v", err)
	}

	var got model.Inbound
	if err := db.First(&got, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var parsed struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(got.Settings), &parsed); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if parsed.Clients[0]["security"] != "auto" || parsed.Clients[1]["security"] != "auto" {
		t.Fatalf("expected removed securities rewritten to auto, got %v", parsed.Clients)
	}
	if parsed.Clients[2]["security"] != "aes-128-gcm" {
		t.Fatalf("expected valid security untouched, got %v", parsed.Clients[2]["security"])
	}

	var rec model.ClientRecord
	if err := db.Where("email = ?", "a@x").First(&rec).Error; err != nil {
		t.Fatalf("reload client record: %v", err)
	}
	if rec.Security != "auto" {
		t.Fatalf("expected client record security rewritten to auto, got %q", rec.Security)
	}
}
