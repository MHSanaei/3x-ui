package database

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientDepletionMigration(t *testing.T) {
	for _, nullable := range []bool{false, true} {
		t.Run(fmt.Sprintf("nullable_%t", nullable), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x-ui.db")
			legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			checkDepletionMigration(t, legacy, path, nullable)
		})
	}
}

func TestClientDepletionMigration_Postgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_DB_DSN"))
	if dsn == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run PostgreSQL migration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handle, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	for _, nullable := range []bool{false, true} {
		t.Run(fmt.Sprintf("nullable_%t", nullable), func(t *testing.T) {
			schema := fmt.Sprintf("depletion_%d", time.Now().UnixNano())
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
				u, err := url.Parse(dsn)
				if err != nil {
					t.Fatal(err)
				}
				query := u.Query()
				query.Set("search_path", schema)
				u.RawQuery = query.Encode()
				scoped = u.String()
			}
			t.Setenv("XUI_DB_DSN", scoped)
			legacy, err := gorm.Open(postgres.Open(scoped), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			checkDepletionMigration(t, legacy, "", nullable)
		})
	}
}

func checkDepletionMigration(t *testing.T, legacy *gorm.DB, path string, nullable bool) {
	t.Helper()
	columns := ""
	if nullable {
		columns = ", depleted_at BIGINT, depleted_reason TEXT"
	}
	for _, sql := range []string{
		"CREATE TABLE client_traffics (id INTEGER PRIMARY KEY, email TEXT, up BIGINT, down BIGINT, total BIGINT, expiry_time BIGINT, enable BOOLEAN" + columns + ")",
		"INSERT INTO client_traffics (id,email,up,down,total,expiry_time,enable) VALUES (1,'legacy',111,222,300,1700000000000,false)",
	} {
		if err := legacy.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if nullable {
		if err := legacy.Exec("INSERT INTO client_traffics (id,email,up,down,depleted_at,depleted_reason) VALUES (2,'known',3,4,1699999999000,'traffic')").Error; err != nil {
			t.Fatal(err)
		}
	}
	previous := db
	db = legacy
	for range 2 {
		err := migrateClientTrafficDepletionColumns()
		if err != nil {
			db = previous
			t.Fatal(err)
		}
	}
	db = previous
	columnsAfter, err := legacy.Migrator().ColumnTypes(&xray.ClientTraffic{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columnsAfter {
		if column.Name() != "depleted_at" {
			continue
		}
		if nullable, known := column.Nullable(); !known || nullable {
			t.Fatal("depleted_at must be NOT NULL after the explicit migration")
		}
		if value, known := column.DefaultValue(); !known || value != "0" {
			t.Fatalf("depleted_at default = %q, known=%t; want 0", value, known)
		}
	}
	handle, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	for range 2 {
		var row xray.ClientTraffic
		if err := GetDB().Where("email = ?", "legacy").First(&row).Error; err != nil {
			t.Fatal(err)
		}
		if !nullable && GetDB().Migrator().HasColumn(&xray.ClientTraffic{}, "depleted_reason") {
			t.Fatal("migration created a persisted reason")
		}
		if row.Up != 111 || row.Down != 222 || row.Total != 300 || row.ExpiryTime != 1700000000000 || row.Enable || row.DepletedAt != 0 || row.DepletedReason != "" {
			t.Fatalf("legacy data changed: %+v", row)
		}
		if nullable {
			var known xray.ClientTraffic
			if err := GetDB().Where("email = ?", "known").First(&known).Error; err != nil {
				t.Fatal(err)
			}
			if known.DepletedAt != 1699999999000 || known.DepletedReason != "traffic" || known.Up != 3 || known.Down != 4 {
				t.Fatalf("known event changed: %+v", known)
			}
		}
		var nulls int64
		if err := GetDB().Model(&xray.ClientTraffic{}).Where("depleted_at IS NULL").Count(&nulls).Error; err != nil || nulls != 0 {
			t.Fatalf("nullable markers = %d, error = %v", nulls, err)
		}
		if err := migrateClientTrafficDepletionColumns(); err != nil {
			t.Fatal(err)
		}
	}
	if err := CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	var reopened xray.ClientTraffic
	if err := GetDB().Where("email = ?", "legacy").First(&reopened).Error; err != nil || reopened.Up != 111 || reopened.DepletedAt != 0 {
		t.Fatalf("reopened row = %+v, error = %v", reopened, err)
	}
}
