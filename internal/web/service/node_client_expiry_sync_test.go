package service

import (
	"testing"

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
		{"node positive adopted even if earlier", late, early, early},
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
// The guard only rejects un-activated (<= 0) values, never a positive one.
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
