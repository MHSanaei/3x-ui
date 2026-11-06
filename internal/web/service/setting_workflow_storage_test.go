package service

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestPublicSettingsDoNotMaterializePrivateWorkflowReceipts(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	rows := make([]model.Setting, 501)
	rows[0] = model.Setting{Key: "webPort", Value: "3456"}
	for i := 1; i < len(rows); i++ {
		rows[i] = model.Setting{Key: fmt.Sprintf("%s20261029-%04d", clientOperationSettingPrefix, i), Value: strings.Repeat("x", 1024)}
	}
	if err := db.CreateInBatches(rows, 200).Error; err != nil {
		t.Fatal(err)
	}
	loaded := 0
	privateLoaded := 0
	callback := "test:public-settings-materialization"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "settings" {
			return
		}
		if values, ok := tx.Statement.Dest.(*[]*model.Setting); ok {
			loaded += len(*values)
			for _, value := range *values {
				if strings.HasPrefix(value.Key, clientOperationSettingPrefix) {
					privateLoaded++
				}
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	settings, err := (&SettingService{}).GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.WebPort != 3456 {
		t.Fatalf("public setting lost: %d", settings.WebPort)
	}
	if privateLoaded != 0 {
		t.Fatalf("public settings materialized %d private workflow receipts among %d rows; want zero private receipts", privateLoaded, loaded)
	}
}
