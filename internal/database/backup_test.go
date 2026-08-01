package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBackupSQLiteProducesValidSnapshotDuringWrites(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	seed := make([]model.Setting, 128)
	value := strings.Repeat("x", 1024)
	for i := range seed {
		seed[i] = model.Setting{Key: fmt.Sprintf("backup-seed-%d", i), Value: value}
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed database: %v", err)
	}

	stop := make(chan struct{})
	firstWrite := make(chan error, 1)
	writesDone := make(chan error, 1)
	go func() {
		for i := range 128 {
			if err := db.Create(&model.Setting{Key: fmt.Sprintf("backup-write-%d", i), Value: value}).Error; err != nil {
				if i == 0 {
					firstWrite <- err
				}
				writesDone <- err
				return
			}
			if i == 0 {
				firstWrite <- nil
			}
			select {
			case <-stop:
				writesDone <- nil
				return
			default:
			}
		}
		writesDone <- nil
	}()

	if err := <-firstWrite; err != nil {
		t.Fatalf("first concurrent write: %v", err)
	}
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := BackupSQLite(backupPath); err != nil {
		close(stop)
		<-writesDone
		t.Fatalf("BackupSQLite: %v", err)
	}
	close(stop)
	if err := <-writesDone; err != nil {
		t.Fatalf("concurrent write: %v", err)
	}
	if err := ValidateSQLiteDB(backupPath); err != nil {
		t.Fatalf("validate backup: %v", err)
	}

	backup, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()

	var seedCount int
	if err := backup.QueryRow("SELECT count(*) FROM settings WHERE key LIKE 'backup-seed-%'").Scan(&seedCount); err != nil {
		t.Fatalf("count seeded rows: %v", err)
	}
	if seedCount != 128 {
		t.Fatalf("seeded row count = %d, want 128", seedCount)
	}
	var firstWriteCount int
	if err := backup.QueryRow("SELECT count(*) FROM settings WHERE key = 'backup-write-0'").Scan(&firstWriteCount); err != nil {
		t.Fatalf("count first concurrent write: %v", err)
	}
	if firstWriteCount != 1 {
		t.Fatalf("first concurrent write count = %d, want 1", firstWriteCount)
	}
}

func TestBackupSQLiteTimesOutWaitingForSourceConnection(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database connection pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	held, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("hold source connection: %v", err)
	}
	defer held.Close()

	previousTimeout := backupSQLiteTimeout
	backupSQLiteTimeout = 20 * time.Millisecond
	t.Cleanup(func() { backupSQLiteTimeout = previousTimeout })
	err = BackupSQLite(filepath.Join(t.TempDir(), "backup.db"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("BackupSQLite error = %v, want context deadline exceeded", err)
	}
}

func TestBackupSQLiteRefusesExistingDestination(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(backupPath, []byte("existing backup"), 0o600); err != nil {
		t.Fatalf("create existing destination: %v", err)
	}
	err := BackupSQLite(backupPath)
	want := fmt.Sprintf("sqlite backup destination already exists: %s", backupPath)
	if err == nil || err.Error() != want {
		t.Fatalf("BackupSQLite error = %v, want %q", err, want)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read existing destination: %v", err)
	}
	if string(data) != "existing backup" {
		t.Fatalf("existing destination = %q, want %q", data, "existing backup")
	}
}

func TestBackupSQLiteStepPages(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	if got := backupSQLiteStepPages(); got != -1 {
		t.Fatalf("WAL backup step pages = %d, want -1", got)
	}
	t.Setenv("XUI_DB_JOURNAL_MODE", "DELETE")
	if got := backupSQLiteStepPages(); got != 128 {
		t.Fatalf("DELETE backup step pages = %d, want 128", got)
	}
}

func TestInitDBCleansBackupDirectories(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	dbDir := t.TempDir()
	orphanDir := filepath.Join(dbDir, sqliteBackupDirPrefix+"orphan")
	if err := os.Mkdir(orphanDir, 0o700); err != nil {
		t.Fatalf("create orphan backup directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "backup.db"), []byte("backup"), 0o600); err != nil {
		t.Fatalf("write orphan backup: %v", err)
	}
	regularDir := filepath.Join(dbDir, ".x-ui-keep")
	if err := os.Mkdir(regularDir, 0o700); err != nil {
		t.Fatalf("create regular directory: %v", err)
	}

	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if _, err := os.Stat(orphanDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan backup directory error = %v, want not exist", err)
	}
	if info, err := os.Stat(regularDir); err != nil || !info.IsDir() {
		t.Fatalf("regular directory info = %v, %v; want existing directory", info, err)
	}
}
