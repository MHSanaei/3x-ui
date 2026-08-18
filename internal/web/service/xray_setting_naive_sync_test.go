package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initNaiveSyncDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func TestSyncNaiveOutboundsTx_DeleteRemovesDBAndMarksStopped(t *testing.T) {
	initNaiveSyncDB(t)
	db := database.GetDB()

	if err := db.Create(&model.NaiveOutbound{
		Tag:       "naive-old",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30000,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create naive record: %v", err)
	}

	payload := `{"outbounds":[{"tag":"direct","protocol":"freedom"}]}`
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	result, err := syncNaiveOutboundsTx(tx, payload)
	if err != nil {
		tx.Rollback()
		t.Fatalf("syncNaiveOutboundsTx: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(result.stopped) != 1 || result.stopped[0] != "naive-old" {
		t.Fatalf("stopped = %#v", result.stopped)
	}
	if len(result.started) != 0 || len(result.restarted) != 0 {
		t.Fatalf("unexpected started/restarted: started=%#v restarted=%#v", result.started, result.restarted)
	}

	var count int64
	if err := db.Model(&model.NaiveOutbound{}).Where("tag = ?", "naive-old").Count(&count).Error; err != nil {
		t.Fatalf("count old tag: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected old tag removed, count=%d", count)
	}
}

func TestSyncNaiveOutboundsTx_RetagCreatesNewAndRemovesOld(t *testing.T) {
	initNaiveSyncDB(t)
	db := database.GetDB()

	if err := db.Create(&model.NaiveOutbound{
		Tag:       "naive-old",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30000,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create naive record: %v", err)
	}

	payload := `{"outbounds":[{"tag":"naive-new","protocol":"naive","settings":{"proxy":"https://user:pass@example.com:443"}}]}`
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	result, err := syncNaiveOutboundsTx(tx, payload)
	if err != nil {
		tx.Rollback()
		t.Fatalf("syncNaiveOutboundsTx: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(result.started) != 1 || result.started[0] != "naive-new" {
		t.Fatalf("started = %#v", result.started)
	}
	if len(result.stopped) != 1 || result.stopped[0] != "naive-old" {
		t.Fatalf("stopped = %#v", result.stopped)
	}
	if len(result.restarted) != 0 {
		t.Fatalf("restarted = %#v", result.restarted)
	}

	var oldCount int64
	if err := db.Model(&model.NaiveOutbound{}).Where("tag = ?", "naive-old").Count(&oldCount).Error; err != nil {
		t.Fatalf("count old tag: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("expected old tag removed, count=%d", oldCount)
	}

	var newRecord model.NaiveOutbound
	if err := db.Where("tag = ?", "naive-new").First(&newRecord).Error; err != nil {
		t.Fatalf("find new tag: %v", err)
	}
	if newRecord.LocalPort <= 0 {
		t.Fatalf("invalid local port: %d", newRecord.LocalPort)
	}
	if !newRecord.Enabled {
		t.Fatalf("expected new tag enabled")
	}
}
