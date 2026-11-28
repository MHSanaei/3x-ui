package database

import (
	"path/filepath"
	"strings"
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
	if err := db.AutoMigrate(
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.Inbound{},
		&xray.ClientTraffic{},
		&model.ClientGlobalTraffic{},
		&model.ApiToken{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	cases := []struct {
		model any
		index string
	}{
		{&model.ClientRecord{}, "idx_client_record_group"},
		{&model.ClientInbound{}, "idx_client_inbounds_inbound_client"},
		{&model.Inbound{}, "idx_inbounds_user_id"},
		{&xray.ClientTraffic{}, "idx_client_traffics_inbound"},
		{&xray.ClientTraffic{}, "idx_client_traffics_renew"},
		{&model.ClientGlobalTraffic{}, "idx_client_global_email"},
		{&model.ClientGlobalTraffic{}, "idx_client_global_updated_at"},
		{&model.ApiToken{}, "idx_api_tokens_token"},
	}
	for _, c := range cases {
		if !db.Migrator().HasIndex(c.model, c.index) {
			t.Errorf("expected index %q to exist after AutoMigrate", c.index)
		}
	}
}

func TestMigrateHotPathIndexesIdempotentAndUsedByPlanner(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	checks := []struct {
		model any
		name  string
	}{
		{&model.ClientInbound{}, "idx_client_inbounds_inbound_client"},
		{&model.ClientGlobalTraffic{}, "idx_client_global_updated_at"},
		{&model.Inbound{}, "idx_inbounds_user_id"},
	}
	for _, c := range checks {
		if !db.Migrator().HasIndex(c.model, c.name) {
			t.Fatalf("%s missing after InitDB", c.name)
		}
	}

	// Dropping and re-running verifies upgrades over pre-existing databases.
	for _, c := range checks {
		if err := db.Exec("DROP INDEX IF EXISTS " + c.name).Error; err != nil {
			t.Fatalf("drop %s: %v", c.name, err)
		}
		if db.Migrator().HasIndex(c.model, c.name) {
			t.Fatalf("%s still present after DROP INDEX", c.name)
		}
	}
	if err := migrateHotPathIndexes(); err != nil {
		t.Fatalf("migrateHotPathIndexes restore: %v", err)
	}
	if err := migrateHotPathIndexes(); err != nil {
		t.Fatalf("migrateHotPathIndexes repeat: %v", err)
	}
	for _, c := range checks {
		if !db.Migrator().HasIndex(c.model, c.name) {
			t.Fatalf("%s missing after migrateHotPathIndexes", c.name)
		}
	}

	if IsPostgres() {
		return
	}
	plans := []struct {
		query string
		index string
	}{
		{
			query: "EXPLAIN QUERY PLAN SELECT client_id FROM client_inbounds WHERE inbound_id = 1",
			index: "idx_client_inbounds_inbound_client",
		},
		{
			query: "EXPLAIN QUERY PLAN SELECT EXISTS (SELECT 1 FROM client_global_traffics WHERE updated_at >= 1)",
			index: "idx_client_global_updated_at",
		},
	}
	for _, tc := range plans {
		var rows []struct{ Detail string }
		if err := db.Raw(tc.query).Scan(&rows).Error; err != nil {
			t.Fatalf("explain %q: %v", tc.query, err)
		}
		used := false
		for _, r := range rows {
			if strings.Contains(r.Detail, tc.index) {
				used = true
			}
		}
		if !used {
			t.Errorf("%q did not use %s: %+v", tc.query, tc.index, rows)
		}
	}
}
