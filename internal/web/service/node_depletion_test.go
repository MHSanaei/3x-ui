package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeDepletionPreservesOriginAndOlderNodes(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	at := time.Now().Add(-time.Minute).UnixMilli()
	cs := xray.ClientTraffic{Email: "origin", Enable: true, Total: 100}
	syncNode(t, svc, 1, "n1-in", cs)
	cs.Up, cs.Enable, cs.DepletedAt, cs.DepletedReason = 100, false, at, "traffic"
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, at, "traffic")
	cs.DepletedAt, cs.DepletedReason = 0, ""
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, at, "traffic")
	cs.DepletedAt, cs.DepletedReason = at+1000, "traffic"
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, at, "traffic")
}

func TestNodeDepletionMetadataOnlyAndAdoption(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	at := time.Now().Add(-time.Minute).UnixMilli()
	cs := xray.ClientTraffic{Email: "metadata-only", Total: 100, Up: 100, Enable: false}
	if err := db.Create(&xray.ClientTraffic{Email: cs.Email, Total: 100, Up: 100}).Error; err != nil {
		t.Fatal(err)
	}
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, 0, "traffic")
	cs.DepletedAt, cs.DepletedReason = at, "traffic"
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, at, "traffic")
	cs.Email = "adopted"
	if _, err := svc.setRemoteTrafficLocked(1, &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
		{Tag: "new-inbound", Port: 41002, Protocol: model.VLESS, ClientStats: []xray.ClientTraffic{cs}},
	}}, false, false); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, cs.Email, at, "traffic")
}

func TestNodeDepletionRejectsInvalidEventsAndPreservesExpiry(t *testing.T) {
	for _, event := range []string{"traffic", "expiry", "both", "older node", "future", "negative", "earlier traffic"} {
		t.Run(event, func(t *testing.T) {
			db := initDepletionTestDB(t)
			createNodeInbound(t, db, 1, "n1-in", 41001)
			svc := &InboundService{}
			at := time.Now().Add(-time.Minute).UnixMilli()
			cs := xray.ClientTraffic{Email: "invalid-event", Enable: true, Total: 100, ExpiryTime: at}
			syncNode(t, svc, 1, "n1-in", cs)
			cs.Up, cs.Enable, cs.DepletedAt, cs.DepletedReason = 100, false, at, event
			want := at
			switch event {
			case "older node":
				cs.DepletedAt, cs.DepletedReason, want = 0, "", 0
			case "future":
				cs.DepletedAt, cs.DepletedReason = time.Now().Add(time.Hour).UnixMilli(), "traffic"
				want = 0
			case "negative":
				cs.DepletedAt, want = -1, 0
			case "earlier traffic":
				cs.DepletedAt, cs.DepletedReason = at-1000, "expiry"
				want = at - 1000
			}
			syncNode(t, svc, 1, "n1-in", cs)
			assertDepletion(t, db, cs.Email, want, "both")
		})
	}
}

func TestNodeDepletionDoesNotRestoreAfterRecovery(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	at := time.Now().Add(-time.Minute).UnixMilli()
	cs := xray.ClientTraffic{Email: "stale-event", Total: 100, Up: 100, DepletedAt: at, DepletedReason: "traffic"}
	syncNode(t, svc, 1, "n1-in", cs)
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", cs.Email).
		Updates(map[string]any{"total": 1000, "enable": true, "depleted_at": 0}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.setRemoteTrafficLocked(1, &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{Tag: "n1-in", ClientStats: []xray.ClientTraffic{cs}}}}, true, false); err != nil {
		t.Fatal(err)
	}
	assertDepletion(t, db, cs.Email, 0, "")
}

func TestNodeRenewalClearsDepletion(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	cs := xray.ClientTraffic{
		Email: "renewed-event", Up: 100, Total: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays,
		DepletedAt: time.Now().Add(-time.Hour).UnixMilli(), DepletedReason: "traffic",
	}
	syncNode(t, svc, 1, "n1-in", cs)
	cs.Up, cs.ExpiryTime, cs.Enable = 0, renewSecondExpiry, true
	cs.DepletedAt, cs.DepletedReason = 0, ""
	syncNode(t, svc, 1, "n1-in", cs)
	assertDepletion(t, db, cs.Email, 0, "")
}

func TestNodeRenewalKeepsNewPeriodEventWhenMasterMissesRecovery(t *testing.T) {
	for _, olderNode := range []bool{false, true} {
		t.Run(fmt.Sprintf("older_node_%t", olderNode), func(t *testing.T) {
			db := initDepletionTestDB(t)
			createNodeInbound(t, db, 1, "n1-in", 41001)
			svc := &InboundService{}
			oldAt := time.Now().Add(-time.Hour).UnixMilli()
			cs := xray.ClientTraffic{
				Email: "renewed-again", Up: 100, Total: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays,
				DepletedAt: oldAt, DepletedReason: "traffic",
			}
			syncNode(t, svc, 1, "n1-in", cs)
			cs.ExpiryTime, cs.ResetCount = renewSecondExpiry, 1
			cs.DepletedAt = oldAt + 1000
			if olderNode {
				cs.DepletedAt, cs.DepletedReason = 0, ""
			}
			syncNode(t, svc, 1, "n1-in", cs)
			assertDepletion(t, db, cs.Email, cs.DepletedAt, "traffic")
		})
	}
}
