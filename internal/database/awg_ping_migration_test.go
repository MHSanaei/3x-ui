package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func assertAWGPingMigrated(t *testing.T) {
	t.Helper()
	db := GetDB()
	var legacy model.ClientRecord
	if err := db.Where("email = ?", "synthetic-legacy").Take(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.Id != 1 || legacy.AWGPing != nil || legacy.AWGPingRevision != 0 {
		t.Fatalf("legacy policy must deny: %#v", legacy.AWGPing)
	}
	legacy.AWGPing = &model.AWGPingPolicy{Enabled: true, Allow: []string{"192.0.2.7/24"}, Deny: []string{"192.0.2.8"}}
	if err := db.Save(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	var reloaded model.ClientRecord
	if err := db.First(&reloaded, legacy.Id).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.AWGPing == nil || reloaded.AWGPing.Allow[0] != "192.0.2.0/24" || reloaded.AWGPing.Deny[0] != "192.0.2.8/32" {
		t.Fatalf("policy not persisted: %#v", reloaded.AWGPing)
	}
	reloaded.AWGPing = &model.AWGPingPolicy{Enabled: false, Allow: []string{}, Deny: []string{}}
	if err := db.Save(&reloaded).Error; err != nil {
		t.Fatal(err)
	}
	var cleared model.ClientRecord
	if err := db.First(&cleared, reloaded.Id).Error; err != nil {
		t.Fatal(err)
	}
	if cleared.AWGPing == nil || cleared.AWGPing.Enabled || len(cleared.AWGPing.Allow) != 0 {
		t.Fatal("cleared policy resurrected")
	}
}

func TestAWGPingSQLiteMigration(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "x-ui.db")
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("CREATE TABLE clients(id INTEGER PRIMARY KEY AUTOINCREMENT,email TEXT NOT NULL UNIQUE)"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("INSERT INTO clients(email) VALUES ('synthetic-legacy')"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("CREATE TABLE inbounds(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,port INTEGER NOT NULL,protocol TEXT,settings TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`INSERT INTO inbounds(id,user_id,port,protocol,settings) VALUES(41,1,51820,'amneziawg','{"clients":[{"email":"synthetic-mirror","enable":true,"awgPing":{"enabled":true,"allow":["192.0.2.0/24"],"deny":[]}}]}')`); err != nil {
		t.Fatal(err)
	}
	_ = legacy.Close()
	t.Setenv("XUI_DB_TYPE", "sqlite")
	t.Setenv("XUI_DB_FOLDER", folder)
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if err := seedClientsFromInboundJSON(); err != nil {
		t.Fatal(err)
	}
	assertAWGPingMigrated(t)
	var mirror model.ClientRecord
	if err := GetDB().Where("email = ?", "synthetic-mirror").Take(&mirror).Error; err != nil {
		t.Fatal(err)
	}
	if mirror.AWGPing != nil {
		t.Fatal("restart seeder granted stale mirror rights")
	}
	var canonical model.ClientRecord
	if err := GetDB().First(&canonical, 1).Error; err != nil {
		t.Fatal(err)
	}
	canonical.AWGPing = &model.AWGPingPolicy{Enabled: true, Allow: []string{"192.0.2.0/24"}}
	if err := GetDB().Save(&canonical).Error; err != nil {
		t.Fatal(err)
	}
	if err := CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	var record model.ClientRecord
	if err := GetDB().First(&record, 1).Error; err != nil {
		t.Fatal(err)
	}
	if record.AWGPing == nil || !record.AWGPing.Enabled {
		t.Fatal("reopen lost canonical enabled policy")
	}
}

func TestAWGPingPostgresMigration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to an isolated PostgreSQL")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handle, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	schema := fmt.Sprintf("awg_ping_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = CloseDB(); _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error }()
	scoped := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		scoped = u.String()
	}
	legacy, err := gorm.Open(postgres.Open(scoped), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("CREATE TABLE clients(id BIGSERIAL PRIMARY KEY,email TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("INSERT INTO clients(email) VALUES ('synthetic-legacy')").Error; err != nil {
		t.Fatal(err)
	}
	h, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Close()
	t.Setenv("XUI_DB_TYPE", "postgres")
	t.Setenv("XUI_DB_DSN", scoped)
	if err := InitDB(""); err != nil {
		t.Fatal(err)
	}
	assertAWGPingMigrated(t)
}
