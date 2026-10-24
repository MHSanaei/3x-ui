package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupHostBindingDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
}

func TestUpdate_OmittedPreservesAndEmptyClearsHostBindings(t *testing.T) {
	setupHostBindingDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	rec := &model.ClientRecord{
		Email:             "hostbind@x",
		UUID:              "11111111-1111-1111-1111-111111111111",
		SubID:             "hostbind@x",
		Enable:            true,
		ClientHostRuleIds: []int{7},
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}

	omitted := rec.ToClient()
	omitted.ClientHostRuleIds = nil
	omitted.Comment = "touch"
	if _, err := svc.Update(inboundSvc, rec.Id, *omitted, 0); err != nil {
		t.Fatalf("Update omitted: %v", err)
	}
	got, err := svc.GetByID(rec.Id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ClientHostRuleIds == nil || len(got.ClientHostRuleIds) != 1 || got.ClientHostRuleIds[0] != 7 {
		t.Fatalf("omitted must preserve list, got %v", got.ClientHostRuleIds)
	}

	cleared := got.ToClient()
	cleared.ClientHostRuleIds = []int{}
	if _, err := svc.Update(inboundSvc, rec.Id, *cleared, 0); err != nil {
		t.Fatalf("Update clear: %v", err)
	}
	after, err := svc.GetByID(rec.Id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if after.ClientHostRuleIds == nil {
		t.Fatalf("empty must clear to non-nil empty, got nil")
	}
	if len(after.ClientHostRuleIds) != 0 {
		t.Fatalf("empty must clear list, got %v", after.ClientHostRuleIds)
	}
}
