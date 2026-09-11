package database

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateInboundExcludeFromSubColumn(t *testing.T) {
	initMigrateDB(t)

	if !db.Migrator().HasColumn(&model.Inbound{}, "exclude_from_sub") {
		t.Fatal("exclude_from_sub column missing after InitDB")
	}
	if err := migrateInboundExcludeFromSubColumn(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
}
