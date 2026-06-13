package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationModels is the FK-aware order in which tables are created and copied
// during `x-ui migrate-db --dsn` (SQLite → PostgreSQL data migration) and in
// related tests.
//
// Important: When adding a new top-level model (like OutboundSubscription),
// you must add it here **in addition to** the list in internal/database/db.go:initModels().
// This list is used for:
//   - Creating the destination schema during cross-DB migration
//   - Truncating tables
//   - Copying data row-by-row
//   - Resyncing Postgres sequences after bulk insert
//
// DumpSQLite / RestoreSQLite are schema-introspective (they read sqlite_master)
// so they do not need manual updates.
func migrationModels() []any {
	return []any{
		&model.User{},
		&model.Setting{},
		&model.HistoryOfSeeders{},
		&model.Node{},
		&model.ApiToken{},
		&model.Inbound{},
		&xray.ClientTraffic{},
		&model.OutboundTraffics{},
		&model.InboundClientIps{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.InboundFallback{},
		&model.NodeClientTraffic{},
		&model.OutboundSubscription{},
	}
}

// MigrateData copies every row from the configured SQLite file at srcPath into
// a fresh PostgreSQL database described by dstDSN. The destination tables are
// (re)created with AutoMigrate before the copy. Source data is left untouched.
func MigrateData(srcPath, dstDSN string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("source sqlite not found at %s: %w", srcPath, err)
	}
	if dstDSN == "" {
		return errors.New("destination DSN is required")
	}

	if err := os.MkdirAll(path.Dir(srcPath), 0755); err != nil {
		return err
	}

	srcDSN := srcPath + "?_journal_mode=WAL&_busy_timeout=10000"
	src, err := gorm.Open(sqlite.Open(srcDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open sqlite source: %w", err)
	}
	srcSQL, err := src.DB()
	if err != nil {
		return err
	}
	defer srcSQL.Close()

	dst, err := gorm.Open(postgres.Open(dstDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open postgres destination: %w", err)
	}
	dstSQL, err := dst.DB()
	if err != nil {
		return err
	}
	defer dstSQL.Close()
	dstSQL.SetConnMaxLifetime(time.Hour)

	log.Println("Creating destination schema...")
	for _, m := range migrationModels() {
		if err := dst.AutoMigrate(m); err != nil {
			return fmt.Errorf("AutoMigrate %T: %w", m, err)
		}
	}

	// AutoMigrate re-creates the legacy client_traffics -> inbounds foreign key,
	// but the running panel drops it (see dropLegacyForeignKeys) and tolerates
	// client_traffics rows whose inbound was deleted. Drop it here too so copying
	// such orphaned rows can't fail with an fk_inbounds_client_stats violation.
	if err := dst.Exec("ALTER TABLE client_traffics DROP CONSTRAINT IF EXISTS fk_inbounds_client_stats").Error; err != nil {
		return fmt.Errorf("drop legacy foreign key: %w", err)
	}

	// Empty the destination tables so the migration is idempotent: a fresh
	// PostgreSQL DB already holds an auto-seeded admin (id=1) from any prior
	// panel start, and a partially-failed earlier run leaves rows behind. Either
	// way a plain INSERT with explicit ids would collide on users_pkey, so clear
	// our tables (only) before copying.
	if err := truncatePostgresTables(dst, migrationModels()); err != nil {
		return fmt.Errorf("clear destination tables: %w", err)
	}

	totalRows := 0
	for _, m := range migrationModels() {
		n, err := copyTable(src, dst, m)
		if err != nil {
			return fmt.Errorf("copy %T: %w", m, err)
		}
		totalRows += n
		log.Printf("  %-32s %d rows", reflect.TypeOf(m).Elem().Name(), n)
	}

	if err := resetPostgresSequences(dst); err != nil {
		log.Printf("warning: failed to reset some postgres sequences: %v", err)
	}

	log.Printf("Migration complete: %d rows across %d tables.", totalRows, len(migrationModels()))
	log.Println("Set XUI_DB_TYPE=postgres and XUI_DB_DSN=... in /etc/default/x-ui, then restart x-ui.")
	return nil
}

// ExportPostgresToSQLite copies every row from the PostgreSQL database described
// by srcDSN into a fresh SQLite file at dstPath. It is the reverse of
// MigrateData and is used to hand a PostgreSQL-backed panel a portable .db file.
// dstPath is created/overwritten; the PostgreSQL source is left untouched.
func ExportPostgresToSQLite(srcDSN, dstPath string) error {
	if srcDSN == "" {
		return errors.New("source DSN is required")
	}
	if err := os.MkdirAll(path.Dir(dstPath), 0755); err != nil {
		return err
	}
	// Start from an empty file so AutoMigrate creates the canonical schema.
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	src, err := gorm.Open(postgres.Open(srcDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open postgres source: %w", err)
	}
	srcSQL, err := src.DB()
	if err != nil {
		return err
	}
	defer srcSQL.Close()

	// No WAL: keep all data in the main file so it is complete once closed.
	dst, err := gorm.Open(sqlite.Open(dstPath+"?_busy_timeout=10000"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("open sqlite destination: %w", err)
	}
	dstSQL, err := dst.DB()
	if err != nil {
		return err
	}
	defer dstSQL.Close()

	return copyAllModels(src, dst)
}

// copyAllModels (re)creates the schema on dst and copies every migrated table
// from src to dst in FK-safe order. src/dst may be any gorm backend.
func copyAllModels(src, dst *gorm.DB) error {
	for _, m := range migrationModels() {
		if err := dst.AutoMigrate(m); err != nil {
			return fmt.Errorf("AutoMigrate %T: %w", m, err)
		}
	}
	for _, m := range migrationModels() {
		if _, err := copyTable(src, dst, m); err != nil {
			return fmt.Errorf("copy %T: %w", m, err)
		}
	}
	return nil
}

func copyTable(src, dst *gorm.DB, mdl any) (int, error) {
	const batchSize = 500

	sliceType := reflect.SliceOf(reflect.PointerTo(reflect.TypeOf(mdl).Elem()))

	stmt := &gorm.Statement{DB: src}
	if err := stmt.Parse(mdl); err != nil {
		return 0, err
	}
	order := strings.Join(stmt.Schema.PrimaryFieldDBNames, ", ")
	table := stmt.Schema.Table
	columns := stmt.Schema.DBNames

	ctx := context.Background()
	total := 0
	for offset := 0; ; offset += batchSize {
		batchPtr := reflect.New(sliceType)
		q := src.Model(mdl).Limit(batchSize).Offset(offset)
		if order != "" {
			q = q.Order(order)
		}
		if err := q.Find(batchPtr.Interface()).Error; err != nil {
			return total, err
		}
		slice := batchPtr.Elem()
		n := slice.Len()
		if n == 0 {
			break
		}

		rows := make([]map[string]any, n)
		for i := 0; i < n; i++ {
			rv := reflect.Indirect(slice.Index(i))
			row := make(map[string]any, len(columns))
			for _, name := range columns {
				value, _ := stmt.Schema.FieldsByDBName[name].ValueOf(ctx, rv)
				row[name] = value
			}
			rows[i] = row
		}

		if err := dst.Table(table).CreateInBatches(rows, 200).Error; err != nil {
			return total, err
		}
		total += n
		if n < batchSize {
			break
		}
	}
	return total, nil
}

// truncatePostgresTables empties every migrated table on dst in a single
// statement, resetting identity sequences. CASCADE covers the inbound/client
// foreign keys regardless of insertion order. Only the panel's own tables are
// touched, never the rest of the schema.
func truncatePostgresTables(dst *gorm.DB, models []any) error {
	tables := make([]string, 0, len(models))
	for _, m := range models {
		stmt := &gorm.Statement{DB: dst}
		if err := stmt.Parse(m); err != nil {
			return err
		}
		tables = append(tables, `"`+stmt.Schema.Table+`"`)
	}
	if len(tables) == 0 {
		return nil
	}
	log.Println("Clearing destination tables...")
	return dst.Exec("TRUNCATE TABLE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE").Error
}

// resetPostgresSequences advances each migrated table's id sequence past MAX(id),
// otherwise the next INSERT-without-id would clash with copied rows.
func resetPostgresSequences(dst *gorm.DB) error {
	return resyncPostgresSequences(dst, migrationModels())
}

// resyncPostgresSequences sets each model's id sequence to MAX(id) so the next
// auto-increment INSERT won't collide with an existing row. Table names are
// resolved from the models themselves (not hardcoded), so they always match the
// migrated tables. The statement is a no-op for tables without an id sequence
// (e.g. composite-PK tables), and idempotent on a healthy DB, so it is safe to
// run both after migration and on every Postgres startup.
func resyncPostgresSequences(db *gorm.DB, models []any) error {
	for _, m := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			continue
		}
		t := stmt.Table
		// t comes from the trusted model set parsed by GORM, not user input, so
		// interpolating it as an identifier is safe. We ignore errors per-table.
		_ = db.Exec(
			`SELECT setval(pg_get_serial_sequence(?, 'id'), COALESCE((SELECT MAX(id) FROM "`+t+`"), 1), true)
			 WHERE pg_get_serial_sequence(?, 'id') IS NOT NULL`,
			t, t,
		).Error
	}
	return nil
}
