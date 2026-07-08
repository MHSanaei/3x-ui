package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	renewFirstExpiry  = int64(1893456000000)
	renewSecondExpiry = renewFirstExpiry + int64(2592000000)
	renewPeriodDays   = 30
)

// TestNodeRenew_ResetsMasterTraffic reproduces #5843: a node-side auto-renew
// extends the deadline and resets the node's counters, and the master must
// start a fresh quota window instead of keeping the old period's usage.
func TestNodeRenew_ResetsMasterTraffic(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renew-reset"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 150, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 950, 150, "before renewal")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 5, Down: 2, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	ct := readTraffic(t, db, email)
	assertUpDown(t, ct, 5, 2, "after renewal")
	if ct.ExpiryTime != renewSecondExpiry {
		t.Errorf("renewal expiry = %d, want %d", ct.ExpiryTime, renewSecondExpiry)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 15, Down: 12, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 15, 12, "accumulating in the new period")
}

// TestNodeRenew_ReenablesDepletedClient: the node re-enables a client it
// renews; the master's depleted-disable must lift with the fresh window.
func TestNodeRenew_ReenablesDepletedClient(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renew-enable"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 900, Down: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("enable", false).Error; err != nil {
		t.Fatalf("force-disable master row: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	if ct := readTraffic(t, db, email); !ct.Enable {
		t.Error("renewal must re-enable the master row when the node reports the client enabled")
	}
}

// TestNodeRenew_ClearsGlobalTraffic mirrors autoRenewClients: stale cross-panel
// pushes for the renewed email must not re-deplete the fresh window.
func TestNodeRenew_ClearsGlobalTraffic(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renew-global"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 900, Down: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "m1", Email: email, Up: 800, Down: 90}).Error; err != nil {
		t.Fatalf("seed global traffic: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 3, Down: 1, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})

	var cnt int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count global traffic: %v", err)
	}
	if cnt != 0 {
		t.Errorf("renewal must clear stale global-traffic rows, found %d", cnt)
	}
}

// TestNodeCounterDip_SameExpiry_KeepsTraffic: a plain counter dip (#5456) with
// no deadline movement stays on the clamp path even for a renewable client.
func TestNodeCounterDip_SameExpiry_KeepsTraffic(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "dip-only"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 150, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 50, Down: 10, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 950, 150, "dip without renewal")
}

// TestNodeExpiryAdvance_RisingCounters_Accumulates: a manual deadline extension
// while traffic keeps flowing is not a renewal and must keep accumulating.
func TestNodeExpiryAdvance_RisingCounters_Accumulates(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "extend-only"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 50, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 160, Down: 90, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 160, 90, "extension while accumulating")
}
