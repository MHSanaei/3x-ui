package database

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestClientKeepAlivePostgresMigrationPreservesLegacyScalars(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("XUI_DB_DSN"))
	}
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN or XUI_DB_DSN to run the PostgreSQL keepAlive migration test")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	adminHandle, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminHandle.Close() })

	schema := fmt.Sprintf("keepalive_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = CloseDB()
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})

	scoped := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		scoped = parsed.String()
	}
	t.Setenv("XUI_DB_TYPE", "postgres")
	t.Setenv("XUI_DB_DSN", scoped)

	legacy, err := gorm.Open(postgres.Open(scoped), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy schema: %v", err)
	}
	if err := legacy.Exec("CREATE TABLE clients (id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL, wg_keep_alive INTEGER DEFAULT 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("INSERT INTO clients (email, wg_keep_alive) VALUES ('legacy-25', 25), ('legacy-large', 70000), ('legacy-off', 0)").Error; err != nil {
		t.Fatal(err)
	}
	legacyHandle, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyHandle.Close(); err != nil {
		t.Fatal(err)
	}

	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB legacy schema: %v", err)
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
	inbound := &model.Inbound{
		UserId: 1, Port: 51889, Protocol: model.AmneziaWG, Tag: "legacy-awg-range-pg",
		Settings: "{\"server\":{\"privateKey\":\"server\",\"publicKey\":\"pub\"},\"clients\":[{\"email\":\"legacy-range\",\"keepAlive\":\"15-25\"}]}",
	}
	if err := GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("save ranged inbound: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("reopen migrated PostgreSQL database: %v", err)
	}

	var dataType string
	if err := GetDB().Raw("SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'clients' AND column_name = 'wg_keep_alive'").Scan(&dataType).Error; err != nil {
		t.Fatal(err)
	}
	if dataType != "text" {
		t.Fatalf("wg_keep_alive data type = %q, want text", dataType)
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
