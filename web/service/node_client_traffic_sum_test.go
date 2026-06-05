package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/xray"
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

func syncNode(t *testing.T, svc *InboundService, nodeID int, tag string, stats ...xray.ClientTraffic) {
	t.Helper()
	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{Tag: tag, ClientStats: stats}},
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

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 200, Down: 200, Enable: true})

	assertUpDown(t, readTraffic(t, db, email), 100, 100, "after baselines")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 150, Down: 150, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 260, Down: 260, Enable: true})

	assertUpDown(t, readTraffic(t, db, email), 210, 210, "after both nodes grow")
}

func TestSingleNode_MirrorsCorrectly(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "solo"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 500, Down: 600, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 500, 600, "first sync")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 700, Down: 800, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 700, 800, "second sync mirrors cumulative")
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

func TestNodeCounterReset_Clamped(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}

	const email = "restart"
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 900, Down: 900, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 950, Down: 950, Enable: true})
	assertUpDown(t, readTraffic(t, db, email), 950, 950, "before node reset")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 50, Down: 50, Enable: true})
	ct := readTraffic(t, db, email)
	if ct.Up < 0 || ct.Down < 0 {
		t.Fatalf("row went negative after node reset: up=%d down=%d", ct.Up, ct.Down)
	}
	assertUpDown(t, ct, 1000, 1000, "after node counter reset (clamped)")
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
