package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

var errInjectedHwidDelete = errors.New("injected client_hwids delete failure")

// Only the trim is poisoned, so the limit update that precedes it still lands
// and the rollback has something to undo.
func failHwidDeletes(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Callback().Delete().Before("gorm:delete").Register("t:hwid:fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "client_hwids" {
			_ = tx.AddError(errInjectedHwidDelete)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Delete().Remove("t:hwid:fail"); err != nil {
			t.Fatalf("remove callback: %v", err)
		}
	})
}

func seedHwids(t *testing.T, db *gorm.DB, subID string, n int) {
	t.Helper()
	now := time.Now().UnixMilli()
	rows := make([]model.ClientHwid, 0, n)
	for i := range n {
		rows = append(rows, model.ClientHwid{
			SubID:     subID,
			HwidHash:  fmt.Sprintf("%s-hash-%d", subID, i),
			FirstSeen: now,
			LastSeen:  now + int64(i),
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_hwids for %q: %v", subID, err)
	}
}

func assertLimitUnchanged(t *testing.T, db *gorm.DB, email string, want int) {
	t.Helper()
	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if rec.LimitHwid != want {
		t.Fatalf("limit_hwid = %d, want %d: the limit update was not rolled back", rec.LimitHwid, want)
	}
}

// Create, Update and BulkCreate all pass nil, so a failed trim used to commit
// the new limit anyway and leave the subscription over it.
func TestSetClientLimitHwidRollsBackFailedTrim(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()

	rec := seedHwidClient(t, 5)
	seedHwids(t, db, rec.SubID, 3)
	failHwidDeletes(t, db)

	svc := &ClientService{}
	if err := svc.setClientLimitHwidByEmail(nil, rec.Email, 1); !errors.Is(err, errInjectedHwidDelete) {
		t.Fatalf("want errInjectedHwidDelete, got: %v", err)
	}

	assertLimitUnchanged(t, db, rec.Email, 5)

	var n int64
	if err := db.Model(&model.ClientHwid{}).Count(&n).Error; err != nil {
		t.Fatalf("count client_hwids: %v", err)
	}
	if n != 3 {
		t.Fatalf("client_hwids = %d, want 3", n)
	}
}

// BulkAdjust used to hand down a bare handle, which commits per statement just
// as nil did, so the guard cannot key on nil alone.
func TestSetClientLimitHwidRollsBackWithBareHandle(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()

	if carriesOpenTx(db) {
		t.Fatal("a bare handle was reported as an open transaction")
	}

	rec := seedHwidClient(t, 5)
	seedHwids(t, db, rec.SubID, 3)
	failHwidDeletes(t, db)

	svc := &ClientService{}
	if err := svc.setClientLimitHwidByEmail(db, rec.Email, 1); !errors.Is(err, errInjectedHwidDelete) {
		t.Fatalf("want errInjectedHwidDelete, got: %v", err)
	}

	assertLimitUnchanged(t, db, rec.Email, 5)
}

// The whole guard hangs on this predicate, so pin the shapes it must tell
// apart: no handle, a shared handle, and a handle that has begun work.
func TestCarriesOpenTx(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()

	if carriesOpenTx(nil) {
		t.Fatal("nil was reported as an open transaction")
	}
	if carriesOpenTx(db) {
		t.Fatal("a bare handle was reported as an open transaction")
	}
	if carriesOpenTx(db.Session(&gorm.Session{})) {
		t.Fatal("a session over a bare handle was reported as an open transaction")
	}

	seen := false
	if err := db.Transaction(func(tx *gorm.DB) error {
		seen = carriesOpenTx(tx)
		return nil
	}); err != nil {
		t.Fatalf("transaction: %v", err)
	}
	if !seen {
		t.Fatal("inside a transaction the predicate returned false")
	}
}
