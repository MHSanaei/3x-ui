package database

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestInitDB_GeneratesPerPanelSubscriptionPaths(t *testing.T) {
	pathPattern := regexp.MustCompile(`^/[0-9a-z]{16}/$`)
	loadPaths := func(dbPath string) map[string]string {
		t.Helper()
		if err := InitDB(dbPath); err != nil {
			t.Fatalf("InitDB failed: %v", err)
		}
		defer func() {
			if err := CloseDB(); err != nil {
				t.Errorf("CloseDB failed: %v", err)
			}
		}()

		keys := []string{"subPath", "subJsonPath", "subClashPath"}
		paths := make(map[string]string, len(keys))
		for _, key := range keys {
			var setting model.Setting
			if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
				t.Fatalf("read %s: %v", key, err)
			}
			if !pathPattern.MatchString(setting.Value) {
				t.Fatalf("%s = %q, want /<16 lowercase alphanumeric characters>/", key, setting.Value)
			}
			paths[key] = setting.Value
		}
		if paths["subPath"] == paths["subJsonPath"] || paths["subPath"] == paths["subClashPath"] || paths["subJsonPath"] == paths["subClashPath"] {
			t.Fatalf("subscription paths must be distinct: %v", paths)
		}
		return paths
	}

	firstDB := filepath.Join(t.TempDir(), "x-ui.db")
	first := loadPaths(firstDB)
	reloaded := loadPaths(firstDB)
	for key, firstPath := range first {
		if firstPath != reloaded[key] {
			t.Fatalf("%s changed after restart: %q, then %q", key, firstPath, reloaded[key])
		}
	}

	second := loadPaths(filepath.Join(t.TempDir(), "x-ui.db"))
	for key, firstPath := range first {
		if firstPath == second[key] {
			t.Fatalf("%s reused across panels: %q", key, firstPath)
		}
	}
}

// Every launcher but install.sh serves the panel on the database as InitDB left
// it, so a credential seeded here is one anyone on the network can log in with.
func TestInitDBLeavesNoGuessableCredential(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		t.Fatalf("read users: %v", err)
	}
	for _, u := range users {
		if crypto.CheckPasswordHash(u.Password, "admin") {
			t.Fatalf("fresh database holds user %q with the password admin", u.Username)
		}
	}
}

// The server prints what EnsureFirstUser returns as the only copy of the first
// login, so it must be the stored credential, and a restart must not mint another.
func TestEnsureFirstUserCreatesOneWorkingCredential(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	username, password, err := EnsureFirstUser()
	if err != nil {
		t.Fatalf("EnsureFirstUser: %v", err)
	}
	var stored model.User
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("read user: %v", err)
	}
	if stored.Username != username || password == "" || !crypto.CheckPasswordHash(stored.Password, password) {
		t.Fatalf("returned %q/%q, stored user %q does not log in with it", username, password, stored.Username)
	}

	againUser, againPass, err := EnsureFirstUser()
	if err != nil || againUser != "" || againPass != "" {
		t.Fatalf("second EnsureFirstUser = %q, %q, %v; want nothing created", againUser, againPass, err)
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("users = %d (%v), want 1", count, err)
	}
}

// Old releases drew the cookie-signing secret from a clock-seeded math/rand, and a
// stored value cannot tell its generator, so an upgrade must replace it exactly once.
func TestSessionSecretRotatesOnceOnUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	reopen := func() []string {
		t.Helper()
		if err := CloseDB(); err != nil {
			t.Fatalf("CloseDB: %v", err)
		}
		if err := InitDB(dbPath); err != nil {
			t.Fatalf("reopen InitDB: %v", err)
		}
		var secrets []string
		if err := db.Model(&model.Setting{}).Where("key = ?", "secret").Pluck("value", &secrets).Error; err != nil {
			t.Fatalf("read secret: %v", err)
		}
		return secrets
	}
	storeSecret := func(value string) {
		t.Helper()
		if err := db.Where("key = ?", "secret").Delete(&model.Setting{}).Error; err != nil {
			t.Fatalf("clear secret: %v", err)
		}
		if err := db.Create(&model.Setting{Key: "secret", Value: value}).Error; err != nil {
			t.Fatalf("store secret: %v", err)
		}
	}

	storeSecret("kept-on-a-fresh-install")
	if got := reopen(); len(got) != 1 || got[0] != "kept-on-a-fresh-install" {
		t.Fatalf("fresh install secret = %q after restart, want it kept", got)
	}

	storeSecret("rfBd56ti2SMtYvSgD5xAV0YU99zampta")
	if err := db.Where("seeder_name = ?", "SessionSecretRotation").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	if got := reopen(); len(got) != 0 {
		t.Fatalf("secret = %q after upgrade, want the pre-upgrade secret dropped", got)
	}
	storeSecret("drawn-after-upgrade")
	if got := reopen(); len(got) != 1 || got[0] != "drawn-after-upgrade" {
		t.Fatalf("secret = %q, want the rotated secret kept so restarts do not log everyone out", got)
	}
}

func TestSeedClientsFromInboundJSON_IsIdempotentAgainstExistingClients(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	settings, err := json.Marshal(map[string]any{
		"clients": []any{
			map[string]any{
				"id":      "ce8d33df-3a64-4f10-8f9b-91c3a8e0c001",
				"email":   "alice@example.com",
				"enable":  true,
				"flow":    "",
				"subId":   "alice-sub",
				"comment": "from-inbound-json",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := model.Inbound{
		UserId:   1,
		Port:     12345,
		Protocol: model.VLESS,
		Settings: string(settings),
		Tag:      "test-inbound",
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	preExisting := &model.ClientRecord{
		Email:   "alice@example.com",
		UUID:    "ce8d33df-3a64-4f10-8f9b-91c3a8e0c001",
		SubID:   "alice-sub",
		Enable:  true,
		Comment: "added-via-api",
	}
	if err := db.Create(preExisting).Error; err != nil {
		t.Fatalf("seed client row: %v", err)
	}

	if err := db.Where("seeder_name = ?", "ClientsTable").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear ClientsTable history: %v", err)
	}

	if err := seedClientsFromInboundJSON(); err != nil {
		t.Fatalf("seedClientsFromInboundJSON should be idempotent against existing rows, got: %v", err)
	}

	var count int64
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "alice@example.com").Count(&count).Error; err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if count != 1 {
		t.Fatalf("alice@example.com should resolve to exactly one row, got %d", count)
	}
}

func TestSeedClientsFromInboundJSON_KeepsTheManualSwitch(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	inbound := model.Inbound{
		UserId:   1,
		Port:     12345,
		Protocol: model.VLESS,
		Settings: `{"clients": [
			{"id": "ce8d33df-3a64-4f10-8f9b-91c3a8e0c001", "email": "paying", "enable": true},
			{"id": "ce8d33df-3a64-4f10-8f9b-91c3a8e0c002", "email": "banned", "enable": false},
			{"id": "ce8d33df-3a64-4f10-8f9b-91c3a8e0c003", "email": "no-switch"}
		]}`,
		Tag: "test-inbound",
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Where("seeder_name = ?", "ClientsTable").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear ClientsTable history: %v", err)
	}

	if err := seedClientsFromInboundJSON(); err != nil {
		t.Fatalf("seedClientsFromInboundJSON: %v", err)
	}

	for email, want := range map[string]bool{"paying": true, "banned": false, "no-switch": true} {
		var row model.ClientRecord
		if err := db.Where("email = ?", email).First(&row).Error; err != nil {
			t.Fatalf("read client %s: %v", email, err)
		}
		if row.Enable != want {
			t.Errorf("client %s migrated with enable = %v, want %v", email, row.Enable, want)
		}
	}
}

func TestNormalizeInboundClientSubId_FillsMissingAndPreservesExisting(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	settings, err := json.Marshal(map[string]any{
		"clients": []any{
			map[string]any{
				"id":    "00000000-0000-0000-0000-000000000001",
				"email": "missing-sub@example.com",
				"subId": "",
			},
			map[string]any{
				"id":    "00000000-0000-0000-0000-000000000002",
				"email": "no-sub-key@example.com",
			},
			map[string]any{
				"id":    "00000000-0000-0000-0000-000000000003",
				"email": "has-sub@example.com",
				"subId": "keep-me-1234",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := model.Inbound{
		UserId:   1,
		Port:     23456,
		Protocol: model.VLESS,
		Settings: string(settings),
		Tag:      "subid-fix-inbound",
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	if err := db.Where("seeder_name = ?", "InboundClientSubIdFix").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := normalizeInboundClientSubId(); err != nil {
		t.Fatalf("normalizeInboundClientSubId: %v", err)
	}

	var reloaded model.Inbound
	if err := db.First(&reloaded, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(reloaded.Settings), &parsed); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	clients, ok := parsed["clients"].([]any)
	if !ok || len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %v", parsed["clients"])
	}

	subIdPattern := regexp.MustCompile(`^[0-9a-z]{16}$`)
	for i := range 2 {
		obj := clients[i].(map[string]any)
		sub, _ := obj["subId"].(string)
		if !subIdPattern.MatchString(sub) {
			t.Fatalf("client %d: expected 16-char [0-9a-z] subId, got %q", i, sub)
		}
	}
	preserved := clients[2].(map[string]any)["subId"].(string)
	if preserved != "keep-me-1234" {
		t.Fatalf("expected existing subId preserved, got %q", preserved)
	}

	var historyCount int64
	if err := db.Model(&model.HistoryOfSeeders{}).Where("seeder_name = ?", "InboundClientSubIdFix").Count(&historyCount).Error; err != nil {
		t.Fatalf("count seeder history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("expected one InboundClientSubIdFix history row, got %d", historyCount)
	}
}

func TestNormalizeSettingPaths_RepairsLegacyValues(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	seed := []model.Setting{
		{Key: "subJsonPath", Value: "YIrCXJOOOL"},
		{Key: "subPath", Value: "/sub"},
		{Key: "subClashPath", Value: "clash/"},
		{Key: "webBasePath", Value: "/panel/"},
	}
	if err := db.Where("key IN ?", []string{"subPath", "subJsonPath", "subClashPath"}).Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("clear generated subscription paths: %v", err)
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed setting %s: %v", seed[i].Key, err)
		}
	}

	if err := normalizeSettingPaths(); err != nil {
		t.Fatalf("normalizeSettingPaths: %v", err)
	}

	want := map[string]string{
		"subJsonPath":  "/YIrCXJOOOL/",
		"subPath":      "/sub/",
		"subClashPath": "/clash/",
		"webBasePath":  "/panel/",
	}
	for key, expected := range want {
		var row model.Setting
		if err := db.Where("key = ?", key).First(&row).Error; err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		if row.Value != expected {
			t.Errorf("%s = %q, want %q", key, row.Value, expected)
		}
	}
}

// A padded group name was listed as its own group while every action on it hit the trimmed
// one, so the upgrade folds stored names onto their trimmed form.
func TestClientGroupNamesTrimmedOnUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	for email, group := range map[string]string{"a@x": "vip ", "b@x": "\tvip", "c@x": "vip"} {
		if err := db.Create(&model.ClientRecord{Email: email, Group: group, Enable: true}).Error; err != nil {
			t.Fatalf("seed %s: %v", email, err)
		}
	}
	if err := db.Where("seeder_name = ?", "ClientGroupNameTrim").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB: %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("reopen InitDB: %v", err)
	}
	var groups []string
	if err := db.Model(&model.ClientRecord{}).Distinct().Pluck("group_name", &groups).Error; err != nil {
		t.Fatalf("read groups: %v", err)
	}
	if len(groups) != 1 || groups[0] != "vip" {
		t.Fatalf("groups after upgrade = %q, want [vip]", groups)
	}
}

// Requests naming a subId no client owns stored HWID rows that nothing ever reads; the
// upgrade drops them and keeps the rows of real subscriptions.
func TestOwnerlessClientHwidsPurgedOnUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if err := db.Create(&model.ClientRecord{Email: "a@x", SubID: "owned", Enable: true}).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	for i, sub := range []string{"owned", "junk-1", "junk-2"} {
		if err := db.Create(&model.ClientHwid{SubID: sub, HwidHash: fmt.Sprintf("h%d", i), FirstSeen: 1, LastSeen: 1}).Error; err != nil {
			t.Fatalf("seed hwid %s: %v", sub, err)
		}
	}
	if err := db.Where("seeder_name = ?", "OwnerlessClientHwidPurge").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB: %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("reopen InitDB: %v", err)
	}
	var subs []string
	if err := db.Model(&model.ClientHwid{}).Pluck("sub_id", &subs).Error; err != nil {
		t.Fatalf("read hwids: %v", err)
	}
	if len(subs) != 1 || subs[0] != "owned" {
		t.Fatalf("hwid rows after upgrade = %q, want [owned]", subs)
	}
}

// The editor fills forwardedPorts from the record, and an emptied field now withdraws the
// forwards: a record left at its creation value would drop live forwards on the next save.
func TestClientForwardedPortsBackfilledOnUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if err := db.Create(&model.ClientRecord{Email: "fwd@x", Enable: true}).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "kept@x", Enable: true, ForwardedPorts: "9000"}).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.Inbound{
		Tag: "awg", Port: 51820, Protocol: model.AmneziaWG, Enable: true,
		Settings: `{"clients":[{"email":"fwd@x","forwardedPorts":"8022"},{"email":"kept@x","forwardedPorts":"9001"}]}`,
	}).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Where("seeder_name = ?", "ClientForwardedPortsBackfill").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB: %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("reopen InitDB: %v", err)
	}
	for email, want := range map[string]string{"fwd@x": "8022", "kept@x": "9000"} {
		var rec model.ClientRecord
		if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
			t.Fatalf("read %s: %v", email, err)
		}
		if rec.ForwardedPorts != want {
			t.Errorf("%s forwardedPorts = %q after upgrade, want %q", email, rec.ForwardedPorts, want)
		}
	}
}
