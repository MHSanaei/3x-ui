package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationModels is the FK-aware order in which tables are created and copied.
// Parents come before their children so foreign-key constraints stay satisfied
// even when checks are not explicitly disabled.
func migrationModels() []any {
	return []any{
		&model.User{},
		&model.Setting{},
		&model.HistoryOfSeeders{},
		&model.CustomGeoResource{},
		&model.Node{},
		&model.ApiToken{},
		&model.Inbound{},
		&xray.ClientTraffic{},
		&model.OutboundTraffics{},
		&model.InboundClientIps{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.InboundFallback{},
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

// copyTable streams every row of `mdl` from src to dst in batches.
func copyTable(src, dst *gorm.DB, mdl any) (int, error) {
	sliceType := reflect.SliceOf(reflect.PointerTo(reflect.TypeOf(mdl).Elem()))
	batchPtr := reflect.New(sliceType)
	batchPtr.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))

	total := 0
	err := src.Model(mdl).FindInBatches(batchPtr.Interface(), 500, func(tx *gorm.DB, _ int) error {
		batch := batchPtr.Elem()
		if batch.Len() == 0 {
			return nil
		}
		if err := dst.CreateInBatches(batchPtr.Interface(), 200).Error; err != nil {
			return err
		}
		total += batch.Len()
		return nil
	}).Error
	return total, err
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
