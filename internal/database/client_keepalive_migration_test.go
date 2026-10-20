package database

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type legacyKeepAliveClient struct {
	Id        int    `gorm:"column:id;primaryKey"`
	Email     string `gorm:"column:email"`
	KeepAlive int    `gorm:"column:wg_keep_alive"`
}

func (legacyKeepAliveClient) TableName() string { return "clients" }

func TestClientKeepAliveSQLiteMigrationPreservesLegacyScalarsAndAddsRanges(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "x-ui.db")
	legacy, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL UNIQUE, wg_keep_alive INTEGER DEFAULT 0)",
		"INSERT INTO clients (email, wg_keep_alive) VALUES ('legacy-25', 25), ('legacy-large', 70000), ('legacy-off', 0), ('legacy-negative', -5)",
		"CREATE TABLE inbounds (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, port INTEGER NOT NULL, protocol TEXT, settings TEXT)",
		`INSERT INTO inbounds (id, user_id, port, protocol, settings) VALUES (41, 1, 51888, 'amneziawg', '{"clients":[{"email":"legacy-negative-inbound","keepAlive":-5}]}')`,
		"CREATE TABLE client_inbounds (client_id INTEGER NOT NULL, inbound_id INTEGER NOT NULL, flow_override TEXT, created_at INTEGER, PRIMARY KEY(client_id, inbound_id))",
		"INSERT INTO client_inbounds (client_id, inbound_id, flow_override, created_at) VALUES (1, 41, '', 1234)",
		"CREATE TABLE client_traffics (id INTEGER PRIMARY KEY AUTOINCREMENT, inbound_id INTEGER, email TEXT, up INTEGER, down INTEGER)",
		"INSERT INTO client_traffics (inbound_id, email, up, down) VALUES (41, 'legacy-25', 1200, 3400)",
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
		{email: "legacy-off", want: "0"},
		{email: "legacy-negative", want: "-5"},
	} {
		var record model.ClientRecord
		if err := GetDB().Where("email = ?", tc.email).First(&record).Error; err != nil {
			t.Fatalf("read %s: %v", tc.email, err)
		}
		if tc.email == "legacy-25" && record.Id != 1 {
			t.Fatalf("legacy client ID = %d, want preserved ID 1", record.Id)
		}
		if record.KeepAlive != tc.want {
			t.Fatalf("%s keepAlive = %q, want %q", tc.email, record.KeepAlive, tc.want)
		}
	}

	for _, tc := range []struct {
		email string
		want  int
	}{{email: "legacy-25", want: 25}, {email: "legacy-large", want: 70000}, {email: "legacy-off", want: 0}} {
		var legacyClient legacyKeepAliveClient
		if err := GetDB().Where("email = ?", tc.email).First(&legacyClient).Error; err != nil {
			t.Fatalf("legacy int model cannot read %s after migration: %v", tc.email, err)
		}
		if legacyClient.KeepAlive != tc.want {
			t.Fatalf("legacy int model read %s keepAlive = %d, want %d", tc.email, legacyClient.KeepAlive, tc.want)
		}
	}
	if err := GetDB().Create(&model.ClientRecord{Email: "new-unset"}).Error; err != nil {
		t.Fatalf("create new unset client: %v", err)
	}
	var legacyUnset legacyKeepAliveClient
	if err := GetDB().Where("email = ?", "new-unset").First(&legacyUnset).Error; err != nil || legacyUnset.KeepAlive != 0 {
		t.Fatalf("legacy int model read of new unset keepAlive = %d, %v; want zero", legacyUnset.KeepAlive, err)
	}
	if err := GetDB().Create(&legacyKeepAliveClient{Email: "old-panel-write", KeepAlive: 45}).Error; err != nil {
		t.Fatalf("legacy int model cannot write after migration: %v", err)
	}
	var currentWrite model.ClientRecord
	if err := GetDB().Where("email = ?", "old-panel-write").First(&currentWrite).Error; err != nil || currentWrite.KeepAlive != "45" {
		t.Fatalf("new model read of legacy write = %q, %v; want 45", currentWrite.KeepAlive, err)
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

	var link model.ClientInbound
	if err := GetDB().Where("client_id = ? AND inbound_id = ?", 1, 41).First(&link).Error; err != nil {
		t.Fatalf("client-inbound link was not preserved: %v", err)
	}
	if link.ClientId != 1 || link.InboundId != 41 || link.FlowOverride != "" || link.CreatedAt != 1234 {
		t.Fatalf("client-inbound link changed during migration: %#v", link)
	}
	var traffic xray.ClientTraffic
	if err := GetDB().Where("inbound_id = ? AND email = ?", 41, "legacy-25").First(&traffic).Error; err != nil {
		t.Fatalf("client traffic was not preserved: %v", err)
	}
	if traffic.Id != 1 || traffic.Up != 1200 || traffic.Down != 3400 {
		t.Fatalf("client traffic changed during migration: %#v", traffic)
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
	var legacyInbound model.Inbound
	if err := GetDB().First(&legacyInbound, 41).Error; err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(legacyInbound.Settings), &payload); err != nil || len(payload.Clients) != 1 || payload.Clients[0].KeepAliveText() != "-5" {
		t.Fatalf("legacy negative inbound keepAlive = %#v, %v; want -5", payload.Clients, err)
	}
}
