package database

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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
