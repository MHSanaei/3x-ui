package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestMergeActivationExpiry covers the pure reconciliation rule in isolation.
func TestMergeActivationExpiry(t *testing.T) {
	const (
		dur   = int64(-2592000000) // 30 days as a "start after first connect" duration
		early = int64(1000)        // earliest absolute deadline (first connection)
		late  = int64(2000)        // a later absolute deadline
	)
	cases := []struct {
		name           string
		existing, node int64
		want           int64
	}{
		{"master unset takes node duration", 0, dur, dur},
		{"master unset takes node activation", 0, early, early},
		{"activation adopted over stored duration", dur, early, early},
		{"node still un-activated does not reset deadline", early, dur, early},
		{"node un-activated zero does not reset deadline", early, 0, early},
		{"node renewal extends the deadline forward", early, late, late},
		{"stale earlier absolute does not clobber later", late, early, late},
		{"both un-activated keep node value", dur, dur, dur},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mergeActivationExpiry(c.existing, c.node); got != c.want {
				t.Fatalf("mergeActivationExpiry(%d,%d) = %d, want %d", c.existing, c.node, got, c.want)
			}
		})
	}
}

func TestStaleNodeDisable(t *testing.T) {
	const (
		early = int64(1000)
		late  = int64(2000)
	)
	cases := []struct {
		name      string
		master    *xray.ClientTraffic
		nodeExp   int64
		wantStale bool
	}{
		{"nil master", nil, early, false},
		{"same absolute deadline", &xray.ClientTraffic{ExpiryTime: late}, late, false},
		{"node older than master after extend", &xray.ClientTraffic{ExpiryTime: late}, early, true},
		{"node newer renewal", &xray.ClientTraffic{ExpiryTime: early}, late, false},
		{"unactivated node", &xray.ClientTraffic{ExpiryTime: late}, -1, false},
		{"unlimited master", &xray.ClientTraffic{ExpiryTime: 0}, early, false},
		{"older expiry but master over quota", &xray.ClientTraffic{
			ExpiryTime: late, Total: 100, Up: 60, Down: 50,
		}, early, false},
		{"older expiry under quota still stale", &xray.ClientTraffic{
			ExpiryTime: late, Total: 100, Up: 10, Down: 10,
		}, early, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := staleNodeDisable(c.master, c.nodeExp); got != c.wantStale {
				t.Fatalf("staleNodeDisable(...) = %v, want %v", got, c.wantStale)
			}
		})
	}
}

// TestNodeFirstConnectExpiry_NotClobbered reproduces the multi-node bug: a
// client is attached to inbounds on two nodes with a "start after first connect"
// expiry. The client connects only on node 1, which activates an absolute
// deadline; node 2 never sees a connection and keeps reporting the negative
// duration. The shared per-email client_traffics row must hold the activated
// deadline — a later node-2 sync must not reset it back to "not started".
func TestNodeFirstConnectExpiry_NotClobbered(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeInbound(t, db, 2, "n2-in", 41002)
	svc := &InboundService{}

	const email = "delayed"
	const duration = int64(-2592000000) // 30 days, not yet started

	// Both nodes start out reporting the un-activated negative duration.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: duration, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: duration, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != duration {
		t.Fatalf("before any connection: expiry = %d, want %d", got, duration)
	}

	// Client connects on node 1: it activates an absolute deadline.
	const activated = int64(1893456000000) // some absolute ms timestamp
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, ExpiryTime: activated, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("after node 1 activation: expiry = %d, want %d", got, activated)
	}

	// Node 2 (no connection there) keeps reporting the negative duration. This
	// must NOT reset the activated deadline.
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, ExpiryTime: duration, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("node 2 clobbered the activated deadline: expiry = %d, want %d", got, activated)
	}

	// Subsequent node 1 syncs keep the same absolute deadline.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, ExpiryTime: activated, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("after further node 1 sync: expiry = %d, want %d", got, activated)
	}
}

// TestNodeFirstConnectExpiry_NotClobbered_WithSettings exercises the full
// production sync path — snapshots carrying real settings JSON, which drives the
// GetClients/SyncInbound branch inside setRemoteTrafficLocked — to prove that
// branch does not re-derive the per-email client_traffics.expiry_time from the
// node's (still negative) settings and undo the merge guard.
func TestNodeFirstConnectExpiry_NotClobbered_WithSettings(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "delayed")
	createNodeInboundWithClient(t, db, 2, "n2-in", 41002, "delayed")
	svc := &InboundService{}

	const email = "delayed"
	const duration = int64(-2592000000)
	const activated = int64(1893456000000)

	negSettings := `{"clients":[{"email":"delayed","enable":true,"expiryTime":-2592000000}]}`
	actSettings := `{"clients":[{"email":"delayed","enable":true,"expiryTime":1893456000000}]}`

	// Both nodes start un-activated.
	syncNodeWithSettings(t, svc, 1, "n1-in", negSettings, xray.ClientTraffic{Email: email, ExpiryTime: duration, Enable: true})
	syncNodeWithSettings(t, svc, 2, "n2-in", negSettings, xray.ClientTraffic{Email: email, ExpiryTime: duration, Enable: true})

	// Node 1 activates (both its ClientStats and its settings now carry the
	// absolute deadline, like a real node after adjustTraffics).
	syncNodeWithSettings(t, svc, 1, "n1-in", actSettings, xray.ClientTraffic{Email: email, Up: 100, Down: 100, ExpiryTime: activated, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("after node 1 activation: expiry = %d, want %d", got, activated)
	}

	// Node 2 still reports the negative duration in BOTH ClientStats and
	// settings. Neither the merge nor SyncInbound may reset the deadline.
	syncNodeWithSettings(t, svc, 2, "n2-in", negSettings, xray.ClientTraffic{Email: email, ExpiryTime: duration, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("node 2 settings-sync clobbered the deadline: expiry = %d, want %d", got, activated)
	}
}

// TestNodeRenewExtendsExpiry guards against over-correcting: a node that renews
// a client (traffic reset / auto-renew) legitimately moves the deadline FORWARD
// to a later absolute timestamp, and that must still propagate to the master.
func TestNodeRenewExtendsExpiry(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renewing"
	const first = int64(1893456000000)
	const renewed = first + int64(2592000000) // +30 days after auto-renew

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 10, Down: 10, ExpiryTime: first, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != first {
		t.Fatalf("after activation: expiry = %d, want %d", got, first)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 20, Down: 20, ExpiryTime: renewed, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != renewed {
		t.Fatalf("node renewal did not propagate: expiry = %d, want %d", got, renewed)
	}
}

// TestNodeStaleExpiryAfterExtend_NotClobbered reproduces #6228: a client expires,
// the admin extends it on the master, then a node that still holds the previous
// deadline syncs enable=false + the old expiry. That snapshot must not undo the
// extension on traffics, client records, or the adopted inbound settings JSON.
func TestNodeStaleExpiryAfterExtend_NotClobbered(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "extended")
	svc := &InboundService{}

	const email = "extended"
	const expired = int64(1786835245763)
	const extended = int64(1789427245763)

	staleSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":false,"expiryTime":%d}]}`, email, expired)

	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	if got := readTraffic(t, db, email); got.ExpiryTime != expired || got.Enable {
		t.Fatalf("after expiry: expiry=%d enable=%v, want expiry=%d enable=false",
			got.ExpiryTime, got.Enable, expired)
	}

	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend traffic: %v", err)
	}
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend record: %v", err)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})

	got := readTraffic(t, db, email)
	if got.ExpiryTime != extended {
		t.Fatalf("stale node expiry clobbered traffics: expiry=%d, want %d", got.ExpiryTime, extended)
	}
	if !got.Enable {
		t.Fatal("stale node disable latched traffics.enable off after extension")
	}

	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("read client record: %v", err)
	}
	if rec.ExpiryTime != extended {
		t.Fatalf("stale SyncInbound clobbered record expiry: %d, want %d", rec.ExpiryTime, extended)
	}
	if !rec.Enable {
		t.Fatal("stale SyncInbound latched clients.enable off after extension")
	}

	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("read inbound: %v", err)
	}
	clients, err := svc.GetClients(&ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	var found bool
	for _, c := range clients {
		if c.Email != email {
			continue
		}
		found = true
		if c.ExpiryTime != extended {
			t.Fatalf("adopted settings kept stale expiry: %d, want %d", c.ExpiryTime, extended)
		}
		if !c.Enable {
			t.Fatal("adopted settings kept enable=false after extension")
		}
	}
	if !found {
		t.Fatal("client missing from adopted inbound settings after stale sync")
	}
}

// TestNodeQuotaDisable_SameExpiryStillLatches ensures #4917 stays intact: a
// node disable for traffic quota (same absolute expiry as the master) must
// still one-way-merge enable=false onto the master.
func TestNodeQuotaDisable_SameExpiryStillLatches(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "quota"
	const expiry = int64(1893456000000)

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 10, Down: 10, Total: 100, ExpiryTime: expiry, Enable: true,
	})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 60, Down: 50, Total: 100, ExpiryTime: expiry, Enable: false,
	})
	if got := readTraffic(t, db, email); got.Enable {
		t.Fatal("same-expiry node disable must still latch master enable off (#4917)")
	}
}

// TestNodeQuotaDisable_OlderExpiryStillLatchesWhenOverQuota: once the master
// row is already depleted, a node disable must latch even if its expiry lags
// the merged max — otherwise staleNodeDisable would skip genuine quota cuts.
func TestNodeQuotaDisable_OlderExpiryStillLatchesWhenOverQuota(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "quota-lag"
	const early = int64(1786835245763)
	const late = int64(1789427245763)

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 10, Down: 10, Total: 100, ExpiryTime: late, Enable: true,
	})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{
			"expiry_time": late, "enable": true, "up": int64(60), "down": int64(50), "total": int64(100),
		}).Error; err != nil {
		t.Fatalf("seed over-quota master: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 60, Down: 50, Total: 100, ExpiryTime: early, Enable: false,
	})
	if got := readTraffic(t, db, email); got.Enable {
		t.Fatal("over-quota master must still adopt node disable despite older node expiry")
	}
	if got := readTraffic(t, db, email); got.ExpiryTime != late {
		t.Fatalf("expiry should stay at master extension: got %d want %d", got.ExpiryTime, late)
	}
}

// TestClientTrafficMergeSQLMatchesHelpers pins the dialect SQL expressions
// against the Go helpers so the in-memory replay after UPDATE cannot drift.
func TestClientTrafficMergeSQLMatchesHelpers(t *testing.T) {
	db := initTrafficTestDB(t)

	const email = "sql-merge"
	cases := []struct {
		name                      string
		masterExpiry              int64
		masterEnable              bool
		masterUp, masterDown, tot int64
		nodeExpiry                int64
		nodeEnable                bool
		wantExpiry                int64
		wantEnable                bool
	}{
		{
			name:         "stale expiry+disable after extend",
			masterExpiry: lateAbs, masterEnable: true,
			nodeExpiry: earlyAbs, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: true,
		},
		{
			name:         "same expiry quota disable",
			masterExpiry: lateAbs, masterEnable: true,
			masterUp: 60, masterDown: 50, tot: 100,
			nodeExpiry: lateAbs, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: false,
		},
		{
			name:         "older expiry but master over quota",
			masterExpiry: lateAbs, masterEnable: true,
			masterUp: 60, masterDown: 50, tot: 100,
			nodeExpiry: earlyAbs, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: false,
		},
		{
			name:         "renewal extends forward",
			masterExpiry: earlyAbs, masterEnable: true,
			nodeExpiry: lateAbs, nodeEnable: true,
			wantExpiry: lateAbs, wantEnable: true,
		},
		{
			name:         "negative node keeps absolute",
			masterExpiry: lateAbs, masterEnable: true,
			nodeExpiry: -2592000000, nodeEnable: true,
			wantExpiry: lateAbs, wantEnable: true,
		},
	}

	enableExpr := database.ClientTrafficEnableMergeExpr()
	expiryExpr := database.ClientTrafficExpiryMergeExpr()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rowEmail := fmt.Sprintf("%s-%d", email, i)
			if err := db.Create(&xray.ClientTraffic{
				InboundId: 1, Email: rowEmail, Enable: c.masterEnable,
				ExpiryTime: c.masterExpiry, Up: c.masterUp, Down: c.masterDown, Total: c.tot,
			}).Error; err != nil {
				t.Fatalf("seed: %v", err)
			}
			master := &xray.ClientTraffic{
				ExpiryTime: c.masterExpiry, Enable: c.masterEnable,
				Up: c.masterUp, Down: c.masterDown, Total: c.tot,
			}
			wantExpiry := mergeActivationExpiry(c.masterExpiry, c.nodeExpiry)
			wantEnable := c.masterEnable
			if !c.nodeEnable && !staleNodeDisable(master, c.nodeExpiry) {
				wantEnable = false
			}
			if wantExpiry != c.wantExpiry || wantEnable != c.wantEnable {
				t.Fatalf("helper expectation drift: helpers=(%d,%v) fixture=(%d,%v)",
					wantExpiry, wantEnable, c.wantExpiry, c.wantEnable)
			}

			if err := db.Exec(
				fmt.Sprintf(
					`UPDATE client_traffics SET enable = %s, expiry_time = %s WHERE email = ?`,
					enableExpr, expiryExpr,
				),
				c.nodeEnable, c.nodeExpiry, c.nodeExpiry,
				c.nodeExpiry, c.nodeExpiry, c.nodeExpiry,
				rowEmail,
			).Error; err != nil {
				t.Fatalf("SQL merge: %v", err)
			}
			got := readTraffic(t, db, rowEmail)
			if got.ExpiryTime != c.wantExpiry || got.Enable != c.wantEnable {
				t.Fatalf("SQL merge got expiry=%d enable=%v, want expiry=%d enable=%v",
					got.ExpiryTime, got.Enable, c.wantExpiry, c.wantEnable)
			}
		})
	}
}

const (
	earlyAbs = int64(1786835245763)
	lateAbs  = int64(1789427245763)
)

// TestNodeActivationLiftsClientRecordExpiry reproduces #5714: the node activates
// the deadline (positive ClientStats) while its settings JSON still carries the
// negative duration, so SyncInbound keeps writing the stale value into the
// client record and the Clients page shows "not started" forever.
func TestNodeActivationLiftsClientRecordExpiry(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "delayed")
	svc := &InboundService{}

	const email = "delayed"
	const duration = int64(-2592000000)
	const activated = int64(1798448344010)
	negSettings := `{"clients":[{"email":"delayed","enable":true,"expiryTime":-2592000000}]}`

	if err := db.Create(&model.ClientRecord{Email: email, Enable: true, ExpiryTime: duration}).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}

	readRecordExpiry := func() int64 {
		t.Helper()
		var rec model.ClientRecord
		if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
			t.Fatalf("read client record: %v", err)
		}
		return rec.ExpiryTime
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", negSettings, xray.ClientTraffic{Email: email, ExpiryTime: duration, Enable: true})
	if got := readRecordExpiry(); got != duration {
		t.Fatalf("before activation: record expiry = %d, want %d", got, duration)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", negSettings, xray.ClientTraffic{Email: email, Up: 100, Down: 100, ExpiryTime: activated, Enable: true})
	if got := readTraffic(t, db, email).ExpiryTime; got != activated {
		t.Fatalf("client_traffics not activated: expiry = %d, want %d", got, activated)
	}
	if got := readRecordExpiry(); got != activated {
		t.Fatalf("client record kept stale duration (#5714): expiry = %d, want %d", got, activated)
	}
}
