package outbound

import (
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAddTrafficReturnsDeferredCommitFailure(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run commit-failure injection")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	const parent = "outbound_commit_parent"
	const child = "outbound_commit_child"
	_ = db.Exec("DROP TABLE IF EXISTS " + child).Error
	_ = db.Exec("DROP TABLE IF EXISTS " + parent).Error
	if err := db.Exec("CREATE TABLE " + parent + " (id bigint PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + child + " (id bigint PRIMARY KEY, parent_id bigint REFERENCES " + parent + "(id) DEFERRABLE INITIALLY DEFERRED)").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS " + child).Error
		_ = db.Exec("DROP TABLE IF EXISTS " + parent).Error
	})
	const callback = "test:outbound-deferred-commit"
	if err := db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "outbound_traffics" {
			return
		}
		if result := tx.Session(&gorm.Session{NewDB: true}).Exec("INSERT INTO " + child + " (id, parent_id) VALUES (1, 999999)"); result.Error != nil {
			tx.AddError(result.Error)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })

	err, _ := (&OutboundService{}).AddTraffic([]*xray.Traffic{{Tag: "commit-test", IsOutbound: true, Up: 1}}, nil)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("AddTraffic error = %v, want deferred foreign-key commit failure", err)
	}
}
