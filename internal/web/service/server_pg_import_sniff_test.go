package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestSniffPgImportKind(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   int
	}{
		{"pg custom archive", []byte("PGDMP\x01\x10\x04"), pgImportPgDump},
		{"raw sqlite database", []byte("SQLite format 3\x00rest of header"), pgImportSQLiteDB},
		{"sqlite cli dump without pragma", []byte("BEGIN TRANSACTION;\nCREATE TABLE t(i);"), pgImportSQLiteDump},
		{"bom and whitespace before pragma", []byte("\xef\xbb\xbf\r\n PRAGMA foreign_keys=OFF;"), pgImportSQLiteDump},
		{"plain-format postgres dump", []byte("--\n-- PostgreSQL database dump\n--"), pgImportUnknown},
		{"empty file", nil, pgImportUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffPgImportKind(tc.header); got != tc.want {
				t.Errorf("sniffPgImportKind(%q) = %d, want %d", tc.header, got, tc.want)
			}
		})
	}

	t.Run("panel migration dump", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "x-ui.db")
		if err := database.InitDB(dbPath); err != nil {
			t.Fatalf("InitDB: %v", err)
		}
		t.Cleanup(func() { _ = database.CloseDB() })
		dump, err := database.DumpSQLiteToBytes(dbPath)
		if err != nil {
			t.Fatalf("DumpSQLiteToBytes: %v", err)
		}
		if got := sniffPgImportKind(dump[:64]); got != pgImportSQLiteDump {
			t.Errorf("sniffPgImportKind(real migration dump) = %d, want %d", got, pgImportSQLiteDump)
		}
	})
}
