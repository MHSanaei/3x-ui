package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// capPoolAtOneConn models a pool whose other connections are all parked on the
// transaction's row locks: any query outside tx then waits forever.
func capPoolAtOneConn(t *testing.T) {
	t.Helper()
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

func runWithoutSecondConn(t *testing.T, name string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		// The DB cleanup's Close fails the parked pool request, which unwinds fn.
		t.Fatalf("%s blocked waiting for a second pool connection while its transaction is open", name)
		return nil
	}
}

func createNodeInboundWithSettings(t *testing.T, db *gorm.DB, nodeID int, tag string, port int, settings string) {
	t.Helper()
	nid := nodeID
	ib := &model.Inbound{UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS, NodeID: &nid, Settings: settings}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create node inbound %q: %v", tag, err)
	}
}

func syncNodeSnapshot(t *testing.T, svc *InboundService, nodeID int, inbounds ...*model.Inbound) error {
	t.Helper()
	_, err := svc.setRemoteTrafficLocked(nodeID, &runtime.TrafficSnapshot{Inbounds: inbounds}, false, false)
	return err
}

func assertNoTraffic(t *testing.T, email string) {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(xray.ClientTraffic{}).Where("email = ?", email).Count(&n).Error; err != nil {
		t.Fatalf("count traffic %q: %v", email, err)
	}
	if n != 0 {
		t.Fatalf("traffic rows for %q = %d, want 0", email, n)
	}
}

func TestNodeSyncInboundRemovalStaysOnOneConnection(t *testing.T) {
	db := initTrafficTestDB(t)
	goneSettings := `{"clients": [{"email": "gone", "enable": true}]}`
	keepSettings := `{"clients": [{"email": "keep", "enable": true}]}`
	createNodeInboundWithSettings(t, db, 1, "n1-gone", 41001, goneSettings)
	createNodeInboundWithSettings(t, db, 1, "n1-keep", 41002, keepSettings)
	svc := &InboundService{}
	keep := &model.Inbound{Tag: "n1-keep", Enable: true, Settings: keepSettings, ClientStats: []xray.ClientTraffic{{Email: "keep", Up: 10, Down: 10, Enable: true}}}
	if err := syncNodeSnapshot(t, svc, 1,
		&model.Inbound{Tag: "n1-gone", Enable: true, Settings: goneSettings, ClientStats: []xray.ClientTraffic{{Email: "gone", Up: 10, Down: 10, Enable: true}}},
		keep); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	readTraffic(t, db, "gone")

	capPoolAtOneConn(t)
	err := runWithoutSecondConn(t, "node sync inbound-removal sweep", func() error {
		return syncNodeSnapshot(t, svc, 1, keep)
	})
	if err != nil {
		t.Fatalf("sync node 1 without n1-gone: %v", err)
	}
	assertNoTraffic(t, "gone")
}

func TestNodeSyncClientGoneStaysOnOneConnection(t *testing.T) {
	db := initTrafficTestDB(t)
	bothSettings := `{"clients": [{"email": "gone", "enable": true}, {"email": "keep", "enable": true}]}`
	createNodeInboundWithSettings(t, db, 1, "n1-in", 41001, bothSettings)
	svc := &InboundService{}
	if err := syncNodeSnapshot(t, svc, 1, &model.Inbound{Tag: "n1-in", Enable: true, Settings: bothSettings, ClientStats: []xray.ClientTraffic{
		{Email: "gone", Up: 10, Down: 10, Enable: true},
		{Email: "keep", Up: 10, Down: 10, Enable: true},
	}}); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	readTraffic(t, db, "gone")

	capPoolAtOneConn(t)
	err := runWithoutSecondConn(t, "node sync per-client sweep", func() error {
		return syncNodeSnapshot(t, svc, 1, &model.Inbound{
			Tag:         "n1-in",
			Enable:      true,
			Settings:    `{"clients": [{"email": "keep", "enable": true}]}`,
			ClientStats: []xray.ClientTraffic{{Email: "keep", Up: 20, Down: 20, Enable: true}},
		})
	})
	if err != nil {
		t.Fatalf("sync node 1 without the gone client: %v", err)
	}
	var baselines int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("node_id = ? AND email = ?", 1, "gone").Count(&baselines).Error; err != nil {
		t.Fatalf("count node baselines: %v", err)
	}
	if baselines != 0 {
		t.Fatalf("node baseline for the gone client = %d rows, want 0", baselines)
	}
}

func TestUpdateClientTrafficsStaysOnOneConnection(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	clients := []model.Client{{Email: "gone@x", ID: "aaaaaaaa-0000-0000-0000-000000000101", Enable: true}}
	oldIb := mkInbound(t, 22101, model.VLESS, clientsSettings(t, clients))
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: oldIb.Id, Email: "gone@x", Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	newIb := *oldIb
	newIb.Settings = `{"clients": []}`

	capPoolAtOneConn(t)
	err := runWithoutSecondConn(t, "inbound edit dropping a client", func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			return svc.updateClientTraffics(tx, oldIb, &newIb)
		})
	})
	if err != nil {
		t.Fatalf("updateClientTraffics: %v", err)
	}
	var n int64
	database.GetDB().Model(xray.ClientTraffic{}).Where("email = ?", "gone@x").Count(&n)
	if n != 0 {
		t.Fatalf("traffic rows for dropped client = %d, want 0", n)
	}
}

func TestUpdateInboundClientRenameOntoExistingStatStaysOnOneConnection(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	source := []model.Client{{Email: "old@x", ID: "aaaaaaaa-0000-0000-0000-000000000102", SubID: "sub-102", Enable: true}}
	ib := mkInbound(t, 22102, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "new@x", Enable: true}).Error; err != nil {
		t.Fatalf("seed target traffic: %v", err)
	}

	renamed := source
	renamed[0].Email = "new@x"
	capPoolAtOneConn(t)
	err := runWithoutSecondConn(t, "client rename onto an existing stats row", func() error {
		_, err := svc.UpdateInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, renamed)}, "old@x")
		return err
	})
	if err != nil {
		t.Fatalf("UpdateInboundClient: %v", err)
	}
	var n int64
	database.GetDB().Model(xray.ClientTraffic{}).Where("email = ?", "old@x").Count(&n)
	if n != 0 {
		t.Fatalf("traffic rows for renamed-away email = %d, want 0", n)
	}
}
