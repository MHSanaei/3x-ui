package database

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDumpSQLiteToBytesWithOptionsExcludesHostSpecificSettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Exec(`CREATE TABLE settings (id integer primary key autoincrement, key text, value text)`).Error; err != nil {
		t.Fatal(err)
	}

	rows := []struct {
		key   string
		value string
	}{
		{"webCertFile", "/etc/ssl/panel/fullchain.pem"},
		{"webKeyFile", "/etc/ssl/panel/privkey.pem"},
		{"subCertFile", "/etc/ssl/sub/fullchain.pem"},
		{"subKeyFile", "/etc/ssl/sub/privkey.pem"},
		{"webBasePath", "/panel/"},
	}

	for _, row := range rows {
		if err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, row.key, row.value).Error; err != nil {
			t.Fatal(err)
		}
	}

	dump, err := DumpSQLiteToBytesWithOptions(dbPath, PortableDumpOptions{
		ExcludeHostSpecific: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(dump)

	for _, forbidden := range []string{
		"/etc/ssl/panel/fullchain.pem",
		"/etc/ssl/panel/privkey.pem",
		"/etc/ssl/sub/fullchain.pem",
		"/etc/ssl/sub/privkey.pem",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("portable dump leaked host-specific value %q in:\n%s", forbidden, got)
		}
	}

	if !strings.Contains(got, "'webBasePath','/panel/'") {
		t.Fatalf("portable dump removed unrelated settings:\n%s", got)
	}

	for _, key := range []string{"webCertFile", "webKeyFile", "subCertFile", "subKeyFile"} {
		if !strings.Contains(got, "'"+key+"',''") {
			t.Fatalf("portable dump did not blank %s:\n%s", key, got)
		}
	}
}
