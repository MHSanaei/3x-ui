package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/gary/dune/internal/database/model"
	"github.com/gary/dune/internal/xray"
)

// AutoMigrate must create the hot-path indexes backing the panel's keyed
// lookups (client group/credential/tg/ip-limit filters, traffic-by-inbound,
// and the email-keyed reads on the global/node traffic side tables). gorm
// creates missing indexes on migrate, so this also protects existing DBs after
// upgrade.
func TestAutoMigrateCreatesHotPathIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ClientRecord{},
		&xray.ClientTraffic{},
		&model.ClientGlobalTraffic{},
		&model.NodeClientTraffic{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	cases := []struct {
		model any
		index string
	}{
		{&model.ClientRecord{}, "idx_client_record_group"},
		{&model.ClientRecord{}, "idx_client_record_uuid"},
		{&model.ClientRecord{}, "idx_client_record_password"},
		{&model.ClientRecord{}, "idx_client_record_tg_id"},
		{&model.ClientRecord{}, "idx_client_record_limit_ip"},
		{&xray.ClientTraffic{}, "idx_client_traffics_inbound"},
		{&model.NodeClientTraffic{}, "idx_node_client_traffics_email"},
	}
	for _, c := range cases {
		if !db.Migrator().HasIndex(c.model, c.index) {
			t.Errorf("expected index %q to exist after AutoMigrate", c.index)
		}
	}
}

// ensurePerformanceIndexes creates the indexes that can't be expressed with
// GORM struct tags (the partial index on client_traffics.expiry_time) plus the
// plain enable/email indexes backing the disable/renew scans. The raw DDL must
// apply cleanly and be idempotent (CREATE INDEX IF NOT EXISTS).
func TestEnsurePerformanceIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&xray.ClientTraffic{}, &model.ClientGlobalTraffic{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// Run twice to prove idempotency (IF NOT EXISTS on an existing index).
	for i := 0; i < 2; i++ {
		if err := ensurePerformanceIndexes(db); err != nil {
			t.Fatalf("ensurePerformanceIndexes (pass %d): %v", i+1, err)
		}
	}

	cases := []struct {
		model any
		index string
	}{
		{&xray.ClientTraffic{}, "idx_ct_enable"},
		{&xray.ClientTraffic{}, "idx_ct_expiry_time"},
		{&model.ClientGlobalTraffic{}, "idx_cgt_email"},
	}
	for _, c := range cases {
		if !db.Migrator().HasIndex(c.model, c.index) {
			t.Errorf("expected index %q to exist after ensurePerformanceIndexes", c.index)
		}
	}
}
