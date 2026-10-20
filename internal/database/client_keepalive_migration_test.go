package database

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientKeepAliveSQLiteMigrationPreservesLegacyScalarsAndAddsRanges(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "x-ui.db")
	legacy, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL UNIQUE, wg_keep_alive INTEGER DEFAULT 0)",
		"INSERT INTO clients (email, wg_keep_alive) VALUES ('legacy-25', 25), ('legacy-large', 70000), ('legacy-off', 0)",
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XUI_DB_FOLDER", dbDir)
	t.Setenv("XUI_DB_TYPE", "sqlite")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB legacy schema: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var dataType string
	if err := GetDB().Raw("SELECT type FROM pragma_table_info('clients') WHERE name = 'wg_keep_alive'").Scan(&dataType).Error; err != nil {
		t.Fatal(err)
	}
	if dataType != "TEXT" {
		t.Fatalf("wg_keep_alive type = %q, want TEXT", dataType)
	}
	for _, tc := range []struct {
		email string
		want  model.KeepAliveValue
	}{
		{email: "legacy-25", want: "25"},
		{email: "legacy-large", want: "70000"},
		{email: "legacy-off", want: ""},
	} {
		var record model.ClientRecord
		if err := GetDB().Where("email = ?", tc.email).First(&record).Error; err != nil {
			t.Fatalf("read %s: %v", tc.email, err)
		}
		if record.KeepAlive != tc.want {
			t.Fatalf("%s keepAlive = %q, want %q", tc.email, record.KeepAlive, tc.want)
		}
	}

	zero := &model.ClientRecord{Email: "explicit-zero", KeepAlive: "0"}
	if err := GetDB().Create(zero).Error; err != nil {
		t.Fatalf("save explicit zero: %v", err)
	}
	var storageType string
	if err := GetDB().Raw("SELECT typeof(wg_keep_alive) FROM clients WHERE email = ?", zero.Email).Scan(&storageType).Error; err != nil {
		t.Fatal(err)
	}
	if storageType != "text" {
		t.Fatalf("explicit zero SQLite storage class = %q, want text", storageType)
	}

	inbound := &model.Inbound{
		UserId: 1, Port: 51888, Protocol: model.AmneziaWG, Tag: "legacy-awg-range",
		Settings: "{\"server\":{\"privateKey\":\"server\",\"publicKey\":\"pub\"},\"clients\":[{\"email\":\"legacy-range\",\"keepAlive\":\"15-25\"}]}",
	}
	if err := GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("save ranged inbound: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}

	var got model.ClientRecord
	if err := GetDB().Where("email = ?", "explicit-zero").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.KeepAlive != "0" {
		t.Fatalf("reopened explicit zero = %q, want 0", got.KeepAlive)
	}
	var reopened model.Inbound
	if err := GetDB().First(&reopened, inbound.Id).Error; err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(reopened.Settings), &payload); err != nil {
		t.Fatalf("read range from inbound settings after reopen: %v", err)
	}
	if len(payload.Clients) != 1 || payload.Clients[0].KeepAliveText() != "15-25" {
		t.Fatalf("reopened inbound keepAlive = %#v, want 15-25", payload.Clients)
	}
}
