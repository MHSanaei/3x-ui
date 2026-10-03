package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A reset of a quota-disabled client must end enabled even when a traffic tick
// runs between its steps: enabling before zeroing lets the tick re-disable it.
func TestResetTrafficOfDepletedClientSurvivesTickMidReset(t *testing.T) {
	resets := []struct {
		name string
		run  func() error
	}{
		{"single", func() error {
			_, err := (&ClientService{}).ResetTrafficByEmail(&InboundService{}, "d@stale")
			return err
		}},
		{"bulk", func() error {
			_, err := (&ClientService{}).BulkResetTraffic(&InboundService{}, []string{"d@stale"})
			return err
		}},
	}
	for _, reset := range resets {
		t.Run(reset.name, func(t *testing.T) {
			requireResetSurvivesTick(t, reset.run)
		})
	}
}

func requireResetSurvivesTick(t *testing.T, reset func() error) {
	t.Helper()
	setupBulkDB(t)
	clients := []model.Client{{Email: "d@stale", ID: "aaaaaaaa-0000-0000-0000-00000000000d", SubID: "sub-d", Enable: false, TotalGB: 1000}}
	ib := mkInbound(t, 23140, model.VLESS, clientsSettings(t, clients))
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	row := xray.ClientTraffic{InboundId: ib.Id, Email: "d@stale", Enable: false, Up: 600, Down: 400, Total: 1000}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	resetTrafficWriterForTest(t)
	StartTrafficWriter()
	parked := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = submitTrafficWrite(func() error {
			close(parked)
			<-release
			return nil
		})
	}()
	<-parked

	resetDone := make(chan error, 1)
	go func() { resetDone <- reset() }()
	waitTrafficWriterQueued(t)
	tickDone := make(chan error, 1)
	go func() {
		_, _, err := (&InboundService{}).AddTraffic(nil, nil)
		tickDone <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for len(twQueue) < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err := <-resetDone; err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := <-tickDone; err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var after xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "d@stale").First(&after).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	c, _ := settingsClient(t, ib.Id, "d@stale")
	rec := lookupClientRecord(t, "d@stale")
	if !after.Enable || !c.Enable || !rec.Enable || after.Up+after.Down != 0 {
		t.Fatalf("after reset: traffic enable=%v used=%d, settings enable=%v, record enable=%v; want all enabled at 0",
			after.Enable, after.Up+after.Down, c.Enable, rec.Enable)
	}
}
