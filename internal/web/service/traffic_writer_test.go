package service

import (
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTrafficWriterStartStopStartAcceptsWrites(t *testing.T) {
	resetTrafficWriterForTest(t)

	StartTrafficWriter()
	var writes atomic.Int32
	if err := submitTrafficWrite(func() error {
		writes.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("first submitTrafficWrite: %v", err)
	}

	StopTrafficWriter()
	StartTrafficWriter()
	if err := submitTrafficWrite(func() error {
		writes.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("second submitTrafficWrite: %v", err)
	}

	if got := writes.Load(); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
}

func TestTrafficWriterSubmitAfterStopRunsInline(t *testing.T) {
	resetTrafficWriterForTest(t)

	StartTrafficWriter()
	StopTrafficWriter()

	ran := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- submitTrafficWrite(func() error {
			close(ran)
			return nil
		})
	}()

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("submitTrafficWrite did not run after traffic writer stopped")
	}
	if err := waitTrafficWriterErr(t, errCh); err != nil {
		t.Fatalf("submitTrafficWrite after stop: %v", err)
	}
}

func TestTrafficWriterStopDrainsQueuedWrite(t *testing.T) {
	resetTrafficWriterForTest(t)

	StartTrafficWriter()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstErr := make(chan error, 1)
	go func() {
		firstErr <- submitTrafficWrite(func() error {
			close(firstStarted)
			<-releaseFirst
			return nil
		})
	}()
	waitTrafficWriterSignal(t, firstStarted, "first write did not start")

	secondRan := make(chan struct{})
	secondErr := make(chan error, 1)
	go func() {
		secondErr <- submitTrafficWrite(func() error {
			close(secondRan)
			return nil
		})
	}()
	waitTrafficWriterQueued(t)

	stopDone := make(chan struct{})
	go func() {
		StopTrafficWriter()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		t.Fatal("StopTrafficWriter returned before in-flight write was released")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)
	waitTrafficWriterSignal(t, stopDone, "StopTrafficWriter did not return")
	waitTrafficWriterSignal(t, secondRan, "queued write was not drained")

	if err := waitTrafficWriterErr(t, firstErr); err != nil {
		t.Fatalf("first submitTrafficWrite: %v", err)
	}
	if err := waitTrafficWriterErr(t, secondErr); err != nil {
		t.Fatalf("second submitTrafficWrite: %v", err)
	}
}

func TestTrafficWriterConcurrentStopDuringSubmitDoesNotHang(t *testing.T) {
	resetTrafficWriterForTest(t)

	StartTrafficWriter()
	started := make(chan struct{})
	release := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- submitTrafficWrite(func() error {
			close(started)
			<-release
			return nil
		})
	}()
	waitTrafficWriterSignal(t, started, "write did not start")

	stopDone := make(chan struct{})
	go func() {
		StopTrafficWriter()
		close(stopDone)
	}()

	close(release)
	waitTrafficWriterSignal(t, stopDone, "StopTrafficWriter hung during submit")
	if err := waitTrafficWriterErr(t, errCh); err != nil {
		t.Fatalf("submitTrafficWrite during stop: %v", err)
	}
}

func resetTrafficWriterForTest(t *testing.T) {
	t.Helper()
	StopTrafficWriter()
	twMu.Lock()
	clearTrafficWriterState()
	twMu.Unlock()
	t.Cleanup(func() {
		StopTrafficWriter()
		twMu.Lock()
		clearTrafficWriterState()
		twMu.Unlock()
	})
}

func waitTrafficWriterQueued(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		twMu.Lock()
		queued := 0
		if twQueue != nil {
			queued = len(twQueue)
		}
		twMu.Unlock()
		if queued > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("write was not queued")
}

func waitTrafficWriterSignal(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

func waitTrafficWriterErr(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for traffic writer result")
		return nil
	}
}

// parkSerializedWriter holds the single writer inside an open transaction, so a write handed to it stays queued.
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

// waitForQueuedWrite reports whether a write reached the writer queue instead of opening its own transaction.
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

// The poll and an admin mutation lock inbounds and client_traffics in opposite order, so a delete in its
// own transaction can deadlock the poll on Postgres (40P01) and resurrect an inbound an edit just saved.
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

// The purge writes inbounds and client_traffics in the opposite order to the poll, so off the writer it
// can deadlock the poll on Postgres (40P01).
func TestDelDepletedClientsRunsOnTheSerialWriter(t *testing.T) {
	startSerializedWriter(t)
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	clients := []model.Client{{Email: "depleted@x", ID: "aaaaaaaa-0000-0000-0000-000000000301", Enable: true}}
	ib := mkInbound(t, 54102, model.VLESS, clientsSettings(t, clients))
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "depleted@x", 60, 60, 100, 0, true)

	release := parkSerializedWriter(t)
	done := make(chan error, 1)
	go func() { done <- svc.DelDepletedClients(-1) }()

	queued := waitForQueuedWrite()
	release()
	if err := waitWriterResult(t, done); err != nil {
		t.Fatalf("DelDepletedClients: %v", err)
	}
	if !queued {
		t.Fatal("the depleted-client purge bypassed the serial traffic writer and opened its own transaction")
	}
	var inbounds int64
	if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Count(&inbounds).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if inbounds != 0 {
		t.Fatalf("depleted inbound %d still present after the purge", ib.Id)
	}
	var traffics int64
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", "depleted@x").Count(&traffics).Error; err != nil {
		t.Fatalf("count traffic rows: %v", err)
	}
	if traffics != 0 {
		t.Fatalf("traffic rows for the depleted client = %d, want 0", traffics)
	}
}
