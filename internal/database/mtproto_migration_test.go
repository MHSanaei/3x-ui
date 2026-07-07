package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initMtprotoMigrationDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
}

func createMtprotoInbound(t *testing.T, remark string, port int, settings map[string]any) *model.Inbound {
	t.Helper()
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	in := &model.Inbound{
		UserId:   1,
		Remark:   remark,
		Port:     port,
		Protocol: model.MTProto,
		Settings: string(raw),
		Tag:      remark,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create mtproto inbound: %v", err)
	}
	return in
}

func clearSeederHistory(t *testing.T, name string) {
	t.Helper()
	if err := db.Where("seeder_name = ?", name).Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear %s history: %v", name, err)
	}
}

func TestStripMtprotoInboundSecretsRemovesDeadSecret(t *testing.T) {
	initMtprotoMigrationDB(t)
	in := createMtprotoInbound(t, "mt-mc", 8443, map[string]any{
		"fakeTlsDomain": "www.cloudflare.com",
		"secret":        "eedeadbeef7777772e636c6f7564666c6172652e636f6d",
		"clients": []any{
			map[string]any{"email": "alice", "secret": "eeaaaa7777772e636c6f7564666c6172652e636f6d", "enable": true, "subId": "s1"},
		},
	})
	clearSeederHistory(t, "StripMtprotoInboundSecrets")

	if err := stripMtprotoInboundSecrets(); err != nil {
		t.Fatalf("stripMtprotoInboundSecrets: %v", err)
	}

	settings := reloadInboundSettings(t, in.Id)
	if _, ok := settings["secret"]; ok {
		t.Fatalf("inbound-level secret must be removed, got %v", settings)
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("clients must be preserved, got %v", settings["clients"])
	}
	if clients[0].(map[string]any)["secret"] != "eeaaaa7777772e636c6f7564666c6172652e636f6d" {
		t.Fatalf("client secret must be preserved, got %v", clients[0])
	}
	if settings["fakeTlsDomain"] != "www.cloudflare.com" {
		t.Fatalf("fakeTlsDomain must be preserved, got %v", settings)
	}
}

func TestStripMtprotoInboundSecretsIsGated(t *testing.T) {
	initMtprotoMigrationDB(t)
	in := createMtprotoInbound(t, "mt-gate", 8444, map[string]any{
		"fakeTlsDomain": "a.com",
		"secret":        "eebeef61",
		"clients":       []any{map[string]any{"email": "x", "secret": "eeaa61", "enable": true}},
	})

	if err := stripMtprotoInboundSecrets(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, ok := reloadInboundSettings(t, in.Id)["secret"]; ok {
		t.Fatal("secret should be stripped on the first run")
	}

	var raw model.Inbound
	if err := db.First(&raw, in.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	raw.Settings = `{"fakeTlsDomain":"a.com","secret":"eereintroduced","clients":[]}`
	if err := db.Save(&raw).Error; err != nil {
		t.Fatalf("reintroduce secret: %v", err)
	}
	if err := stripMtprotoInboundSecrets(); err != nil {
		t.Fatalf("second run (history gate): %v", err)
	}
	if _, ok := reloadInboundSettings(t, in.Id)["secret"]; !ok {
		t.Fatal("second run must be a no-op: the seeder is one-time, so a re-added secret is left alone")
	}
}

func TestSeedThenStripPreservesLegacySecretOnClient(t *testing.T) {
	initMtprotoMigrationDB(t)
	const legacy = "eedeadbeefdeadbeefdeadbeefdeadbe7777772e636c6f7564666c6172652e636f6d"
	in := createMtprotoInbound(t, "mt-legacy", 8445, map[string]any{
		"fakeTlsDomain": "www.cloudflare.com",
		"secret":        legacy,
	})
	clearSeederHistory(t, "MtprotoSecretsToClients")
	clearSeederHistory(t, "StripMtprotoInboundSecrets")

	if err := seedMtprotoSecretsToClients(); err != nil {
		t.Fatalf("seedMtprotoSecretsToClients: %v", err)
	}
	if err := stripMtprotoInboundSecrets(); err != nil {
		t.Fatalf("stripMtprotoInboundSecrets: %v", err)
	}

	settings := reloadInboundSettings(t, in.Id)
	if _, ok := settings["secret"]; ok {
		t.Fatalf("inbound-level secret must be gone after seed+strip, got %v", settings)
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("seed should have created exactly one client, got %v", settings["clients"])
	}
	if got := clients[0].(map[string]any)["secret"]; got != legacy {
		t.Fatalf("the legacy secret must survive on the seeded client, got %v want %s", got, legacy)
	}
}
