package service

import (
	"fmt"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// createNodeWithMultiplier persists the node row setRemoteTrafficLocked reads
// its billed-traffic multiplier from. A zero multiplier exercises the GORM
// default (100 = 1x) exactly like a node created through the API without the
// operator touching the field.
func createNodeWithMultiplier(t *testing.T, db *gorm.DB, nodeID int, multiplier int64) {
	t.Helper()
	n := &model.Node{
		Id:                nodeID,
		Name:              fmt.Sprintf("mult-node-%d", nodeID),
		Address:           "127.0.0.1",
		Port:              2053,
		ApiToken:          "token",
		TrafficMultiplier: multiplier,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("create node %d: %v", nodeID, err)
	}
}

func setNodeMultiplier(t *testing.T, db *gorm.DB, nodeID int, multiplier int64) {
	t.Helper()
	if err := db.Model(&model.Node{}).Where("id = ?", nodeID).
		Update("traffic_multiplier", multiplier).Error; err != nil {
		t.Fatalf("set node %d multiplier: %v", nodeID, err)
	}
}

func TestNodeMultiplier_DefaultIs1x(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 0) // GORM default kicks in
	svc := &InboundService{}

	var stored int64
	if err := db.Raw("SELECT traffic_multiplier FROM nodes WHERE id = ?", 1).Scan(&stored).Error; err != nil {
		t.Fatalf("read stored multiplier: %v", err)
	}
	if stored != 100 {
		t.Fatalf("stored multiplier = %d, want the GORM default 100", stored)
	}

	const email = "mult-default"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 100, 100, "default multiplier bills 1:1")
}

func TestNodeMultiplier_ScalesDelta(t *testing.T) {
	const MB = int64(1024 * 1024)
	cases := []struct {
		name       string
		mult       int64
		wantBilled int64
	}{
		{"legacy sub-1x heals to 1x", 50, 100 * MB},
		{"1x", 100, 100 * MB},
		{"1.5x", 150, 150 * MB},
		{"2x", 200, 200 * MB},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := initTrafficTestDB(t)
			createNodeInbound(t, db, 1, "n1-in", 41001)
			createNodeWithMultiplier(t, db, 1, c.mult)
			svc := &InboundService{}

			email := "mult-" + c.name
			// First sync seeds the row at 0 and baselines the raw 100MB.
			syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100 * MB, Down: 100 * MB, Enable: true})
			assertUpDown(t, readTraffic(t, db, email), 0, 0, "baseline sync never bills history")

			// Second sync: raw delta 100MB is billed at the node's multiplier.
			syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200 * MB, Down: 200 * MB, Enable: true})
			assertUpDown(t, readTraffic(t, db, email), c.wantBilled, c.wantBilled, "delta billed at multiplier")
		})
	}
}

// A re-delivered snapshot produces delta 0 — the baseline tracks raw counters,
// so nothing is billed again no matter how large the multiplier is.
func TestNodeMultiplier_RepeatedSnapshotNoDoubleBill(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 200)
	svc := &InboundService{}

	const email = "mult-repeat"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 200, 200, "first delta billed at 2x")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 200, 200, "repeated snapshot must not re-bill")
}

// Raising the multiplier scales only deltas produced after the change; the
// already-billed total is never recalculated.
func TestNodeMultiplier_ChangeAffectsFutureOnly(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 100)
	svc := &InboundService{}

	const email = "mult-change"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 100, 100, "delta at 1x")

	setNodeMultiplier(t, db, 1, 200)
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 300, Down: 300, Enable: true})
	// 100 (billed at 1x) + 100*2 (billed at 2x) = 300, not 200*2 = 400.
	assertUpDown(t, readTraffic(t, db, email), 300, 300, "history not recalculated after 1x -> 2x")
}

// Two nodes sharing one email each bill their own delta at their own
// multiplier; neither re-bills the other's traffic.
func TestNodeMultiplier_MultiNodeIndependent(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeInbound(t, db, 2, "n2-in", 41002)
	createNodeWithMultiplier(t, db, 1, 200) // node A: 2x
	createNodeWithMultiplier(t, db, 2, 100) // node B: 1x
	svc := &InboundService{}

	const email = "mult-shared"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "baselines never bill history")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true}) // delta 100 x2 = 200
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 150, Down: 150, Enable: true}) // delta 50 x1 = 50
	assertUpDown(t, readTraffic(t, db, email), 250, 250, "each node's delta billed at its own multiplier")
}

// A node-side counter reset is rebaselined, not billed — with any multiplier.
func TestNodeMultiplier_CounterResetBillsNothing(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 200)
	svc := &InboundService{}

	const email = "mult-reset"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 900, Down: 900, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 950, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 100, 100, "delta 50 at 2x")

	// Node reboot drops the counter to 50: negative delta bills zero.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 50, Down: 50, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 100, 100, "counter reset must not bill")

	// Post-reset accrual resumes at 2x from the rebaselined counter.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 80, Down: 80, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 160, 160, "post-reset delta 30 at 2x")
}

// A central reset clears the per-node baseline: the node's stale cumulative
// counter must not be re-billed at the multiplier afterwards.
func TestNodeMultiplier_CentralResetNoHistoricalRebill(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 200)
	StartTrafficWriter()
	svc := &InboundService{}

	const email = "mult-central-reset"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 300, Down: 300, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 400, 400, "delta 200 at 2x")

	if err := svc.ResetClientTrafficByEmail(email); err != nil {
		t.Fatalf("ResetClientTrafficByEmail: %v", err)
	}
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "right after reset")

	// Stale pre-reset counter rebaselines silently — no rebill of history.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 340, Down: 340, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "stale node counter must not revert the reset")

	// Genuine post-reset usage bills at 2x: (370-340)*2 = 60.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 370, Down: 370, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 60, 60, "post-reset usage billed at 2x")
}

// A manual updateTraffic writes the billed counter directly — that column IS
// the billed total. The raw baseline is untouched, so later node syncs keep
// billing only fresh deltas at the multiplier on top of the manual value.
func TestNodeMultiplier_ManualUpdateKeepsBilledSemantics(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 200)
	StartTrafficWriter()
	svc := &InboundService{}

	const email = "mult-manual"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 200, 200, "delta 100 at 2x")

	// Operator correction sets the billed total directly.
	if err := svc.UpdateClientTrafficByEmail(email, 1000, 500); err != nil {
		t.Fatalf("UpdateClientTrafficByEmail: %v", err)
	}
	assertUpDown(t, readTraffic(t, db, email), 1000, 500, "manual write sets the billed total")

	// Re-delivered snapshot produces delta 0 — the manual value is untouched.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1000, 500, "no re-bill after manual update")

	// Fresh usage bills at 2x on top of the manual base: +50*2.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 250, Down: 250, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1100, 600, "post-update delta billed at 2x on manual base")
}

// A node-side renewal adopts the node's fresh counters billed at the node's
// multiplier, then keeps scaling later deltas in the new window.
func TestNodeMultiplier_RenewalBillsAtMultiplier(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeWithMultiplier(t, db, 1, 200)
	svc := &InboundService{}

	const email = "mult-renew"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 150, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1900, 300, "first window billed at 2x")

	// Renewal: adopt canon (5/2) billed at 2x -> (10/4).
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 5, Down: 2, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 10, 4, "renewed counters billed at 2x")

	// New-window delta (15-5=10 / 12-2=10) at 2x -> +20/+20.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 15, Down: 12, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 30, 24, "new window accrues at 2x")
}

// The multiplier is billing state, not node config: an edit that only moves it
// persists the new value without flagging the node dirty, so a billing change
// never re-pushes config or reconnects the node. A real config edit must still
// flag it. Both write paths share nodeConfigChanged, so both are exercised.
func TestNodeMultiplier_UpdateKeepsConfigDirtyClean(t *testing.T) {
	createNode := func(t *testing.T, db *gorm.DB, name string) {
		t.Helper()
		n := &model.Node{
			Id: 1, Name: name, Scheme: "https", Address: "127.0.0.1", Port: 2053,
			BasePath: "/", ApiToken: "token", Enable: true, TlsVerifyMode: "verify",
			InboundSyncMode: "all", TrafficMultiplier: 100,
		}
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}
	readNode := func(t *testing.T, db *gorm.DB) (int64, bool) {
		t.Helper()
		var row struct {
			Multiplier int64
			Dirty      bool
		}
		if err := db.Raw("SELECT traffic_multiplier AS multiplier, config_dirty AS dirty FROM nodes WHERE id = 1").Scan(&row).Error; err != nil {
			t.Fatalf("read node: %v", err)
		}
		return row.Multiplier, row.Dirty
	}

	t.Run("Update", func(t *testing.T) {
		db := initTrafficTestDB(t)
		createNode(t, db, "dirty-node")
		svc := &NodeService{}

		in := &model.Node{
			Name: "dirty-node", Scheme: "https", Address: "127.0.0.1", Port: 2053,
			BasePath: "/", Enable: true, TlsVerifyMode: "verify", InboundSyncMode: "all",
			TrafficMultiplier: 200,
		}
		if err := svc.Update(1, in); err != nil {
			t.Fatalf("Update: %v", err)
		}
		mult, dirty := readNode(t, db)
		if mult != 200 {
			t.Fatalf("stored multiplier = %d, want 200", mult)
		}
		if dirty {
			t.Fatal("a multiplier-only edit must not flag the node dirty")
		}

		// Positive control: a genuine config edit still flags it.
		in.Name = "dirty-node-renamed"
		if err := svc.Update(1, in); err != nil {
			t.Fatalf("Update (rename): %v", err)
		}
		if _, dirty := readNode(t, db); !dirty {
			t.Fatal("a config edit must still flag the node dirty")
		}
	})

	t.Run("UpdateFromRequest", func(t *testing.T) {
		db := initTrafficTestDB(t)
		createNode(t, db, "dirty-req")
		svc := &NodeService{}

		mult := int64(150)
		req := &NodeMutationRequest{
			Name: "dirty-req", Scheme: "https", Address: "127.0.0.1", Port: 2053,
			BasePath: "/", Enable: true, TlsVerifyMode: "verify", InboundSyncMode: "all",
			TrafficMultiplier: &mult,
		}
		if err := svc.UpdateFromRequest(1, req); err != nil {
			t.Fatalf("UpdateFromRequest: %v", err)
		}
		stored, dirty := readNode(t, db)
		if stored != 150 {
			t.Fatalf("stored multiplier = %d, want 150", stored)
		}
		if dirty {
			t.Fatal("a multiplier-only request must not flag the node dirty")
		}

		req.Name = "dirty-req-renamed"
		if err := svc.UpdateFromRequest(1, req); err != nil {
			t.Fatalf("UpdateFromRequest (rename): %v", err)
		}
		if _, dirty := readNode(t, db); !dirty {
			t.Fatal("a config edit must still flag the node dirty")
		}
	})
}
