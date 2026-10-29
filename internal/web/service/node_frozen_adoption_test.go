package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestFrozenNodeSnapshotDoesNotRepushMetadataAfterAcknowledgedConfig(t *testing.T) {
	db := initTrafficTestDB(t)
	if err := db.Create(&model.Node{Id: 1, Name: "node", Guid: "node-guid"}).Error; err != nil {
		t.Fatal(err)
	}
	createNodeInboundWithClient(t, db, 1, "n1-stay", 41001, "kept")
	createNodeInbound(t, db, 1, "n1-delete", 41002)
	svc := &InboundService{}
	stored := `{"clients":[{"email":"kept","enable":true,"expiryTime":0,"created_at":111}]}`
	wire := `{"clients":[{"email":"kept","enable":true,"expiryTime":0,"created_at":222}]}`
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
		{Tag: "n1-stay", Enable: true, Settings: stored, ClientStats: []xray.ClientTraffic{{Email: "kept", Enable: true}}},
		{Tag: "n1-delete", Enable: true, Settings: `{"clients":[]}`},
	}}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	snap.Inbounds[0].Settings = wire
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, true); err != nil {
		t.Fatal(err)
	}
	var node model.Node
	if err := db.First(&node, 1).Error; err != nil {
		t.Fatal(err)
	}
	if node.ConfigDirty {
		t.Fatal("acknowledged push was requeued for frozen node metadata; deletion cannot converge")
	}
	snap.Inbounds = snap.Inbounds[:1]
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Inbound{}).Where("tag = ?", "n1-delete").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("node-side deletion retained %d central inbounds", count)
	}
}
