package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestStageSQLiteUploadRebuildsFromDump(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x-ui.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	dump, err := database.DumpSQLiteToBytes(dbPath)
	if err != nil {
		t.Fatalf("DumpSQLiteToBytes: %v", err)
	}

	uploadPath := filepath.Join(dir, "upload.dump")
	if err := os.WriteFile(uploadPath, dump, 0o644); err != nil {
		t.Fatalf("write upload: %v", err)
	}
	upload, err := os.Open(uploadPath)
	if err != nil {
		t.Fatalf("open upload: %v", err)
	}
	defer upload.Close()

	staged := filepath.Join(dir, "x-ui.db.temp")
	if err := stageSQLiteUpload(upload, importKindSQLiteDump, staged); err != nil {
		t.Fatalf("stageSQLiteUpload: %v", err)
	}
	if _, err := os.Stat(staged + ".dump"); !os.IsNotExist(err) {
		t.Errorf("intermediate dump file %s.dump was not cleaned up", staged)
	}
	if err := database.ValidateSQLiteDB(staged); err != nil {
		t.Errorf("staged database fails integrity check: %v", err)
	}
	if err := database.PrepareSQLiteForMigration(staged); err != nil {
		t.Errorf("staged database fails the panel pre-flight: %v", err)
	}
}
