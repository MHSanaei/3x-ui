package service

import (
	"testing"

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
