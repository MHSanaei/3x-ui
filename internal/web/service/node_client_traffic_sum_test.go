package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

func initTrafficTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return database.GetDB()
}

func createNodeInbound(t *testing.T, db *gorm.DB, nodeID int, tag string, port int) {
	t.Helper()
	nid := nodeID
	ib := &model.Inbound{UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS, NodeID: &nid}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create node inbound %q: %v", tag, err)
	}
}

// createNodeInboundWithClient mirrors createNodeInbound but stores the client
// in the settings JSON so emailUsedByOtherInbounds can see the attachment.
func createNodeInboundWithClient(t *testing.T, db *gorm.DB, nodeID int, tag string, port int, email string) {
	t.Helper()
	nid := nodeID
	settings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)
	ib := &model.Inbound{UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS, NodeID: &nid, Settings: settings}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create node inbound %q: %v", tag, err)
	}
}

func syncNode(t *testing.T, svc *InboundService, nodeID int, tag string, stats ...xray.ClientTraffic) {
	t.Helper()
	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{Tag: tag, ClientStats: stats}},
	}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked node %d: %v", nodeID, err)
	}
}

// syncNodeWithSettings mirrors syncNode but carries a real settings JSON on
// the snapshot inbound, like production nodes do — the sync mirrors snapshot
// settings onto the central row, and the shared-accumulator guard reads the
// clients out of those settings.
func syncNodeWithSettings(t *testing.T, svc *InboundService, nodeID int, tag, settings string, stats ...xray.ClientTraffic) {
	t.Helper()
	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{Tag: tag, Settings: settings, ClientStats: stats}},
	}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked node %d: %v", nodeID, err)
	}
}

func readTraffic(t *testing.T, db *gorm.DB, email string) xray.ClientTraffic {
	t.Helper()
	var ct xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&ct).Error; err != nil {
		t.Fatalf("read client_traffics %q: %v", email, err)
	}
	return ct
}

func assertUpDown(t *testing.T, ct xray.ClientTraffic, wantUp, wantDown int64, when string) {
	t.Helper()
	if ct.Up != wantUp || ct.Down != wantDown {
		t.Errorf("%s: up=%d down=%d, want %d/%d", when, ct.Up, ct.Down, wantUp, wantDown)
	}
}

func TestTwoNodesShareEmail_SumsCorrectly(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeInbound(t, db, 2, "n2-in", 41002)
	svc := &InboundService{}

	const email = "shared"

	// First-ever sync from each node: the row is created at 0 and the current
	// node counters become the baseline. Only deltas beyond those baselines count.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})

	assertUpDown(t, readTraffic(t, db, email), 0, 0, "after baselines — historical node traffic not imported")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 150, Down: 150, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 260, Down: 260, Enable: true})

	assertUpDown(t, readTraffic(t, db, email), 110, 110, "after both nodes grow — deltas (50+60) accrue")
}

func TestSingleNode_MirrorsCorrectly(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "solo"
	// First sync: row seeded at 0, current counter becomes the baseline.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 500, Down: 600, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "first sync — historical traffic not imported")

	// Second sync: delta (700-500=200 / 800-600=200) accrues normally.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 700, Down: 800, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 200, 200, "second sync — delta accrues")
}

func TestUpgrade_PreExistingRow_NoDoubleCount(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "legacy"
	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: email, Up: 1000, Down: 2000, Enable: true}).Error; err != nil {
		t.Fatalf("seed pre-existing row: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 1000, Down: 2000, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1000, 2000, "first snapshot must not double-count")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 1100, Down: 2100, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1100, 2100, "growth after upgrade accrues")
}

// TestGhostData_NoPhantomTraffic reproduces the 50 GB phantom traffic bug:
// a node retains stale data for a deleted client (e.g. de-1 was offline during
// deletion). When the master first syncs this email again it must NOT import the
// stale counter — it sets Up=0 and records the current node value as the
// baseline so only future deltas count.
func TestGhostData_NoPhantomTraffic(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "de1-in", 41001)
	svc := &InboundService{}

	const email = "pouya"
	const staleBytes int64 = 50 * 1024 * 1024 * 1024 // 50 GB stale on de-1

	// Node reports a client with a large pre-existing counter (ghost data from a
	// deleted account that survived on the node). Master has no row for this email.
	syncNode(t, svc, 1, "de1-in", xray.ClientTraffic{Email: email, Up: staleBytes, Down: staleBytes, Enable: true})

	ct := readTraffic(t, db, email)
	if ct.Up >= staleBytes || ct.Down >= staleBytes {
		t.Errorf("ghost data imported: up=%d down=%d; stale node counter must not count as new traffic", ct.Up, ct.Down)
	}
	assertUpDown(t, ct, 0, 0, "first sync of ghost email — imported as 0, not 50 GB")

	// Subsequent syncs only add incremental usage beyond the baseline.
	syncNode(t, svc, 1, "de1-in", xray.ClientTraffic{Email: email, Up: staleBytes + 1024, Down: staleBytes + 2048, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 1024, 2048, "only incremental traffic beyond baseline counts")
}

func TestNodeCounterReset_NoReAdd(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "restart"
	// First sync seeds at 0 (baseline=900). Second sync adds delta 950-900=50.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 900, Down: 900, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 950, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 50, 50, "before node reset")

	// Node reboot drops the counter to 50. delta=50-950=-900 is a counter reset,
	// not new traffic: add 0 and rebaseline to 50, never re-add the node's full
	// cumulative counter onto the master total (#5456).
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 50, Down: 50, Enable: true})
	ct := readTraffic(t, db, email)
	if ct.Up < 0 || ct.Down < 0 {
		t.Fatalf("row went negative after node reset: up=%d down=%d", ct.Up, ct.Down)
	}
	assertUpDown(t, ct, 50, 50, "after node counter reset: rebaselined, not re-added")

	// Post-reset accrual resumes from the new baseline: 80-50=30.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 80, Down: 80, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 80, 80, "post-reset delta accrues from rebaselined counter")
}

func TestCentralReset_NoReAdd(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeInbound(t, db, 2, "n2-in", 41002)
	svc := &InboundService{}

	const email = "reset"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})

	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"up": 0, "down": 0}).Error; err != nil {
		t.Fatalf("simulate central reset: %v", err)
	}

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 210, Down: 210, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 205, Down: 205, Enable: true})

	assertUpDown(t, readTraffic(t, db, email), 15, 15, "after central reset only increments accrue")
}

// A real reset (ResetClientTrafficByEmail) must clear the per-node baseline so
// the node's pre-reset cumulative — including traffic it counted but had not yet
// synced — cannot leak back onto the master after the reset (#5476, #5390).
func TestCentralResetClearsNodeBaseline_NoLeak(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	StartTrafficWriter()
	svc := &InboundService{}

	const email = "reset-revert"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 300, Down: 300, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 200, 200, "before reset")

	if err := svc.ResetClientTrafficByEmail(email); err != nil {
		t.Fatalf("ResetClientTrafficByEmail: %v", err)
	}
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "right after reset")

	var baselines int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", email).Count(&baselines).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if baselines != 0 {
		t.Fatalf("reset must clear node baseline rows, found %d", baselines)
	}

	// Node still reports its pre-reset cumulative (340 > last synced 300: usage it
	// had not synced before the reset). It must not revert the reset.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 340, Down: 340, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "stale node counter must not revert reset")

	// Genuine post-reset usage accrues from the rebaselined counter: 370-340=30.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 370, Down: 370, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 30, 30, "post-reset usage accrues")
}

func TestInboundRemoval_KeepsSharedEmailRow(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "shared")
	createNodeInboundWithClient(t, db, 2, "n2-in", 41002, "shared")
	svc := &InboundService{}

	const email = "shared"
	settings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)
	// Baselines set: node1=100, node2=200. Row seeded at 0.
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNodeWithSettings(t, svc, 2, "n2-in", settings, xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})
	// node1 delta=50, node2 delta=60 → total=110.
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, xray.ClientTraffic{Email: email, Up: 150, Down: 150, Enable: true})
	syncNodeWithSettings(t, svc, 2, "n2-in", settings, xray.ClientTraffic{Email: email, Up: 260, Down: 260, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 110, 110, "baseline sum")

	// Node 1 rebuilt (reinstall / another master's reconcile): its inbound
	// vanishes from the snapshot. The shared accumulator must survive — losing
	// it would let the next node sync re-seed the row with that node's counter
	// alone, showing only the last panel's number instead of the sum.
	if _, err := svc.setRemoteTrafficLocked(1, &runtime.TrafficSnapshot{}, false); err != nil {
		t.Fatalf("sync node 1 with empty snapshot: %v", err)
	}
	assertUpDown(t, readTraffic(t, db, email), 110, 110, "after node 1 inbound removal")

	// Node 2 keeps accruing onto the surviving row. node2 delta=300-260=40 → 150.
	syncNodeWithSettings(t, svc, 2, "n2-in", settings, xray.ClientTraffic{Email: email, Up: 300, Down: 300, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 150, 150, "after node 2 grows")
}

func TestClientGoneFromOneNode_KeepsSharedEmailRow(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "shared")
	createNodeInboundWithClient(t, db, 2, "n2-in", 41002, "shared")
	svc := &InboundService{}

	const email = "shared"
	settings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)
	// First syncs set baselines (node1=100, node2=200). Row seeded at 0.
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNodeWithSettings(t, svc, 2, "n2-in", settings, xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})

	// Client detached from node 1's inbound only: its stats vanish from that
	// inbound's snapshot while node 2 still hosts the email.
	syncNodeWithSettings(t, svc, 1, "n1-in", `{"clients": []}`)
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "after client left node 1 — row unchanged at 0")

	// Node 2 delta: 240-200=40 → accrues to 40.
	syncNodeWithSettings(t, svc, 2, "n2-in", settings, xray.ClientTraffic{Email: email, Up: 240, Down: 240, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 40, 40, "node 2 keeps accruing")
}

// TestStatsUnderSiblingInbound_KeepsNodeBaseline reproduces the recurring
// sweep bug behind #5202: the client is attached to two inbounds of the SAME
// node, the node reports its stats under n1-a only, but the master-side row
// is owned by n1-b's mirror. The per-email sweep for n1-b must not drop the
// (node, email) baseline that n1-a's delta computation needs — doing so every
// cycle froze the client's counter permanently.
func TestStatsUnderSiblingInbound_KeepsNodeBaseline(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-a", 41001, "fresh")
	createNodeInbound(t, db, 1, "n1-b", 41002)
	svc := &InboundService{}

	const email = "fresh"
	var ibB model.Inbound
	if err := db.Where("tag = ?", "n1-b").First(&ibB).Error; err != nil {
		t.Fatalf("load n1-b: %v", err)
	}
	// Master-side row created when the client was added on the panel, owned by
	// n1-b's mirror (e.g. the client form targeted that inbound).
	if err := db.Create(&xray.ClientTraffic{InboundId: ibB.Id, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("seed master row: %v", err)
	}

	settings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)
	sync := func(up, down int64) {
		t.Helper()
		snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
			{Tag: "n1-a", Settings: settings, ClientStats: []xray.ClientTraffic{{Email: email, Up: up, Down: down, Enable: true}}},
			{Tag: "n1-b", Settings: `{"clients": []}`},
		}}
		if _, err := svc.setRemoteTrafficLocked(1, snap, false); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}

	sync(630, 630)
	var baselines int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("node_id = ? AND email = ?", 1, email).Count(&baselines).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if baselines != 1 {
		t.Fatalf("baseline must survive the sibling-inbound sweep, found %d rows", baselines)
	}

	sync(700, 700)
	assertUpDown(t, readTraffic(t, db, email), 70, 70, "delta accrues once baseline survives")
}

// TestMultiAttach_SameNode_DivergentSiblings reproduces #5274: a client is
// attached to several inbounds of the SAME node. Xray reports client traffic
// globally per email, so the node's enriched inbound list copies one shared
// counter onto every inbound the client is on. When those copies diverge — a
// legacy per-inbound row surviving the v3.2.x→v3.3.x upgrade, or any drift —
// the per-inbound delta loop used to treat the lower sibling as a node-counter
// reset and re-add its full value, inflating the client far past real usage.
// The merge must collapse the email to its node-wide total and count it once.
func TestMultiAttach_SameNode_DivergentSiblings(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-a", 41001, "multi")
	createNodeInboundWithClient(t, db, 1, "n1-b", 41002, "multi")
	createNodeInboundWithClient(t, db, 1, "n1-c", 41003, "multi")
	svc := &InboundService{}

	const email = "multi"
	settings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)

	// The three inbounds report the same email with diverging values; the
	// node's true per-email total is the largest (the shared global counter).
	sync := func(a, b, c int64) {
		t.Helper()
		snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
			{Tag: "n1-a", Settings: settings, ClientStats: []xray.ClientTraffic{{Email: email, Up: a, Down: a, Enable: true}}},
			{Tag: "n1-b", Settings: settings, ClientStats: []xray.ClientTraffic{{Email: email, Up: b, Down: b, Enable: true}}},
			{Tag: "n1-c", Settings: settings, ClientStats: []xray.ClientTraffic{{Email: email, Up: c, Down: c, Enable: true}}},
		}}
		if _, err := svc.setRemoteTrafficLocked(1, snap, false); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}

	// First-ever sync: row seeded at 0, canon (max of siblings = 100) becomes the
	// baseline. The node total is NOT imported as historical traffic — this prevents
	// ghost data from a previously-deleted account being counted as real usage.
	sync(100, 50, 80)
	assertUpDown(t, readTraffic(t, db, email), 0, 0, "first sync seeds at 0 — not the sum (230) nor the max (100)")

	// Second sync: canon grew from 100→150, delta=50 accrues. Siblings 60 and 90
	// do not re-add their full values — the canon clamp prevents inflation.
	sync(150, 60, 90)
	assertUpDown(t, readTraffic(t, db, email), 50, 50, "second sync: only the 50-unit increment counts, not every sibling")

	// Equal siblings (the healthy current-schema case): canon grew from 150→200,
	// delta=50 accrues once.
	sync(200, 200, 200)
	assertUpDown(t, readTraffic(t, db, email), 100, 100, "equal siblings accrue the single increment")
}

func TestDelClientStat_CleansNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	const email = "gone"
	if err := db.Create(&xray.ClientTraffic{InboundId: 1, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: email, Up: 10, Down: 10}).Error; err != nil {
		t.Fatalf("seed node baseline 1: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 2, Email: email, Up: 20, Down: 20}).Error; err != nil {
		t.Fatalf("seed node baseline 2: %v", err)
	}

	if err := svc.DelClientStat(db, email); err != nil {
		t.Fatalf("DelClientStat: %v", err)
	}

	var cnt int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected node baselines cleaned, found %d", cnt)
	}
}

func TestNodeDelete_CleansNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)
	nodeSvc := NodeService{}

	if err := db.Create(&model.NodeClientTraffic{NodeId: 7, Email: "a", Up: 1, Down: 1}).Error; err != nil {
		t.Fatalf("seed node 7 a: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 7, Email: "b", Up: 2, Down: 2}).Error; err != nil {
		t.Fatalf("seed node 7 b: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 8, Email: "c", Up: 3, Down: 3}).Error; err != nil {
		t.Fatalf("seed node 8 c: %v", err)
	}

	if err := nodeSvc.Delete(7); err != nil {
		t.Fatalf("NodeService.Delete(7): %v", err)
	}

	var sevenCnt, eightCnt int64
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", 7).Count(&sevenCnt)
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", 8).Count(&eightCnt)
	if sevenCnt != 0 {
		t.Errorf("node 7 baselines not cleaned: %d remain", sevenCnt)
	}
	if eightCnt != 1 {
		t.Errorf("node 8 baseline should survive, found %d", eightCnt)
	}
}
