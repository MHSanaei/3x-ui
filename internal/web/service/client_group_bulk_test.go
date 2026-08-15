package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAddToGroupReportsOnlyChangedRecordsIncludingNull(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	if err := db.Create(&model.ClientGroup{Name: "paid"}).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	rows := []model.ClientRecord{
		{Email: "same@example", UUID: "same", Group: "paid"},
		{Email: "other@example", UUID: "other", Group: "free"},
		{Email: "null@example", UUID: "null"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "null@example").UpdateColumn("group_name", nil).Error; err != nil {
		t.Fatalf("set NULL group: %v", err)
	}

	got, err := (&ClientService{}).AddToGroup([]string{"same@example", "other@example", "null@example", "missing@example"}, "paid")
	if err != nil {
		t.Fatalf("AddToGroup: %v", err)
	}
	if got != 2 {
		t.Fatalf("affected = %d, want 2 changed records", got)
	}
	got, err = (&ClientService{}).AddToGroup([]string{"same@example", "other@example", "null@example"}, "paid")
	if err != nil {
		t.Fatalf("second AddToGroup: %v", err)
	}
	if got != 0 {
		t.Fatalf("second affected = %d, want 0", got)
	}
}
