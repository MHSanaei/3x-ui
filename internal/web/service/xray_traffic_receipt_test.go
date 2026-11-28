package service

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestXrayTrafficBatchSurvivesFullQueueAndWriterRestart(t *testing.T) {
	testXrayTrafficBatchSurvivesFullQueueAndWriterRestart(t)
}

func TestXrayTrafficBatchSurvivesFullQueueAndWriterRestartPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	testXrayTrafficBatchSurvivesFullQueueAndWriterRestart(t)
}

func testXrayTrafficBatchSurvivesFullQueueAndWriterRestart(t *testing.T) {
	t.Helper()
	db := initTrafficTestDB(t)
	resetTrafficWriterForTest(t)
	ib := model.Inbound{Tag: "queue-retry", Protocol: model.VLESS, Enable: true, Settings: "{}"}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	client := xray.ClientTraffic{InboundId: ib.Id, Email: "queue-retry@example", Enable: true}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	assertTotals := func(want int64) {
		t.Helper()
		var inbound model.Inbound
		var traffic xray.ClientTraffic
		if err := db.First(&inbound, ib.Id).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&traffic, client.Id).Error; err != nil {
			t.Fatal(err)
		}
		if inbound.Up != want || traffic.Up != want {
			t.Fatalf("inbound/client up = %d/%d, want %d/%d", inbound.Up, traffic.Up, want, want)
		}
	}

	StartTrafficWriter()
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- submitTrafficWrite(func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	waitTrafficWriterSignal(t, entered, "writer did not pick up the blocking request")
	twMu.Lock()
	for len(twBgQueue) < cap(twBgQueue) {
		twBgQueue <- &trafficWriteRequest{apply: func() error { return nil }, done: make(chan error, 1)}
	}
	twMu.Unlock()

	tags := []*xray.Traffic{{IsInbound: true, Tag: ib.Tag, Up: 19}}
	clients := []*xray.ClientTraffic{{Email: client.Email, Up: 19}}
	add := func(id string) error {
		_, _, err := (&InboundService{}).AddXrayTrafficBatch(id, tags, clients)
		return err
	}
	if err := add("original-sample"); err == nil || !strings.Contains(err.Error(), "traffic writer queue full") {
		t.Fatalf("saturated writer returned %v, want queue full", err)
	}
	assertTotals(0)
	unblock()
	if err := waitTrafficWriterErr(t, writeDone); err != nil {
		t.Fatal(err)
	}
	StopTrafficWriter()
	StartTrafficWriter()
	if err := add("original-sample"); err != nil {
		t.Fatal(err)
	}
	assertTotals(19)

	// A commit response can be lost. Retrying its identity must preserve the totals.
	StopTrafficWriter()
	StartTrafficWriter()
	if err := add("original-sample"); err != nil {
		t.Fatal(err)
	}
	assertTotals(19)
	if err := add("next-sample"); err != nil {
		t.Fatal(err)
	}
	assertTotals(38)
}
