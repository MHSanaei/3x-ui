package dbtest

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInitDBGivesEachTestItsOwnDatabase(t *testing.T) {
	t.Run("first test writes", func(t *testing.T) {
		InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
		if err := database.GetDB().Create(&model.Setting{Key: "dbtestProbe", Value: "first"}).Error; err != nil {
			t.Fatalf("write probe: %v", err)
		}
	})

	t.Run("next test starts clean", func(t *testing.T) {
		InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
		var leaked int64
		if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", "dbtestProbe").Count(&leaked).Error; err != nil {
			t.Fatalf("count probe: %v", err)
		}
		if leaked != 0 {
			t.Fatalf("database holds %d probe rows written by the previous test; copies must not share state", leaked)
		}
		var admins int64
		if err := database.GetDB().Model(&model.User{}).Count(&admins).Error; err != nil {
			t.Fatalf("count users: %v", err)
		}
		if admins != 1 {
			t.Fatalf("users = %d, want the 1 seeded admin a fresh install has", admins)
		}
	})
}
