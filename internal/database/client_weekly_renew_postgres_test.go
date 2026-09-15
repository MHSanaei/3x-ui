package database

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientWeeklyRenewMigration_Postgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_DB_DSN"))
	if dsn == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the weekly migration test")
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
			schema := fmt.Sprintf("weekly_renew_%d", time.Now().UnixNano())
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
			legacyHandle, err := legacy.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = legacyHandle.Close() })
			column := ""
			if nullable {
				column = ", reset_weekday INTEGER"
			}
			for _, ddl := range []string{
				"CREATE TABLE clients (id BIGSERIAL PRIMARY KEY, email TEXT, reset INTEGER, reset_day INTEGER, reset_max INTEGER, expiry_time BIGINT" + column + ")",
				"CREATE TABLE client_traffics (id BIGSERIAL PRIMARY KEY, email TEXT, reset INTEGER, reset_day INTEGER, reset_max INTEGER, reset_count INTEGER, expiry_time BIGINT, up BIGINT, down BIGINT" + column + ")",
				"INSERT INTO clients (email,reset,reset_day,reset_max,expiry_time) VALUES ('legacy',30,15,3,1893456000000)",
				"INSERT INTO client_traffics (email,reset,reset_day,reset_max,reset_count,expiry_time,up,down) VALUES ('legacy',30,15,3,2,1893456000000,111,222)",
			} {
				if err := legacy.Exec(ddl).Error; err != nil {
					t.Fatal(err)
				}
			}
			if nullable {
				if err := legacy.Exec("INSERT INTO clients (email,reset_weekday) VALUES ('weekly',2)").Error; err != nil {
					t.Fatal(err)
				}
				if err := legacy.Exec("INSERT INTO client_traffics (email,reset_weekday) VALUES ('weekly',2)").Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := InitDB(""); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"clients", "client_traffics"} {
				var nulls int64
				if err := GetDB().Table(table).Where("reset_weekday IS NULL").Count(&nulls).Error; err != nil || nulls != 0 {
					t.Fatalf("%s NULL weekdays/error = %d/%v", table, nulls, err)
				}
				if nullable {
					var weekday int
					if err := GetDB().Table(table).Where("email = ?", "weekly").Pluck("reset_weekday", &weekday).Error; err != nil || weekday != 2 {
						t.Fatalf("%s weekday/error = %d/%v, want 2/nil", table, weekday, err)
					}
				}
			}
			var client model.ClientRecord
			if err := GetDB().Where("email = ?", "legacy").First(&client).Error; err != nil {
				t.Fatal(err)
			}
			var traffic xray.ClientTraffic
			if err := GetDB().Where("email = ?", "legacy").First(&traffic).Error; err != nil {
				t.Fatal(err)
			}
			if client.ResetWeekday != 0 || client.Reset != 30 || client.ResetDay != 15 || client.ResetMax != 3 || client.ExpiryTime != 1893456000000 || traffic.ResetWeekday != 0 || traffic.Reset != 30 || traffic.ResetDay != 15 || traffic.ResetMax != 3 || traffic.ResetCount != 2 || traffic.Up != 111 || traffic.Down != 222 || traffic.ExpiryTime != client.ExpiryTime {
				t.Fatalf("legacy PostgreSQL limits changed: client=%+v traffic=%+v", client, traffic)
			}
			if err := migrateClientResetWeekdayColumns(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
