package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestDedupeInboundSettingsClients_CollapsesDuplicateEmails covers the #5770
// repair: settings.clients arrays written by older builds can carry the same
// email several times; startup must collapse them to the first occurrence and
// leave clean inbounds byte-for-byte untouched.
func TestDedupeInboundSettingsClients_CollapsesDuplicateEmails(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	dupSettings := `{"clients": [` +
		`{"id": "u1", "email": "dup@x", "subId": "s1", "enable": true},` +
		`{"id": "u2", "email": "keep@x", "subId": "s2", "enable": true},` +
		`{"id": "u1", "email": "dup@x", "subId": "s1", "enable": true},` +
		`{"id": "u1", "email": "dup@x", "subId": "s1", "enable": true}]}`
	dirty := model.Inbound{UserId: 1, Port: 21001, Protocol: model.VLESS, Tag: "dedupe-dirty", Settings: dupSettings}
	if err := db.Create(&dirty).Error; err != nil {
		t.Fatalf("create dirty inbound: %v", err)
	}

	cleanSettings := `{"clients": [{"id": "u3", "email": "solo@x", "subId": "s3", "enable": true}]}`
	clean := model.Inbound{UserId: 1, Port: 21002, Protocol: model.VLESS, Tag: "dedupe-clean", Settings: cleanSettings}
	if err := db.Create(&clean).Error; err != nil {
		t.Fatalf("create clean inbound: %v", err)
	}

	if err := dedupeInboundSettingsClients(); err != nil {
		t.Fatalf("dedupeInboundSettingsClients: %v", err)
	}

	var gotDirty model.Inbound
	if err := db.First(&gotDirty, dirty.Id).Error; err != nil {
		t.Fatalf("reload dirty inbound: %v", err)
	}
	var parsed struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(gotDirty.Settings), &parsed); err != nil {
		t.Fatalf("parse repaired settings: %v", err)
	}
	if len(parsed.Clients) != 2 {
		t.Fatalf("expected 2 clients after dedupe, got %d: %s", len(parsed.Clients), gotDirty.Settings)
	}
	if parsed.Clients[0]["email"] != "dup@x" || parsed.Clients[1]["email"] != "keep@x" {
		t.Fatalf("expected first occurrences [dup@x keep@x], got %v", parsed.Clients)
	}

	var gotClean model.Inbound
	if err := db.First(&gotClean, clean.Id).Error; err != nil {
		t.Fatalf("reload clean inbound: %v", err)
	}
	if gotClean.Settings != cleanSettings {
		t.Fatalf("clean inbound settings were rewritten:\nbefore: %s\nafter:  %s", cleanSettings, gotClean.Settings)
	}

	if err := dedupeInboundSettingsClients(); err != nil {
		t.Fatalf("second dedupe run: %v", err)
	}
	var again model.Inbound
	if err := db.First(&again, dirty.Id).Error; err != nil {
		t.Fatalf("reload after second run: %v", err)
	}
	if again.Settings != gotDirty.Settings {
		t.Fatal("dedupe is not idempotent: settings changed on the second run")
	}
}
