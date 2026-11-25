package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSnapshotRevisionsDirectWritesAndRollback(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			driver := sqlite.Open(filepath.Join(t.TempDir(), "revisions.db"))
			if dialect == "postgres" {
				dsn := os.Getenv("XUI_TEST_PG_DSN")
				if dsn == "" {
					dsn = os.Getenv("XUI_DB_DSN")
				}
				if dsn == "" {
					t.Skip("set XUI_TEST_PG_DSN for PostgreSQL")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{Logger: logger.Discard})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := db.AutoMigrate(&model.Inbound{}, &model.ClientRecord{}); err != nil {
				t.Fatal(err)
			}
			legacy := model.Inbound{Tag: "revision-legacy"}
			if err := db.Create(&legacy).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Where("id = ?", legacy.Id).Delete(&model.Inbound{}) })
			if err := db.Exec("UPDATE inbounds SET snapshot_revision = '' WHERE id = ?", legacy.Id).Error; err != nil {
				t.Fatal(err)
			}
			if err := migrateSnapshotRevisions(db); err != nil {
				t.Fatal(err)
			}
			var migrated model.Inbound
			if err := db.First(&migrated, legacy.Id).Error; err != nil {
				t.Fatal(err)
			}
			if len(migrated.SnapshotRevision) != 32 {
				t.Fatalf("legacy row was not versioned: %q", migrated.SnapshotRevision)
			}
			ib := model.Inbound{Tag: "revision-test", Settings: `{"clients":[]}`}
			if err := db.Create(&ib).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Where("id = ?", ib.Id).Delete(&model.Inbound{}) })
			read := func() string {
				var row model.Inbound
				if err := db.First(&row, ib.Id).Error; err != nil {
					t.Fatal(err)
				}
				return row.SnapshotRevision
			}
			original := read()
			if len(original) != 32 {
				t.Fatalf("missing durable token %q", original)
			}
			if err := db.Exec("UPDATE inbounds SET up = ?, down = ? WHERE id = ?", 10, 20, ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			if got := read(); got != original {
				t.Fatal("usage invalidated structure")
			}
			if err := db.Exec("UPDATE inbounds SET settings = settings WHERE id = ?", ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			if got := read(); got != original {
				t.Fatal("no-op invalidated structure")
			}
			sentinel := errors.New("rollback")
			err = db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("UPDATE inbounds SET settings = ? WHERE id = ?", `{"clients":[{"email":"new"}]}`, ib.Id).Error; err != nil {
					return err
				}
				return sentinel
			})
			if !errors.Is(err, sentinel) || read() != original {
				t.Fatalf("rollback changed structure: %v", err)
			}
			if err := db.Exec("UPDATE inbounds SET settings = ? WHERE id = ?", `{"clients":[{"email":"new"}]}`, ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			changed := read()
			if changed == original {
				t.Fatal("direct SQL settings edit missed")
			}
			if err := migrateSnapshotRevisions(db); err != nil {
				t.Fatal(err)
			}
			if read() != changed {
				t.Fatal("reinstall lost durable revision")
			}
			client := model.ClientRecord{Email: "snapshot-revision-test", UUID: "a"}
			if err := db.Create(&client).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Where("id = ?", client.Id).Delete(&model.ClientRecord{}) })
			var record model.ClientRecord
			if err := db.First(&record, client.Id).Error; err != nil {
				t.Fatal(err)
			}
			before := record.SnapshotRevision
			if err := db.Model(&record).UpdateColumn("updated_at", 999).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&record, client.Id).Error; err != nil {
				t.Fatal(err)
			}
			if record.SnapshotRevision != before {
				t.Fatal("timestamp-only write invalidated structure")
			}
			if err := db.Exec("UPDATE clients SET enable = ? WHERE id = ?", false, client.Id).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&record, client.Id).Error; err != nil {
				t.Fatal(err)
			}
			if record.SnapshotRevision == before {
				t.Fatal("automatic lifecycle write missed")
			}
		})
	}
}
