package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A PostgreSQL server that accepts the connection and then stops answering
// must not hang the database download and the scheduled backups forever.
func TestExportPostgresDBGivesUpOnAStalledServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake pg_dump is a POSIX shell script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "pg_dump"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XUI_DB_DSN", "postgres://xui:secret@127.0.0.1:5432/xui?sslmode=disable")
	prev := pgDumpTimeout
	pgDumpTimeout = 300 * time.Millisecond
	t.Cleanup(func() { pgDumpTimeout = prev })

	start := time.Now()
	_, err := (&ServerService{}).exportPostgresDB()
	if err == nil || !strings.Contains(err.Error(), "pg_dump did not finish within 300ms") {
		t.Fatalf("exportPostgresDB() error = %v, want the timeout", err)
	}
	if took := time.Since(start); took > 15*time.Second {
		t.Fatalf("exportPostgresDB() returned after %v", took)
	}
}
