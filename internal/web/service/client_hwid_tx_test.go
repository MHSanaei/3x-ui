package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

var errInjectedHwidDelete = errors.New("injected client_hwids delete failure")

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
			SubID: subID, HwidHash: fmt.Sprintf("%s-hash-%d", subID, i),
			FirstSeen: now, LastSeen: now + int64(i),
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_hwids for %q: %v", subID, err)
	}
}

func assertHwidState(t *testing.T, db *gorm.DB, email string, limit, devices int) {
	t.Helper()
	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if rec.LimitHwid != limit {
		t.Fatalf("limit_hwid = %d, want %d", rec.LimitHwid, limit)
	}
	var n int64
	if err := db.Model(&model.ClientHwid{}).Where("sub_id = ?", rec.SubID).Count(&n).Error; err != nil {
		t.Fatalf("count client_hwids: %v", err)
	}
	if n != int64(devices) {
		t.Fatalf("client_hwids = %d, want %d", n, devices)
	}
}

func TestSetClientLimitHwidRollsBackFailedTrim(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()
	rec := seedHwidClient(t, 5)
	seedHwids(t, db, rec.SubID, 3)
	failHwidDeletes(t, db)

	err := (&ClientService{}).setClientLimitHwidByEmail(rec.Email, 1)
	if !errors.Is(err, errInjectedHwidDelete) {
		t.Fatalf("want errInjectedHwidDelete, got: %v", err)
	}
	assertHwidState(t, db, rec.Email, 5, 3)
}

func TestClientHwidTxRejectsUnserializedHandle(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()
	rec := seedHwidClient(t, 5)
	svc := &ClientService{}

	if err := svc.setClientLimitHwidByEmailTx(db, rec.Email, 1); !errors.Is(err, errClientHwidWriteNotSerialized) {
		t.Fatalf("bare handle error = %v, want errClientHwidWriteNotSerialized", err)
	}
	if err := runSerializedTx(func(tx *gorm.DB) error {
		return svc.setClientLimitHwidByEmailTx(tx, rec.Email, 1)
	}); err != nil {
		t.Fatalf("serialized update: %v", err)
	}
	assertHwidState(t, db, rec.Email, 1, 0)
}

func TestBulkAdjustHwidRollsBackFailedTrim(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	rec := &model.ClientRecord{Email: "bulk-hwid@x", SubID: "bulk-sub", Enable: true, LimitHwid: 5}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	seedHwids(t, db, rec.SubID, 3)
	failHwidDeletes(t, db)

	limit := 1
	res, _, err := (&ClientService{}).BulkAdjust(&InboundService{}, []string{rec.Email}, 0, 0, "", &limit, "")
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != errInjectedHwidDelete.Error() {
		t.Fatalf("skipped = %+v, want injected failure", res.Skipped)
	}
	assertHwidState(t, db, rec.Email, 5, 3)
}

func TestBulkCreateWithdrawsTombstoneWhenHwidTrimFails(t *testing.T) {
	setupBulkDB(t)
	StartTrafficWriter()
	t.Cleanup(StopTrafficWriter)
	db := database.GetDB()
	const email = "reborn-bulk@x"
	const subID = "reborn-bulk-sub"
	tombstoneClientEmail(email)
	t.Cleanup(func() { withdrawClientTombstones(email) })
	seedHwids(t, db, subID, 3)
	failHwidDeletes(t, db)
	ib := mkInbound(t, 30441, model.VLESS, `{"clients":[]}`)

	res, _, err := (&ClientService{}).BulkCreate(&InboundService{}, []ClientCreatePayload{{
		Client: model.Client{
			Email: email, SubID: subID, ID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enable: true,
		},
		InboundIds: []int{ib.Id}, LimitHwid: 1,
	}})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != errInjectedHwidDelete.Error() {
		t.Fatalf("skipped = %+v, want injected HWID failure", res.Skipped)
	}
	if isClientEmailTombstoned(email) {
		t.Fatal("live bulk-created client retained a delete tombstone")
	}
}

func TestSetClientLimitHwidIsSerializedWithSyncInbound(t *testing.T) {
	db := durablePostgresDB(t)
	if err := db.Exec("TRUNCATE client_hwids, clients RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("reset tables: %v", err)
	}
	rec := seedHwidClient(t, 5)
	seedHwids(t, db, rec.SubID, 3)
	StartTrafficWriter()
	t.Cleanup(StopTrafficWriter)

	read := make(chan struct{})
	release := make(chan struct{})
	staleDone := make(chan error, 1)
	go func() {
		staleDone <- runSerializedTx(func(tx *gorm.DB) error {
			var stale model.ClientRecord
			if err := tx.Where("email = ?", rec.Email).First(&stale).Error; err != nil {
				return err
			}
			close(read)
			<-release
			return tx.Save(&stale).Error
		})
	}()
	<-read

	limitDone := make(chan error, 1)
	go func() { limitDone <- (&ClientService{}).setClientLimitHwidByEmail(rec.Email, 1) }()
	time.Sleep(100 * time.Millisecond)
	close(release)
	if err := <-staleDone; err != nil {
		t.Fatalf("stale SyncInbound write: %v", err)
	}
	if err := <-limitDone; err != nil {
		t.Fatalf("set limit: %v", err)
	}
	assertHwidState(t, db, rec.Email, 1, 1)
}

func BenchmarkSetClientLimitHwidSerialized(b *testing.B) {
	dbDir := b.TempDir()
	b.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		b.Fatalf("InitDB: %v", err)
	}
	b.Cleanup(func() { _ = database.CloseDB() })
	StartTrafficWriter()
	b.Cleanup(StopTrafficWriter)
	db := database.GetDB()
	emails := make([]string, 100)
	for i := range emails {
		emails[i] = fmt.Sprintf("bench-%03d@x", i)
		rec := &model.ClientRecord{Email: emails[i], SubID: fmt.Sprintf("bench-sub-%03d", i), Enable: true}
		if err := db.Create(rec).Error; err != nil {
			b.Fatalf("seed client: %v", err)
		}
	}
	svc := &ClientService{}
	for _, count := range []int{1, 100} {
		b.Run(fmt.Sprintf("clients_%d", count), func(b *testing.B) {
			for range b.N {
				for i := range count {
					if err := svc.setClientLimitHwidByEmail(emails[i], 2); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
