package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestSeedClientsFromInboundJSON_IsIdempotentAgainstExistingClients(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
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
