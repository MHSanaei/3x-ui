package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func snapshotRevisionDDLDB(t testing.TB, dialect string) (*gorm.DB, func() *gorm.DB) {
	t.Helper()
	driver := sqlite.Open(filepath.Join(t.TempDir(), "ddl.db"))
	if dialect == DialectPostgres {
		dsn := os.Getenv("XUI_TEST_PG_DSN")
		if dsn == "" {
			dsn = os.Getenv("XUI_DB_DSN")
		}
		if dsn == "" {
			t.Skip("set XUI_TEST_PG_DSN for PostgreSQL DDL tests")
		}
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		pool, err := admin.DB()
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("revision_ddl_%d", time.Now().UnixNano())
		if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; _ = pool.Close() })
		driver = postgres.Open(dsn + " search_path=" + schema)
	}
	open := func() *gorm.DB {
		db, err := gorm.Open(driver, &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		pool, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	db := open()
	if err := db.AutoMigrate(&model.Inbound{}, &model.ClientRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateSnapshotRevisions(db); err != nil {
		t.Fatal(err)
	}
	return db, open
}

func TestSnapshotRevisionPostgresTypeDDL(t *testing.T) {
	db, _ := snapshotRevisionDDLDB(t, DialectPostgres)
	for _, sql := range []string{
		"ALTER TABLE inbounds ALTER COLUMN remark TYPE VARCHAR(1024)",
		"ALTER TABLE clients ALTER COLUMN comment TYPE VARCHAR(1024)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("revision trigger blocked DDL %q: %v", sql, err)
		}
	}
	if err := migrateSnapshotRevisions(db); err != nil {
		t.Fatal(err)
	}
	ib := model.Inbound{Tag: "after-ddl"}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err := db.Model(&model.Inbound{}).Select("snapshot_revision").Where("id = ?", ib.Id).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE inbounds SET remark = 'changed' WHERE id = ?", ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inbound{}).Select("snapshot_revision").Where("id = ?", ib.Id).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before == after || len(after) != 32 {
		t.Fatalf("DDL lost revision invalidation: %q -> %q", before, after)
	}
}

func TestSnapshotRevisionMigrationRecreationAndReopen(t *testing.T) {
	for _, dialect := range []string{DialectSQLite, DialectPostgres} {
		t.Run(dialect, func(t *testing.T) {
			db, reopen := snapshotRevisionDDLDB(t, dialect)
			if dialect == DialectSQLite {
				if err := db.Exec("PRAGMA recursive_triggers = ON").Error; err != nil {
					t.Fatal(err)
				}
			}
			ib := model.Inbound{Tag: "ddl-reopen"}
			if err := db.Create(&ib).Error; err != nil {
				t.Fatal(err)
			}
			read := func(conn *gorm.DB) string {
				var revision string
				if err := conn.Model(&model.Inbound{}).Select("snapshot_revision").Where("id = ?", ib.Id).Scan(&revision).Error; err != nil {
					t.Fatal(err)
				}
				return revision
			}
			original := read(db)
			for _, table := range []string{"inbounds", "clients"} {
				if err := db.Exec("ALTER TABLE " + table + " ADD COLUMN retired_snapshot_field TEXT").Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := dropSnapshotRevisionTriggers(db); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"inbounds", "clients"} {
				name := "snapshot_revision_" + table
				sql := fmt.Sprintf(`CREATE TRIGGER %s_update AFTER UPDATE OF retired_snapshot_field ON %s WHEN NEW.retired_snapshot_field IS NOT OLD.retired_snapshot_field BEGIN UPDATE %s SET snapshot_revision = lower(hex(randomblob(16))) WHERE id = NEW.id; END`, name, table, table)
				if dialect == DialectPostgres {
					sql = fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE OF retired_snapshot_field ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, name, table, name)
				}
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			migrate := func(tx *gorm.DB) error {
				if err := dropSnapshotRevisionTriggers(tx); err != nil {
					return err
				}
				for _, table := range []string{"inbounds", "clients"} {
					if err := tx.Exec("ALTER TABLE " + table + " DROP COLUMN retired_snapshot_field").Error; err != nil {
						return err
					}
				}
				return migrateSnapshotRevisions(tx)
			}
			rollback := fmt.Errorf("roll back DDL")
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := migrate(tx); err != nil {
					return err
				}
				return rollback
			}); !errors.Is(err, rollback) {
				t.Fatalf("DDL rollback: %v", err)
			}
			if !db.Migrator().HasColumn("inbounds", "retired_snapshot_field") || read(db) != original {
				t.Fatal("DDL rollback lost schema or revision")
			}
			if err := db.Transaction(migrate); err != nil {
				t.Fatalf("retired-column migration blocked: %v", err)
			}
			if db.Migrator().HasColumn("inbounds", "retired_snapshot_field") {
				t.Fatal("retired column survived migration")
			}
			if err := db.Exec("UPDATE inbounds SET remark = 'migrated' WHERE id = ?", ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			changed := read(db)
			if changed == original {
				t.Fatal("recreated trigger missed direct SQL")
			}
			pool, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			if err := pool.Close(); err != nil {
				t.Fatal(err)
			}
			db = reopen()
			if err := dropSnapshotRevisionTriggers(db); err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&model.Inbound{}, &model.ClientRecord{}); err != nil {
				t.Fatal(err)
			}
			if err := migrateSnapshotRevisions(db); err != nil {
				t.Fatal(err)
			}
			if read(db) != changed {
				t.Fatal("reopen/recreation changed durable revision")
			}
			if err := db.Exec("UPDATE inbounds SET up = up + 1, down = down + 1 WHERE id = ?", ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			if read(db) != changed {
				t.Fatal("counter-only update invalidated reopened catalog")
			}
			if err := db.Exec("UPDATE inbounds SET remark = 'after reopen' WHERE id = ?", ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			if read(db) == changed {
				t.Fatal("reopened trigger missed configuration change")
			}
		})
	}
}

func BenchmarkSnapshotRevisionSteadyState(b *testing.B) {
	for _, dialect := range []string{DialectSQLite, DialectPostgres} {
		b.Run(dialect, func(b *testing.B) {
			for _, mode := range []string{"scoped-baseline", "generic"} {
				b.Run(mode, func(b *testing.B) {
					db, _ := snapshotRevisionDDLDB(b, dialect)
					for _, table := range []string{"inbounds", "clients"} {
						if mode != "scoped-baseline" {
							continue
						}
						var definition string
						if dialect == DialectSQLite {
							if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?", "snapshot_revision_"+table+"_update").Scan(&definition).Error; err != nil {
								b.Fatal(err)
							}
						} else {
							if err := db.Raw("SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgname = ? AND tgrelid = ?::regclass", "snapshot_revision_"+table, table).Scan(&definition).Error; err != nil {
								b.Fatal(err)
							}
						}
						var value any = &model.Inbound{}
						if table == "clients" {
							value = &model.ClientRecord{}
						}
						stmt := &gorm.Statement{DB: db}
						if err := stmt.Parse(value); err != nil {
							b.Fatal(err)
						}
						var columns []string
						for _, col := range stmt.Schema.DBNames {
							if col == "snapshot_revision" || (table == "inbounds" && (col == "up" || col == "down")) || (table == "clients" && col == "updated_at") {
								continue
							}
							columns = append(columns, `"`+col+`"`)
						}
						definition = strings.Replace(definition, "UPDATE ON", "UPDATE OF "+strings.Join(columns, ",")+" ON", 1)
						drop := "DROP TRIGGER snapshot_revision_" + table + "_update"
						if dialect == DialectPostgres {
							drop = "DROP TRIGGER snapshot_revision_" + table + " ON " + table
						}
						if err := db.Exec(drop).Error; err != nil {
							b.Fatal(err)
						}
						if err := db.Exec(definition).Error; err != nil {
							b.Fatal(err)
						}
					}
					inbounds := make([]model.Inbound, 1000)
					clients := make([]model.ClientRecord, 1000)
					for i := range inbounds {
						inbounds[i] = model.Inbound{Tag: fmt.Sprintf("bench-%d", i), Settings: `{"clients":[]}`}
						clients[i] = model.ClientRecord{Email: fmt.Sprintf("bench-%d", i)}
					}
					if err := db.CreateInBatches(inbounds, 100).Error; err != nil {
						b.Fatal(err)
					}
					if err := db.CreateInBatches(clients, 100).Error; err != nil {
						b.Fatal(err)
					}
					for _, kind := range []string{"counters", "timestamps", "noop"} {
						b.Run(kind, func(b *testing.B) {
							statement := "UPDATE inbounds SET up = up + 1, down = down + 1"
							if kind == "timestamps" {
								statement = "UPDATE clients SET updated_at = updated_at + 1"
							}
							if kind == "noop" {
								statement = "UPDATE inbounds SET settings = settings"
							}
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								if err := db.Exec(statement).Error; err != nil {
									b.Fatal(err)
								}
							}
							b.ReportMetric(1000, "rows/op")
						})
					}
				})
			}
		})
	}
}
