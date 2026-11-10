package service

import (
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"

	"gorm.io/gorm"
)

func countSettingWrites(t *testing.T) *atomic.Int32 {
	t.Helper()
	var writes atomic.Int32
	count := func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			writes.Add(1)
		}
	}
	db := database.GetDB()
	const hook = "test:count_setting_writes"
	if err := db.Callback().Create().After("gorm:create").Register(hook, count); err != nil {
		t.Fatalf("register create hook: %v", err)
	}
	if err := db.Callback().Update().After("gorm:update").Register(hook, count); err != nil {
		t.Fatalf("register update hook: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(hook)
		_ = db.Callback().Update().Remove(hook)
	})
	return &writes
}

// The persisted GUID equals the in-process default until the next restart, and
// comparing against that default rewrote the row on every read (status tick, heartbeat).
func TestGetPanelGuidPersistsOnceThenOnlyReads(t *testing.T) {
	setupSettingTestDB(t)
	writes := countSettingWrites(t)
	s := &SettingService{}

	first, err := s.GetPanelGuid()
	if err != nil {
		t.Fatalf("first GetPanelGuid: %v", err)
	}
	if got := writes.Load(); got != 1 {
		t.Fatalf("first read wrote settings %d times, want 1", got)
	}

	for range 3 {
		again, err := s.GetPanelGuid()
		if err != nil {
			t.Fatalf("repeat GetPanelGuid: %v", err)
		}
		if again != first {
			t.Fatalf("panel GUID changed from %q to %q", first, again)
		}
	}
	if got := writes.Load(); got != 1 {
		t.Fatalf("repeat reads wrote settings %d times in total, want 1", got)
	}
}
