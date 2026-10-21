package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Legacy JSON of either serialization style must remain discoverable after
// the supported ClientsTable startup migration (#5805, #6886).
func TestGetClientTrafficTgBot_SettingsSerializationStyles(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	db := database.GetDB()

	const tgId int64 = 123456789
	cases := []struct {
		name     string
		settings string
		email    string
		port     int
	}{
		{"compact", `{"clients":[{"id":"u1","email":"compact-user","tgId":123456789}]}`, "compact-user", 41001},
		{"spaced", `{"clients": [{"id": "u2", "email": "spaced-user", "tgId": 123456789}]}`, "spaced-user", 41002},
		{"indented", `{
			"clients": [
				{"id": "u3", "email": "indented-user", "tgId": 123456789}
			]
		}`, "indented-user", 41003},
	}
	for _, c := range cases {
		inbound := &model.Inbound{UserId: 1, Tag: "tg-" + c.name, Enable: true, Port: c.port, Protocol: model.VLESS, Settings: c.settings}
		if err := db.Create(inbound).Error; err != nil {
			t.Fatalf("create %s inbound: %v", c.name, err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: c.email, Enable: true, Up: 10, Down: 20}).Error; err != nil {
			t.Fatalf("create %s client_traffics: %v", c.name, err)
		}
		if c.name == "indented" {
			// The existing migration also repairs a missing link for an existing record.
			if err := db.Create(&model.ClientRecord{Email: c.email, UUID: "u3", TgID: tgId}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	// Model an upgrade from JSON-only storage, before the ClientsTable seeder.
	if err := db.Where("seeder_name = ?", "ClientsTable").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	var records []model.ClientRecord
	if err := database.GetDB().Where("tg_id = ?", tgId).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("ClientsTable migration created %d records, want 3", len(records))
	}

	svc := InboundService{}
	traffics, err := svc.GetClientTrafficTgBot(tgId)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot: %v", err)
	}
	got := make(map[string]bool, len(traffics))
	for _, tr := range traffics {
		got[tr.Email] = true
	}
	if len(traffics) != 3 || !got["compact-user"] || !got["spaced-user"] || !got["indented-user"] {
		t.Fatalf("expected traffic for compact-user, spaced-user and indented-user, got %v", got)
	}

	other, err := svc.GetClientTrafficTgBot(42)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot(42): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("expected no traffic for unknown tgId, got %d rows", len(other))
	}
}
