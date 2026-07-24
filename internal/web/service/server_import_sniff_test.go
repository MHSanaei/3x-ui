package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestSniffImportKind(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   int
	}{
		{"pg custom archive", []byte("PGDMP\x01\x10\x04"), importKindPgDump},
		{"raw sqlite database", []byte("SQLite format 3\x00rest of header"), importKindSQLiteDB},
		{"sqlite cli dump without pragma", []byte("BEGIN TRANSACTION;\nCREATE TABLE t(i);"), importKindSQLiteDump},
		{"bom and whitespace before pragma", []byte("\xef\xbb\xbf\r\n PRAGMA foreign_keys=OFF;"), importKindSQLiteDump},
		{"plain-format postgres dump", []byte("--\n-- PostgreSQL database dump\n--"), importKindUnknown},
		{"empty file", nil, importKindUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffImportKind(tc.header); got != tc.want {
				t.Errorf("sniffImportKind(%q) = %d, want %d", tc.header, got, tc.want)
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
		if got := sniffImportKind(dump[:64]); got != importKindSQLiteDump {
			t.Errorf("sniffImportKind(real migration dump) = %d, want %d", got, importKindSQLiteDump)
		}
	})
}
