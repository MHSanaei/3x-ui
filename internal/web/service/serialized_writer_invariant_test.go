package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// parkSerializedWriter blocks the single traffic-writer goroutine inside an open
// transaction, so a write handed to the writer stays visibly queued.
func parkSerializedWriter(t *testing.T) (release func()) {
	t.Helper()
	entered := make(chan struct{})
	releaseCh := make(chan struct{})
	go func() {
		_ = submitTrafficWrite(func() error {
			return database.GetDB().Transaction(func(tx *gorm.DB) error {
				close(entered)
				<-releaseCh
				return nil
			})
		})
	}()
	waitTrafficWriterSignal(t, entered, "serial writer did not enter its transaction")
	return func() { close(releaseCh) }
}

// waitForQueuedWrite reports whether a write reached the traffic writer's
// queue, i.e. it was handed to the single writer instead of running on its own
// goroutine with its own transaction.
func waitForQueuedWrite() bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		twMu.Lock()
		queued := len(twQueue) > 0
		twMu.Unlock()
		if queued {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func waitWriterResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the queued write to finish")
		return nil
	}
}

// TestDelInboundRunsOnTheSerialWriter pins an inbound deletion to the single
// traffic writer. The poll and an admin mutation take the inbounds and
// client_traffics rows in opposite order, so a delete that opens its own
// transaction can deadlock the poll on Postgres (SQLSTATE 40P01) and can
// resurrect an inbound a concurrent edit just saved.
func TestDelInboundRunsOnTheSerialWriter(t *testing.T) {
	startSerializedWriter(t)
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	ib := mkInbound(t, 54101, model.VLESS, `{"clients": []}`)

	release := parkSerializedWriter(t)
	done := make(chan error, 1)
	go func() {
		_, _, err := svc.delInbound(ib.Id)
		done <- err
	}()

	queued := waitForQueuedWrite()
	release()
	if err := waitWriterResult(t, done); err != nil {
		t.Fatalf("delInbound: %v", err)
	}
	if !queued {
		t.Fatal("the inbound delete bypassed the serial traffic writer and opened its own transaction")
	}
	var remaining int64
	if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Count(&remaining).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("inbound %d still present after the delete", ib.Id)
	}
}
