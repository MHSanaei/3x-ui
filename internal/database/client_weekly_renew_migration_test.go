package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientWeeklyRenewMigration(t *testing.T) {
	for _, nullable := range []bool{false, true} {
		name := "missing columns"
		if nullable {
			name = "nullable columns and configured weekday"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x-ui.db")
			legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			column := ""
			if nullable {
				column = ", reset_weekday INTEGER"
			}
			for _, ddl := range []string{
				"CREATE TABLE clients (id INTEGER PRIMARY KEY, email TEXT, reset INTEGER, reset_day INTEGER, reset_max INTEGER, expiry_time BIGINT" + column + ")",
				"CREATE TABLE client_traffics (id INTEGER PRIMARY KEY, email TEXT, reset INTEGER, reset_day INTEGER, reset_max INTEGER, reset_count INTEGER, expiry_time BIGINT, up BIGINT, down BIGINT" + column + ")",
				"INSERT INTO clients (id,email,reset,reset_day,reset_max,expiry_time) VALUES (1,'legacy',30,15,3,1893456000000)",
				"INSERT INTO client_traffics (id,email,reset,reset_day,reset_max,reset_count,expiry_time,up,down) VALUES (1,'legacy',30,15,3,2,1893456000000,111,222)",
			} {
				if err := legacy.Exec(ddl).Error; err != nil {
					t.Fatal(err)
				}
			}
			if nullable {
				if err := legacy.Exec("INSERT INTO clients (id,email,reset_weekday) VALUES (2,'weekly',2)").Error; err != nil {
					t.Fatal(err)
				}
				if err := legacy.Exec("INSERT INTO client_traffics (id,email,reset_weekday) VALUES (2,'weekly',2)").Error; err != nil {
					t.Fatal(err)
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
			for _, table := range []string{"clients", "client_traffics"} {
				var nulls int64
				if err := GetDB().Table(table).Where("reset_weekday IS NULL").Count(&nulls).Error; err != nil || nulls != 0 {
					t.Fatalf("%s NULL weekdays/error = %d/%v", table, nulls, err)
				}
				if nullable {
					var weekday int
					if err := GetDB().Table(table).Where("email = ?", "weekly").Pluck("reset_weekday", &weekday).Error; err != nil || weekday != 2 {
						t.Fatalf("%s configured weekday/error = %d/%v", table, weekday, err)
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
			if client.Reset != 30 || client.ResetDay != 15 || client.ResetMax != 3 || client.ResetWeekday != 0 || client.ExpiryTime != 1893456000000 {
				t.Fatalf("legacy client changed: %+v", client)
			}
			if traffic.Reset != 30 || traffic.ResetDay != 15 || traffic.ResetMax != 3 || traffic.ResetCount != 2 || traffic.ResetWeekday != 0 || traffic.ExpiryTime != 1893456000000 || traffic.Up != 111 || traffic.Down != 222 {
				t.Fatalf("legacy traffic changed: %+v", traffic)
			}
			if err := migrateClientResetWeekdayColumns(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
