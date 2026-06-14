package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AutoMigrate must create the hot-path indexes added for client group filters
// and client_traffics inbound lookups. gorm creates missing indexes on migrate,
// so this also protects existing DBs after upgrade.
func TestAutoMigrateCreatesHotPathIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClientRecord{}, &xray.ClientTraffic{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	cases := []struct {
		model any
		index string
	}{
		{&model.ClientRecord{}, "idx_client_record_group"},
		{&xray.ClientTraffic{}, "idx_client_traffics_inbound"},
	}
	for _, c := range cases {
		if !db.Migrator().HasIndex(c.model, c.index) {
			t.Errorf("expected index %q to exist after AutoMigrate", c.index)
		}
	}
}
