package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetRecordsByTgID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	db := database.GetDB()

	records := []model.ClientRecord{
		{Email: "alice@x", TgID: 100, SubID: "sa"},
		{Email: "bob@x", TgID: 100, SubID: "sb"},
		{Email: "carol@x", TgID: 200, SubID: "sc"},
		{Email: "dave@x", TgID: 0, SubID: "sd"},
	}
	for _, r := range records {
		if err := db.Create(&r).Error; err != nil {
			t.Fatalf("create record %q: %v", r.Email, err)
		}
	}

	t.Run("multiple clients share tgId", func(t *testing.T) {
		got, err := svc.GetRecordsByTgID(100)
		if err != nil {
			t.Fatalf("GetRecordsByTgID(100): %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 records, got %d", len(got))
		}
		emails := make(map[string]bool)
		for _, r := range got {
			emails[r.Email] = true
		}
		if !emails["alice@x"] || !emails["bob@x"] {
			t.Fatalf("expected alice@x and bob@x, got %v", got)
		}
	})

	t.Run("single client by tgId", func(t *testing.T) {
		got, err := svc.GetRecordsByTgID(200)
		if err != nil {
			t.Fatalf("GetRecordsByTgID(200): %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 record, got %d", len(got))
		}
		if got[0].Email != "carol@x" {
			t.Fatalf("expected carol@x, got %s", got[0].Email)
		}
	})

	t.Run("tgId zero returns own record", func(t *testing.T) {
		got, err := svc.GetRecordsByTgID(0)
		if err != nil {
			t.Fatalf("GetRecordsByTgID(0): %v", err)
		}
		if len(got) != 1 || got[0].Email != "dave@x" {
			t.Fatalf("expected dave@x for tgId=0, got %v", got)
		}
	})

	t.Run("nonexistent tgId returns empty", func(t *testing.T) {
		got, err := svc.GetRecordsByTgID(999)
		if err != nil {
			t.Fatalf("GetRecordsByTgID(999): %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 records, got %d", len(got))
		}
	})
}
