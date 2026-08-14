package database

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUniqueSubIDUpgradeSeedsBeforeCreatingIndex(t *testing.T) {
	t.Setenv("XUI_DB_TYPE", "sqlite")
	t.Setenv(subIDEnforceEnv, "1")
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.AutoMigrate(&model.Inbound{}, &model.User{}); err != nil {
		t.Fatalf("migrate legacy inbound: %v", err)
	}
	if err := legacy.Create(&model.User{Username: "legacy-admin", Password: "legacy-password"}).Error; err != nil {
		t.Fatalf("create legacy user: %v", err)
	}
	settings := `{"clients":[{"email":"one@example.test","id":"u1","subId":"shared"},{"email":"two@example.test","id":"u2","subId":"shared"}]}`
	if err := legacy.Create(&model.Inbound{Tag: "legacy", Port: 443, Protocol: model.VLESS, Settings: settings}).Error; err != nil {
		t.Fatalf("create legacy inbound: %v", err)
	}
	legacySQL, _ := legacy.DB()
	_ = legacySQL.Close()

	if err := InitDB(path); err == nil {
		t.Fatal("upgrade with duplicate seeded sub_ids unexpectedly succeeded")
	}
	t.Cleanup(func() { _ = CloseDB() })
	var clients int64
	if err := db.Model(&model.ClientRecord{}).Where("sub_id = ?", "shared").Count(&clients).Error; err != nil {
		t.Fatalf("count seeded clients: %v", err)
	}
	if clients != 2 {
		t.Fatalf("seeded duplicate clients = %d, want 2 before index preflight", clients)
	}
	if db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("unique index was left behind after duplicate preflight failure")
	}
}

func freshUniqueSubIDDB(t *testing.T) {
	t.Helper()
	t.Setenv("XUI_DB_TYPE", "sqlite")
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
}

func TestUniqueSubID_PostgresRejectsWrongPredicate(t *testing.T) {
	dsn := os.Getenv("XUI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to a reachable Postgres to run this test")
	}
	pg, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	previous := db
	db = pg
	t.Cleanup(func() { db = previous })
	if err := pg.Migrator().DropTable(&model.ClientRecord{}); err != nil {
		t.Fatalf("drop clients: %v", err)
	}
	if err := pg.AutoMigrate(&model.ClientRecord{}); err != nil {
		t.Fatalf("migrate clients: %v", err)
	}
	t.Cleanup(func() { _ = pg.Migrator().DropTable(&model.ClientRecord{}) })
	if err := pg.Exec("DROP INDEX IF EXISTS " + subIDUniqueIndex).Error; err != nil {
		t.Fatalf("drop generated index: %v", err)
	}
	if err := pg.Exec("CREATE UNIQUE INDEX " + subIDUniqueIndex + " ON clients (sub_id) WHERE sub_id IS NULL").Error; err != nil {
		t.Fatalf("create wrong index: %v", err)
	}
	if err := createUniqueSubIDIndex(); err == nil {
		t.Fatal("Postgres same-named index with wrong predicate was accepted")
	}
	if err := pg.Exec("DROP INDEX " + subIDUniqueIndex).Error; err != nil {
		t.Fatalf("drop wrong index: %v", err)
	}
	t.Setenv(subIDEnforceEnv, "1")
	if err := createUniqueSubIDIndex(); err != nil {
		t.Fatalf("create valid Postgres index: %v", err)
	}
}

func TestUniqueSubID_GatedByEnv(t *testing.T) {
	freshUniqueSubIDDB(t)
	if db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("index must NOT be created without the enforce opt-in")
	}

	t.Setenv(subIDEnforceEnv, "1")
	if err := createUniqueSubIDIndex(); err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if !db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("index should be created once opted in")
	}
}

func TestUniqueSubID_IndexCreatedAndEnforced(t *testing.T) {
	t.Setenv(subIDEnforceEnv, "1")
	freshUniqueSubIDDB(t)
	if !db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("expected DB-003 unique sub_id index after init")
	}
	if err := db.Create(&model.ClientRecord{Email: "a", SubID: "dup", UUID: "ua"}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "b", SubID: "dup", UUID: "ub"}).Error; err == nil {
		t.Fatal("a second client with the same non-empty sub_id must violate the unique index")
	}
}

func TestUniqueSubID_EmptyNotConstrained(t *testing.T) {
	t.Setenv(subIDEnforceEnv, "1")
	freshUniqueSubIDDB(t)
	if err := db.Create(&model.ClientRecord{Email: "e1", SubID: "", UUID: "u1"}).Error; err != nil {
		t.Fatalf("first empty sub_id: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "e2", SubID: "", UUID: "u2"}).Error; err != nil {
		t.Fatalf("second empty sub_id must be allowed by the partial index: %v", err)
	}
}

func TestUniqueSubID_FailsClosedOnExistingDuplicates(t *testing.T) {
	freshUniqueSubIDDB(t)
	if err := db.Create(&model.ClientRecord{Email: "d1", SubID: "dup", UUID: "u1"}).Error; err != nil {
		t.Fatalf("seed dup 1: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: "d2", SubID: "dup", UUID: "u2"}).Error; err != nil {
		t.Fatalf("seed dup 2: %v", err)
	}
	t.Setenv(subIDEnforceEnv, "1")
	if err := createUniqueSubIDIndex(); err == nil {
		t.Fatal("must fail closed while duplicate non-empty sub_ids exist")
	}
	if db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("index must NOT be created while duplicates exist")
	}

	if err := db.Where("email = ?", "d2").Delete(&model.ClientRecord{}).Error; err != nil {
		t.Fatalf("resolve dup: %v", err)
	}
	if err := createUniqueSubIDIndex(); err != nil {
		t.Fatalf("create after resolve: %v", err)
	}
	if !db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("index should be created once duplicates are resolved")
	}
}

func TestUniqueSubID_RejectsWrongDefinition(t *testing.T) {
	for _, tc := range []struct {
		name string
		ddl  string
	}{
		{"non-unique-on-subid", "CREATE INDEX " + subIDUniqueIndex + " ON clients (sub_id)"},
		{"unique-on-email-pred-subid", "CREATE UNIQUE INDEX " + subIDUniqueIndex + " ON clients (email) WHERE sub_id <> ''"},
		{"unique-subid-not-partial", "CREATE UNIQUE INDEX " + subIDUniqueIndex + " ON clients (sub_id)"},
		{"unique-subid-wrong-predicate", "CREATE UNIQUE INDEX " + subIDUniqueIndex + " ON clients (sub_id) WHERE sub_id IS NULL"},
		{"composite-subid-email", "CREATE UNIQUE INDEX " + subIDUniqueIndex + " ON clients (sub_id, email) WHERE sub_id <> ''"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			freshUniqueSubIDDB(t)
			if err := db.Exec("DROP INDEX IF EXISTS " + subIDUniqueIndex).Error; err != nil {
				t.Fatalf("drop index: %v", err)
			}
			if err := db.Exec(tc.ddl).Error; err != nil {
				t.Fatalf("create squatting index: %v", err)
			}
			if err := createUniqueSubIDIndex(); err == nil {
				t.Fatalf("%s: a same-named index of the wrong shape must be rejected", tc.name)
			}
		})
	}
}

func TestUniqueSubID_Idempotent(t *testing.T) {
	t.Setenv(subIDEnforceEnv, "1")
	freshUniqueSubIDDB(t)
	for i := 0; i < 3; i++ {
		if err := createUniqueSubIDIndex(); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if !db.Migrator().HasIndex(&model.ClientRecord{}, subIDUniqueIndex) {
		t.Fatal("index should exist after repeated runs")
	}
}
