package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSyncInboundClientWritesTimestampOnce(t *testing.T) {
	testSyncInboundClientWritesTimestampOnce(t, initTrafficTestDB(t))
}

func TestSyncInboundClientWritesTimestampOncePostgres(t *testing.T) {
	testSyncInboundClientWritesTimestampOnce(t, durablePostgresDB(t))
}

func testSyncInboundClientWritesTimestampOnce(t *testing.T, db *gorm.DB) {
	t.Helper()
	tag := fmt.Sprintf("single-client-write-%d", time.Now().UnixNano())
	inbound := durableTestInbound(nil, tag, 42534)
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	svc := &ClientService{}
	client := model.Client{Email: tag + "@x", Enable: true, TotalGB: 200, Comment: "old", UpdatedAt: 600}
	if err := svc.SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}

	updates := 0
	const callback = "test:count-client-reconcile-writes"
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		sql := tx.Statement.SQL.String()
		if tx.Statement.Table == "clients" && strings.Contains(sql, "updated_at") {
			updates++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	assertClient := func(wantTimestamp int64, wantUpdates int) {
		t.Helper()
		var rec model.ClientRecord
		if err := db.Where("email = ?", client.Email).First(&rec).Error; err != nil {
			t.Fatal(err)
		}
		if rec.Enable || rec.TotalGB != 0 || rec.Comment != "" || rec.UpdatedAt != wantTimestamp {
			t.Fatalf("saved enabled/quota/comment/timestamp: %t/%d/%q/%d, want false/0/empty/%d", rec.Enable, rec.TotalGB, rec.Comment, rec.UpdatedAt, wantTimestamp)
		}
		if updates != wantUpdates {
			t.Fatalf("client UPDATEs = %d, want %d", updates, wantUpdates)
		}
		updates = 0
	}
	client.Enable, client.TotalGB, client.Comment, client.UpdatedAt = false, 0, "", 400
	if err := svc.SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	assertClient(600, 1)
	if err := svc.SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	assertClient(600, 0)
	client.UpdatedAt = 900
	if err := svc.SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	assertClient(900, 1)

	rollback := errors.New("roll back client change")
	err := db.Transaction(func(tx *gorm.DB) error {
		client.Comment = "rolled back"
		client.UpdatedAt = 1000
		if err := svc.SyncInbound(tx, inbound.Id, []model.Client{client}); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v", err)
	}
	assertClient(900, 1)
}
