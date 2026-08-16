package service

import (
	"testing"

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

func TestNodeDisableIsStale(t *testing.T) {
	const (
		early = int64(1000)
		late  = int64(2000)
	)
	cases := []struct {
		name                  string
		masterExpiry, nodeExp int64
		nodeEnable, wantStale bool
	}{
		{"node still enabled", late, early, true, false},
		{"same absolute deadline", late, late, false, false},
		{"node older than master after extend", late, early, false, true},
		{"node newer renewal disable", early, late, false, false},
		{"unactivated node", late, -1, false, false},
		{"unlimited master", 0, early, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nodeDisableIsStale(c.masterExpiry, c.nodeExp, c.nodeEnable); got != c.wantStale {
				t.Fatalf("nodeDisableIsStale(%d,%d,%v) = %v, want %v",
					c.masterExpiry, c.nodeExp, c.nodeEnable, got, c.wantStale)
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
// The guard rejects un-activated (<= 0) values and earlier absolute deadlines,
// never a later positive one.
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
// the admin extends it on the master (shared client_traffics gets a later
// deadline and is re-enabled), then a node that still holds the previous
// deadline syncs enable=false + the old expiry. That snapshot must not undo the
// extension or latch the master disable — otherwise the client is removed again
// with "due to expiration or traffic limit".
func TestNodeStaleExpiryAfterExtend_NotClobbered(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "extended")
	svc := &InboundService{}

	const email = "extended"
	const expired = int64(1786835245763)  // past
	const extended = int64(1789427245763) // later extension on the master

	staleSettings := `{"clients":[{"email":"extended","enable":false,"expiryTime":1786835245763}]}`
	liveSettings := `{"clients":[{"email":"extended","enable":true,"expiryTime":1789427245763}]}`

	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	if got := readTraffic(t, db, email); got.ExpiryTime != expired || got.Enable {
		t.Fatalf("after expiry: expiry=%d enable=%v, want expiry=%d enable=false",
			got.ExpiryTime, got.Enable, expired)
	}

	// Master-side extension (UpdateClientStat / BulkAdjust) lands on the shared row.
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend: %v", err)
	}

	// Stale node snapshot still reporting the previous deadline + disable.
	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	got := readTraffic(t, db, email)
	if got.ExpiryTime != extended {
		t.Fatalf("stale node expiry clobbered the extension: expiry=%d, want %d", got.ExpiryTime, extended)
	}
	if !got.Enable {
		t.Fatal("stale node disable latched the master enable off after extension")
	}

	// A live node that has received the extension may still report it.
	syncNodeWithSettings(t, svc, 1, "n1-in", liveSettings,
		xray.ClientTraffic{Email: email, Up: 1, Down: 1, ExpiryTime: extended, Enable: true})
	got = readTraffic(t, db, email)
	if got.ExpiryTime != extended || !got.Enable {
		t.Fatalf("after live sync: expiry=%d enable=%v, want expiry=%d enable=true",
			got.ExpiryTime, got.Enable, extended)
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
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"enable": true, "up": 10, "down": 10, "total": 100}).Error; err != nil {
		t.Fatalf("seed master enable: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 60, Down: 50, Total: 100, ExpiryTime: expiry, Enable: false,
	})
	if got := readTraffic(t, db, email); got.Enable {
		t.Fatal("same-expiry node disable must still latch master enable off (#4917)")
	}
}

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
