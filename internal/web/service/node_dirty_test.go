package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// While a node is config-dirty (a local edit committed before it could be
// mirrored to the node), the traffic pull must not overwrite the central
// inbound's config columns from the node's stale snapshot — only traffic
// counters may advance. Otherwise a reconnecting node reverts the edit.
func TestSetRemoteTraffic_DirtyPreservesConfig(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	id := node.Id

	const desiredSettings = `{"clients":[{"email":"a@x"}]}`
	central := &model.Inbound{
		UserId:   1,
		NodeID:   &id,
		Tag:      "in-443-tcp",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: desiredSettings,
	}
	if err := db.Create(central).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Tag:      "in-443-tcp",
			Enable:   true,
			Port:     443,
			Protocol: model.VLESS,
			Settings: `{"clients":[{"email":"b@x"}]}`,
			Up:       500,
			Down:     700,
		}},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(id, snap, true); err != nil {
		t.Fatalf("setRemoteTrafficLocked dirty: %v", err)
	}

	var got model.Inbound
	if err := db.First(&got, central.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if got.Settings != desiredSettings {
		t.Fatalf("dirty pull overwrote settings: want %q got %q", desiredSettings, got.Settings)
	}
	if got.Up != 500 || got.Down != 700 {
		t.Fatalf("traffic counters not applied while dirty: up=%d down=%d", got.Up, got.Down)
	}
}

// Deleting a *disabled* client attached to a node inbound must still propagate
// to the node. The node's own DB carries the (disabled) client, so the central
// panel has to mark the node dirty (→ reconcile) instead of dropping the delete
// and letting the next traffic snapshot resurrect the client. Regression for
// the enable-flag gate that used to skip the node path entirely (#5352).
func TestDelInboundClientByEmail_DisabledNodeClientMarksDirty(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	// Offline node so nodePushPlan reports dirty without needing a live runtime.
	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "offline"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	id := node.Id

	central := &model.Inbound{
		UserId:   1,
		NodeID:   &id,
		Tag:      "in-443-tcp",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"email":"a@x","enable":false}]}`,
	}
	if err := db.Create(central).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if _, err := clientSvc.DelInboundClientByEmail(inboundSvc, central.Id, "a@x", false); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}

	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(id); err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	} else if !dirty {
		t.Fatal("deleting a disabled node client must mark the node dirty (#5352)")
	}
}

// ClearNodeDirty must be a compare-and-swap on config_dirty_at so a concurrent
// edit that re-dirties the node during a reconcile is not silently cleared.
func TestNodeDirty_ClearIsCASOnDirtyAt(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	node := &model.Node{Name: "n2", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	nodeSvc := NodeService{}
	if err := nodeSvc.MarkNodeDirty(node.Id); err != nil {
		t.Fatalf("MarkNodeDirty: %v", err)
	}
	_, _, dirty, dirtyAt, err := nodeSvc.NodeSyncState(node.Id)
	if err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	}
	if !dirty {
		t.Fatal("node should be dirty after MarkNodeDirty")
	}

	if err := nodeSvc.ClearNodeDirty(node.Id, dirtyAt-1); err != nil {
		t.Fatalf("ClearNodeDirty stale token: %v", err)
	}
	if _, _, stillDirty, _, _ := nodeSvc.NodeSyncState(node.Id); !stillDirty {
		t.Fatal("stale-token clear must not clear the dirty flag")
	}

	if err := nodeSvc.ClearNodeDirty(node.Id, dirtyAt); err != nil {
		t.Fatalf("ClearNodeDirty matching token: %v", err)
	}
	if _, _, stillDirty, _, _ := nodeSvc.NodeSyncState(node.Id); stillDirty {
		t.Fatal("matching-token clear must clear the dirty flag")
	}
}

func TestMarkNodeDirtyTxRollsBackWithTransaction(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	node := &model.Node{Name: "n3", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	nodeSvc := NodeService{}
	rollbackErr := errors.New("force rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := nodeSvc.MarkNodeDirtyTx(tx, node.Id); err != nil {
			return err
		}
		return rollbackErr
	}); !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback tx: got %v want %v", err, rollbackErr)
	}
	if _, _, dirty, _, err := nodeSvc.NodeSyncState(node.Id); err != nil {
		t.Fatalf("NodeSyncState after rollback: %v", err)
	} else if dirty {
		t.Fatal("dirty flag escaped a rolled-back transaction")
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return nodeSvc.MarkNodeDirtyTx(tx, node.Id)
	}); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	if _, _, dirty, _, err := nodeSvc.NodeSyncState(node.Id); err != nil {
		t.Fatalf("NodeSyncState after commit: %v", err)
	} else if !dirty {
		t.Fatal("dirty flag should commit with its transaction")
	}
}

// Editing a node must mark it config-dirty so the next traffic-sync tick
// reconciles (pushes the panel's inbounds to the remote) before pulling a
// snapshot. Without the dirty flag, re-pointing a node to a fresh server
// makes the orphan sweep delete every central inbound absent from the empty
// snapshot (#5461).
func TestNodeService_UpdateMarksNodeDirty(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	node := &model.Node{
		Name:     "n1",
		Address:  "10.0.0.1",
		Port:     2096,
		ApiToken: "tok",
		Enable:   true,
		Status:   "online",
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	edited := &model.Node{
		Name:     node.Name,
		Address:  "10.0.0.2",
		Port:     2097,
		ApiToken: node.ApiToken,
		Enable:   true,
	}
	nodeSvc := NodeService{}
	if err := nodeSvc.Update(node.Id, edited); err != nil {
		t.Fatalf("Update: %v", err)
	}

	_, _, dirty, _, err := nodeSvc.NodeSyncState(node.Id)
	if err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	}
	if !dirty {
		t.Fatal("Update must mark the node config-dirty so sync reconciles before snapshot sweep (#5461)")
	}

	var got model.Node
	if err := db.First(&got, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if got.Address != "10.0.0.2" || got.Port != 2097 {
		t.Fatalf("node row not updated: address=%q port=%d", got.Address, got.Port)
	}
}
