package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestNodeDelete_BlocksWhenInboundsAttached guards DB-002: a node that still
// owns inbounds must not be deletable (which would orphan those inbounds with a
// dangling node_id), while a node with none deletes cleanly together with its
// traffic baselines.
func TestNodeDelete_BlocksWhenInboundsAttached(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &NodeService{}

	node := &model.Node{Name: "n1"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	createNodeInbound(t, db, node.Id, "n1-in-443", 443)

	// With an inbound attached, Delete must fail and leave node + inbound intact.
	if err := svc.Delete(node.Id); err == nil {
		t.Fatal("Delete should fail while an inbound is still attached")
	}
	var nodeCnt, ibCnt int64
	db.Model(&model.Node{}).Where("id = ?", node.Id).Count(&nodeCnt)
	db.Model(&model.Inbound{}).Where("node_id = ?", node.Id).Count(&ibCnt)
	if nodeCnt != 1 || ibCnt != 1 {
		t.Fatalf("after blocked delete: node=%d inbound=%d, want 1/1", nodeCnt, ibCnt)
	}

	// Detach the inbound and seed a traffic baseline; Delete now succeeds and
	// cleans the baseline.
	if err := db.Where("node_id = ?", node.Id).Delete(&model.Inbound{}).Error; err != nil {
		t.Fatalf("detach inbound: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: node.Id, Email: "gone"}).Error; err != nil {
		t.Fatalf("seed baseline: %v", err)
	}
	if err := svc.Delete(node.Id); err != nil {
		t.Fatalf("Delete (no inbounds attached): %v", err)
	}
	var baseCnt int64
	db.Model(&model.Node{}).Where("id = ?", node.Id).Count(&nodeCnt)
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", node.Id).Count(&baseCnt)
	if nodeCnt != 0 || baseCnt != 0 {
		t.Fatalf("after delete: node=%d baseline=%d, want 0/0", nodeCnt, baseCnt)
	}
}
