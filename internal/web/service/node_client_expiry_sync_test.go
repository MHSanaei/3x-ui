package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
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
		{"master absolute ignores later node absolute", early, late, early},
		{"node value equal to master is a no-op", early, early, early},
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
	now := time.Now().UnixMilli()
	const quota = int64(100)
	cases := []struct {
		name      string
		master    *xray.ClientTraffic
		node      xray.ClientTraffic
		deltaUp   int64
		deltaDown int64
		wantStale bool
	}{
		{name: "nil master", node: xray.ClientTraffic{ExpiryTime: earlyAbs}},
		{
			name:      "matching limits are a genuine verdict",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota},
			node:      xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota},
			wantStale: false,
		},
		{
			name:      "node still holds the pre-extension deadline",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs},
			node:      xray.ClientTraffic{ExpiryTime: earlyAbs},
			wantStale: true,
		},
		{
			name:      "node still holds the pre-top-up quota",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs, Total: 2 * quota, Up: 60, Down: 50},
			node:      xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota},
			wantStale: true,
		},
		{
			name:   "master itself expired",
			master: &xray.ClientTraffic{ExpiryTime: earlyAbs},
			node:   xray.ClientTraffic{ExpiryTime: earlyAbs - 1000},
		},
		{
			name:   "master itself over quota",
			master: &xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota, Up: 60, Down: 50},
			node:   xray.ClientTraffic{ExpiryTime: earlyAbs, Total: quota},
		},
		{
			name:      "this tick's deltas cross the master quota",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota, Up: 40, Down: 50},
			node:      xray.ClientTraffic{ExpiryTime: earlyAbs, Total: quota},
			deltaUp:   10,
			deltaDown: 10,
		},
		{
			// Same row without the deltas: the crossing above is the deltas' doing.
			name:      "under the master quota before this tick's deltas",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs, Total: quota, Up: 40, Down: 50},
			node:      xray.ClientTraffic{ExpiryTime: earlyAbs, Total: quota},
			wantStale: true,
		},
		{
			name:      "un-activated node duration against an absolute master",
			master:    &xray.ClientTraffic{ExpiryTime: lateAbs},
			node:      xray.ClientTraffic{ExpiryTime: -2592000000},
			wantStale: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nodeDisableIsStale(c.master, c.node, now, c.deltaUp, c.deltaDown); got != c.wantStale {
				t.Fatalf("nodeDisableIsStale(...) = %v, want %v", got, c.wantStale)
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

// TestNodeRenewExtendsExpiry: node auto-renew (reset + later expiry + counter
// drop) must still move master expiry forward via nodeClientRenewed.
func TestNodeRenewExtendsExpiry(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renewing"
	const first = int64(1893456000000)
	const renewed = first + int64(2592000000) // +30 days after auto-renew

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 0, Down: 0, ExpiryTime: first, Reset: 30, Enable: true,
	})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 100, Down: 100, ExpiryTime: first, Reset: 30, Enable: true,
	})
	if got := readTraffic(t, db, email).ExpiryTime; got != first {
		t.Fatalf("after activation: expiry = %d, want %d", got, first)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 5, Down: 5, ExpiryTime: renewed, Reset: 30, Enable: true,
	})
	if got := readTraffic(t, db, email).ExpiryTime; got != renewed {
		t.Fatalf("node renewal did not propagate: expiry = %d, want %d", got, renewed)
	}
}

// TestNodeRenew_WithMatchingSettings: renew still applies when settings JSON
// also carries the later absolute (guard must not block real renewals).
func TestNodeRenew_WithMatchingSettings(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renew-settings"
	firstSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, renewFirstExpiry)
	renewSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, renewSecondExpiry)

	syncNodeWithSettings(t, svc, 1, "n1-in", firstSettings, xray.ClientTraffic{
		Email: email, Up: 0, Down: 0, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true,
	})
	syncNodeWithSettings(t, svc, 1, "n1-in", firstSettings, xray.ClientTraffic{
		Email: email, Up: 100, Down: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Enable: true,
	})
	syncNodeWithSettings(t, svc, 1, "n1-in", renewSettings, xray.ClientTraffic{
		Email: email, Up: 5, Down: 5, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays, Enable: true,
	})
	if got := readTraffic(t, db, email).ExpiryTime; got != renewSecondExpiry {
		t.Fatalf("renewal with matching settings: got %d want %d", got, renewSecondExpiry)
	}
}

// A node still holding the pre-extension deadline must not undo the extension on
// traffics, client records or the adopted settings JSON (#6228).
func TestNodeStaleExpiryAfterExtend_NotClobbered(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "extended")
	svc := &InboundService{}

	const email = "extended"
	expired, extended := earlyAbs, lateAbs

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

// TestNodeStaleLift_MarksNodeDirty: a lifecycle lift must mark the node dirty
// and store lifted settings centrally so reconcile can re-push (#6228).
func TestNodeStaleLift_MarksNodeDirty(t *testing.T) {
	db := initTrafficTestDB(t)
	node := &model.Node{Name: "lift-n", Address: "127.0.0.1", Port: 2097, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	createNodeInboundWithClient(t, db, node.Id, "n1-in", 41001, "extended")
	svc := &InboundService{}

	const email = "extended"
	expired, extended := earlyAbs, lateAbs
	staleSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":false,"expiryTime":%d}]}`, email, expired)

	syncNodeWithSettings(t, svc, node.Id, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend: %v", err)
	}
	if err := db.Model(model.Node{}).Where("id = ?", node.Id).
		Updates(map[string]any{"config_dirty": false, "config_dirty_at": int64(0)}).Error; err != nil {
		t.Fatalf("clear dirty: %v", err)
	}

	syncNodeWithSettings(t, svc, node.Id, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})

	var n model.Node
	if err := db.Select("config_dirty").Where("id = ?", node.Id).First(&n).Error; err != nil {
		t.Fatalf("read node: %v", err)
	}
	if !n.ConfigDirty {
		t.Fatal("lifecycle lift must mark the node dirty so reconcile re-pushes")
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
		if c.ExpiryTime != extended || !c.Enable {
			t.Fatalf("lifted settings not stored: expiry=%d enable=%v", c.ExpiryTime, c.Enable)
		}
	}
	if !found {
		t.Fatal("client missing from lifted inbound settings")
	}
	if ib.Settings == staleSettings {
		t.Fatal("central settings must differ from pre-lift wire blob (FP basis)")
	}
}

// A node disable decided on the master's own limits is genuine and must still
// one-way-merge enable=false onto the master (#4917).
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

// Once the master row is itself depleted, a node disable latches even though the
// node's limits lag — otherwise genuine quota cuts would be skipped.
func TestNodeQuotaDisable_OlderExpiryStillLatchesWhenOverQuota(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "quota-lag"
	early, late := earlyAbs, lateAbs

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

// TestNodeMasterShorten_NotClobberedWhileDirty: master shortened expiry while
// config_dirty; a lagging longer node snapshot must not raise client_traffics.
func TestNodeMasterShorten_NotClobberedWhileDirty(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "shortened")
	svc := &InboundService{}

	const email = "shortened"
	longExp, shortExp := lateAbs, earlyAbs
	longSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, longExp)

	syncNodeWithSettings(t, svc, 1, "n1-in", longSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: longExp, Enable: true})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": shortExp, "enable": true}).Error; err != nil {
		t.Fatalf("master shorten: %v", err)
	}
	if err := db.Model(model.Node{}).Where("id = ?", 1).
		Updates(map[string]any{"config_dirty": true, "config_dirty_at": int64(1)}).Error; err != nil {
		t.Fatalf("mark dirty: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in", Settings: longSettings,
			ClientStats: []xray.ClientTraffic{{Email: email, ExpiryTime: longExp, Enable: true, Up: 5, Down: 5}},
		}},
	}
	before := readTraffic(t, db, email)
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Update("total", int64(999)).Error; err != nil {
		t.Fatalf("master total: %v", err)
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, true, false); err != nil {
		t.Fatalf("dirty sync: %v", err)
	}
	got := readTraffic(t, db, email)
	if got.ExpiryTime != shortExp {
		t.Fatalf("dirty sync raised expiry: got %d want %d", got.ExpiryTime, shortExp)
	}
	if got.Total != 999 {
		t.Fatalf("dirty sync adopted node total: got %d want 999", got.Total)
	}
	if got.Up < before.Up+5 || got.Down < before.Down+5 {
		t.Fatalf("dirty sync must still accumulate traffic: before=(%d,%d) after=(%d,%d)",
			before.Up, before.Down, got.Up, got.Down)
	}
}

// The master's shortened absolute survives a snapshot whose ClientStats still
// report the longer deadline (#6228).
func TestNodeMasterShorten_LaggingClientStatsIgnored(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "short-settings")
	svc := &InboundService{}

	const email = "short-settings"
	longExp, shortExp := lateAbs, earlyAbs
	longSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, longExp)
	shortSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, shortExp)

	syncNodeWithSettings(t, svc, 1, "n1-in", longSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: longExp, Enable: true})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": shortExp, "enable": true}).Error; err != nil {
		t.Fatalf("master shorten: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in", Settings: shortSettings,
			ClientStats: []xray.ClientTraffic{{Email: email, ExpiryTime: longExp, Enable: true}},
		}},
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("clean sync: %v", err)
	}
	if got := readTraffic(t, db, email); got.ExpiryTime != shortExp {
		t.Fatalf("lagging ClientStats raised expiry: got %d want %d", got.ExpiryTime, shortExp)
	}
}

// TestNodeMasterShorten_CleanSiblingCannotRaise: a clean sibling still holding
// the longer deadline in settings+ClientStats must not undo a master shorten.
func TestNodeMasterShorten_CleanSiblingCannotRaise(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "sib-short")
	createNodeInboundWithClient(t, db, 2, "n2-in", 41002, "sib-short")
	svc := &InboundService{}

	const email = "sib-short"
	longExp, shortExp := lateAbs, earlyAbs
	longSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, longExp)

	syncNodeWithSettings(t, svc, 1, "n1-in", longSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: longExp, Enable: true})
	syncNodeWithSettings(t, svc, 2, "n2-in", longSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: longExp, Enable: true})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": shortExp, "enable": true}).Error; err != nil {
		t.Fatalf("master shorten: %v", err)
	}

	// Sibling 2 is clean and still reports the old longer deadline.
	syncNodeWithSettings(t, svc, 2, "n2-in", longSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: longExp, Enable: true})
	if got := readTraffic(t, db, email); got.ExpiryTime != shortExp {
		t.Fatalf("clean sibling raised shortened expiry: got %d want %d", got.ExpiryTime, shortExp)
	}
}

// TestNodeMasterShorten_LaggingStatsNotTreatedAsRenew: after shorten, ClientStats
// may still show the longer deadline with Reset+dip — must not call renewal.
func TestNodeMasterShorten_LaggingStatsNotTreatedAsRenew(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "short-renew-trap"
	longExp, shortExp := lateAbs, earlyAbs
	shortSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, shortExp)

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 0, Down: 0, ExpiryTime: longExp, Reset: 30, Enable: true,
	})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 100, Down: 100, ExpiryTime: longExp, Reset: 30, Enable: true,
	})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": shortExp, "enable": true}).Error; err != nil {
		t.Fatalf("master shorten: %v", err)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", shortSettings,
		xray.ClientTraffic{Email: email, Up: 5, Down: 5, ExpiryTime: longExp, Reset: 30, Enable: true})
	if got := readTraffic(t, db, email); got.ExpiryTime != shortExp {
		t.Fatalf("lagging stats treated as renew: got %d want %d", got.ExpiryTime, shortExp)
	}
}

// TestNodeStaleExpiryAfterExtend_WhileDirty keeps master extend while dirty.
func TestNodeStaleExpiryAfterExtend_WhileDirty(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "extended-dirty")
	svc := &InboundService{}

	const email = "extended-dirty"
	expired, extended := earlyAbs, lateAbs
	staleSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":false,"expiryTime":%d}]}`, email, expired)

	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend: %v", err)
	}
	if err := db.Model(model.Node{}).Where("id = ?", 1).
		Updates(map[string]any{"config_dirty": true, "config_dirty_at": int64(1)}).Error; err != nil {
		t.Fatalf("mark dirty: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in", Settings: staleSettings,
			ClientStats: []xray.ClientTraffic{{Email: email, ExpiryTime: expired, Enable: false}},
		}},
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, true, false); err != nil {
		t.Fatalf("dirty sync: %v", err)
	}
	got := readTraffic(t, db, email)
	if got.ExpiryTime != extended || !got.Enable {
		t.Fatalf("dirty sync clobbered extend: expiry=%d enable=%v", got.ExpiryTime, got.Enable)
	}
}

// TestNodeRenewal_SkippedWhileDirty: renewal-shaped stats must not adopt
// expiry/enable over a pending master push.
func TestNodeRenewal_SkippedWhileDirty(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "renew-dirty"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 100, Down: 100, ExpiryTime: earlyAbs, Enable: true, Reset: 30,
	})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": earlyAbs, "enable": true, "up": int64(100), "down": int64(100)}).Error; err != nil {
		t.Fatalf("seed master: %v", err)
	}
	if err := db.Model(model.Node{}).Where("id = ?", 1).
		Updates(map[string]any{"config_dirty": true, "config_dirty_at": int64(1)}).Error; err != nil {
		t.Fatalf("mark dirty: %v", err)
	}
	// Renewal shape: later expiry + counters below baseline.
	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in",
			ClientStats: []xray.ClientTraffic{{
				Email: email, Up: 10, Down: 10, ExpiryTime: lateAbs, Enable: true, Reset: 30,
			}},
		}},
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, true, false); err != nil {
		t.Fatalf("dirty sync: %v", err)
	}
	got := readTraffic(t, db, email)
	if got.ExpiryTime != earlyAbs {
		t.Fatalf("renewal while dirty raised expiry: got %d want %d", got.ExpiryTime, earlyAbs)
	}

	// Same renewal-shaped stats on a clean tick must still advance expiry —
	// dirty must not have burned the dipped baseline.
	if err := db.Model(model.Node{}).Where("id = ?", 1).
		Updates(map[string]any{"config_dirty": false, "config_dirty_at": int64(0)}).Error; err != nil {
		t.Fatalf("clear dirty: %v", err)
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("clean sync: %v", err)
	}
	if got := readTraffic(t, db, email); got.ExpiryTime != lateAbs {
		t.Fatalf("renewal after dirty clear did not apply: got %d want %d", got.ExpiryTime, lateAbs)
	}
}

// TestNodeExtend_FreshSettingsLaggingDisable: after extend, settings may already
// show the new absolute while ClientStats still report enable=false + old expiry.
func TestNodeExtend_FreshSettingsLaggingDisable(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "ext-lag")
	svc := &InboundService{}

	const email = "ext-lag"
	expired, extended := earlyAbs, lateAbs
	staleSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":false,"expiryTime":%d}]}`, email, expired)
	freshSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, extended)

	syncNodeWithSettings(t, svc, 1, "n1-in", staleSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: expired, Enable: false})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"expiry_time": extended, "enable": true}).Error; err != nil {
		t.Fatalf("master extend: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in", Settings: freshSettings,
			ClientStats: []xray.ClientTraffic{{Email: email, ExpiryTime: expired, Enable: false}},
		}},
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("clean sync: %v", err)
	}
	got := readTraffic(t, db, email)
	if got.ExpiryTime != extended {
		t.Fatalf("expiry clobbered: got %d want %d", got.ExpiryTime, extended)
	}
	if !got.Enable {
		t.Fatal("lagging ClientStats enable=false latched master off after extend")
	}
}

// A client that used no traffic never dips below its baseline, so the renewal
// counter is the only evidence the node auto-renewed (#6228).
func TestNodeRenew_ZeroTrafficUsesResetCount(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "idle-renew"
	firstSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, earlyAbs)
	renewSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, lateAbs)

	syncNodeWithSettings(t, svc, 1, "n1-in", firstSettings, xray.ClientTraffic{
		Email: email, ExpiryTime: earlyAbs, Reset: 30, Enable: true,
	})
	syncNodeWithSettings(t, svc, 1, "n1-in", renewSettings, xray.ClientTraffic{
		Email: email, ExpiryTime: lateAbs, Reset: 30, ResetCount: 1, Enable: true,
	})

	got := readTraffic(t, db, email)
	if got.ExpiryTime != lateAbs {
		t.Fatalf("zero-traffic renewal dropped: expiry=%d want %d", got.ExpiryTime, lateAbs)
	}
	if got.ResetCount != 1 {
		t.Fatalf("renewal count not persisted, so the next renewal cannot be seen: got %d want 1", got.ResetCount)
	}
}

// A quota top-up leaves the expiry alone, so the node's lagging disable must be
// recognised by the stale quota it was decided against (#6228).
func TestNodeQuotaTopUp_LaggingDisableIgnored(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "topped-up"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 10, Down: 10, Total: 100, ExpiryTime: lateAbs, Enable: true,
	})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{
			"total": int64(500), "enable": true, "up": int64(60), "down": int64(50),
		}).Error; err != nil {
		t.Fatalf("master top-up: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Up: 60, Down: 50, Total: 100, ExpiryTime: lateAbs, Enable: false,
	})
	if got := readTraffic(t, db, email); !got.Enable {
		t.Fatal("node disable decided on the pre-top-up quota latched over the raised one")
	}
}

// The settings lift is authoritative in both directions: a blob predating a
// master disable must not carry enable=true back into central settings (#4917).
func TestNodeStaleEnable_LiftedOffInSettings(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "cut-off")
	svc := &InboundService{}

	const email = "cut-off"
	liveSettings := fmt.Sprintf(
		`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, lateAbs)

	syncNodeWithSettings(t, svc, 1, "n1-in", liveSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: lateAbs, Enable: true})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Update("enable", false).Error; err != nil {
		t.Fatalf("master disable: %v", err)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", liveSettings,
		xray.ClientTraffic{Email: email, ExpiryTime: lateAbs, Enable: true})

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
		if c.Enable {
			t.Fatal("adopted settings re-enabled a client the master disabled")
		}
	}
	if !found {
		t.Fatal("client missing from adopted inbound settings")
	}
}

// The tick whose push just landed freezes only the lifecycle merge: adoption,
// new client rows and traffic accumulation must keep working (#6228).
func TestNodeJustPushed_FreezesLifecycleOnly(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const kept = "kept"
	const fresh = "fresh"
	extended := lateAbs + 86400000

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: kept, Up: 10, Down: 10, Total: 100, ExpiryTime: lateAbs, Enable: true,
	})
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", kept).
		Updates(map[string]any{"expiry_time": extended, "total": int64(500)}).Error; err != nil {
		t.Fatalf("master edit: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag: "n1-in",
			ClientStats: []xray.ClientTraffic{
				{Email: kept, Up: 20, Down: 20, Total: 100, ExpiryTime: lateAbs, Enable: false},
				{Email: fresh, Up: 5, Down: 5, ExpiryTime: lateAbs, Enable: true},
			},
		}},
	}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, true); err != nil {
		t.Fatalf("just-pushed sync: %v", err)
	}

	got := readTraffic(t, db, kept)
	if got.ExpiryTime != extended || got.Total != 500 || !got.Enable {
		t.Fatalf("just-pushed tick adopted lagging lifecycle: expiry=%d total=%d enable=%v",
			got.ExpiryTime, got.Total, got.Enable)
	}
	if got.Up != 10 || got.Down != 10 {
		t.Fatalf("just-pushed tick dropped this tick's traffic: up=%d down=%d, want 10/10", got.Up, got.Down)
	}
	if row := readTraffic(t, db, fresh); row.Email != fresh {
		t.Fatalf("just-pushed tick skipped adoption of a new client: %+v", row)
	}
}

// TestClientTrafficMergeSQLMatchesHelpers pins the dialect SQL expressions
// against the Go helpers so the in-memory replay after UPDATE cannot drift.
func TestClientTrafficMergeSQLMatchesHelpers(t *testing.T) {
	db := initTrafficTestDB(t)

	const email = "sql-merge"
	now := time.Now().UnixMilli()
	cases := []struct {
		name                      string
		masterExpiry              int64
		masterEnable              bool
		masterUp, masterDown, tot int64
		deltaUp, deltaDown        int64
		nodeExpiry, nodeTotal     int64
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
			nodeExpiry: lateAbs, nodeTotal: 100, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: false,
		},
		{
			name:         "older expiry but master over quota",
			masterExpiry: lateAbs, masterEnable: true,
			masterUp: 60, masterDown: 50, tot: 100,
			nodeExpiry: earlyAbs, nodeTotal: 100, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: false,
		},
		{
			name:         "master absolute ignores later node",
			masterExpiry: earlyAbs, masterEnable: true,
			nodeExpiry: lateAbs, nodeEnable: true,
			wantExpiry: earlyAbs, wantEnable: true,
		},
		{
			name:         "negative node keeps absolute",
			masterExpiry: lateAbs, masterEnable: true,
			nodeExpiry: -2592000000, nodeEnable: true,
			wantExpiry: lateAbs, wantEnable: true,
		},
		{
			name:         "expired master latches node disable",
			masterExpiry: earlyAbs, masterEnable: true,
			nodeExpiry: earlyAbs - 1000, nodeEnable: false,
			wantExpiry: earlyAbs, wantEnable: false,
		},
		{
			name:         "older expiry crosses quota via deltas",
			masterExpiry: lateAbs, masterEnable: true,
			masterUp: 40, masterDown: 50, tot: 100,
			deltaUp: 10, deltaDown: 10,
			nodeExpiry: earlyAbs, nodeTotal: 100, nodeEnable: false,
			wantExpiry: lateAbs, wantEnable: false,
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
			node := xray.ClientTraffic{ExpiryTime: c.nodeExpiry, Total: c.nodeTotal}
			if !c.nodeEnable && !nodeDisableIsStale(master, node, now, c.deltaUp, c.deltaDown) {
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
				c.nodeEnable, c.nodeExpiry, c.nodeTotal, now, c.deltaUp, c.deltaDown,
				c.nodeExpiry,
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

// Relative to the run: the merge rules now compare the master deadline against
// wall-clock now, so fixed timestamps would rot into the wrong side of it.
var (
	earlyAbs = time.Now().UnixMilli() - 30*86400000
	lateAbs  = time.Now().UnixMilli() + 30*86400000
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
