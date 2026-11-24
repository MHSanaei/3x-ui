package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestNormalizeTrafficResetDay(t *testing.T) {
	tests := map[int]int{
		0:  1,
		1:  1,
		15: 15,
		31: 31,
		32: 31,
	}
	for input, want := range tests {
		if got := normalizeTrafficResetDay(input); got != want {
			t.Errorf("normalizeTrafficResetDay(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestUpdateInboundPreserveClientTraffic(t *testing.T) {
	setupConflictDB(t)
	ib := makeInboundWithSubSortIndex("in-7006-tcp", 7006, 1)
	svc := &InboundService{}
	ib.PreserveClientTraffic = new(true)
	if _, _, err := svc.AddInbound(ib); err != nil {
		t.Fatalf("add inbound: %v", err)
	}

	for _, tc := range []struct {
		value *bool
		want  bool
	}{{nil, true}, {new(false), false}} {
		updated := *ib
		updated.PreserveClientTraffic = tc.value
		got, _, err := svc.UpdateInbound(&updated)
		if err != nil {
			t.Fatalf("update inbound: %v", err)
		}
		stored, err := svc.GetInbound(ib.Id)
		if err != nil {
			t.Fatalf("read inbound: %v", err)
		}
		if got.PreservesClientTraffic() != tc.want || stored.PreservesClientTraffic() != tc.want {
			t.Fatalf("preserve client traffic: returned %t, stored %t, want %t", got.PreservesClientTraffic(), stored.PreservesClientTraffic(), tc.want)
		}
	}
}

func TestSetRemoteTrafficPreserveClientTraffic(t *testing.T) {
	setupConflictDB(t)
	const nodeID = 1
	svc := &InboundService{}
	for _, tc := range []struct {
		value *bool
		want  bool
	}{{new(true), true}, {nil, true}, {new(false), false}} {
		snapshot := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
			Tag: "in-7007-tcp", Port: 7007, Protocol: model.VLESS, Enable: true,
			Settings: `{"clients":[]}`, TrafficReset: "monthly", PreserveClientTraffic: tc.value,
		}}}
		if _, err := svc.setRemoteTrafficLocked(nodeID, snapshot, false, false); err != nil {
			t.Fatalf("merge snapshot: %v", err)
		}
		var stored model.Inbound
		if err := database.GetDB().Where("node_id = ?", nodeID).First(&stored).Error; err != nil {
			t.Fatalf("read adopted inbound: %v", err)
		}
		if stored.PreservesClientTraffic() != tc.want {
			t.Fatalf("preserve client traffic = %t, want %t", stored.PreservesClientTraffic(), tc.want)
		}
	}
}

func TestManagedInboundChecksResetSupportBeforeSave(t *testing.T) {
	setupConflictDB(t)
	useTestRuntimeManager(t)
	node := newFakeNodeHTTP(t)
	existing := realNodeInbound(t, node, 7008, nil)
	db := database.GetDB()
	svc := &InboundService{}
	checkUnsupported := func(err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "does not support inbound-only scheduled traffic resets") {
			t.Fatalf("want node upgrade error, got %v", err)
		}
	}

	add := makeInboundWithSubSortIndex("in-7009-tcp", 7009, 1)
	add.NodeID, add.TrafficReset, add.PreserveClientTraffic = existing.NodeID, "monthly", new(true)
	_, _, err := svc.AddInbound(add)
	checkUnsupported(err)
	var count int64
	if err := db.Model(&model.Inbound{}).Where("node_id = ?", *existing.NodeID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("after refused add: inbound count=%d, err=%v", count, err)
	}

	update := *existing
	update.NodeID = nil // the edit form may omit nodeId
	update.TrafficReset, update.PreserveClientTraffic = "monthly", new(true)
	_, _, err = svc.UpdateInbound(&update)
	checkUnsupported(err)
	stored, err := svc.GetInbound(existing.Id)
	if err != nil || stored.PreservesClientTraffic() || stored.TrafficReset != "never" {
		t.Fatalf("refused edit changed the inbound: %+v, err=%v", stored, err)
	}
	_, _, dirty, _, err := (&NodeService{}).NodeSyncState(*existing.NodeID)
	if err != nil || dirty {
		t.Fatalf("after refused saves: dirty=%t, err=%v", dirty, err)
	}

	// A legacy edit can reactivate a dormant preference while omitting the flag.
	if err := db.Model(existing).Update("preserve_client_traffic", true).Error; err != nil {
		t.Fatal(err)
	}
	update.PreserveClientTraffic = nil
	_, _, err = svc.UpdateInbound(&update)
	checkUnsupported(err)

	// A retry after upgrading the node uses its new capability immediately.
	node.mu.Lock()
	node.caps = wirecodec.CapPreserveClientTraffic
	node.mu.Unlock()
	if _, _, err := svc.UpdateInbound(&update); err != nil {
		t.Fatalf("edit on upgraded node: %v", err)
	}
	if _, _, err := svc.AddInbound(add); err != nil {
		t.Fatalf("add on upgraded node: %v", err)
	}
	stored, err = svc.GetInbound(add.Id)
	if err != nil || !stored.PreservesClientTraffic() {
		t.Fatalf("accepted add did not persist the opt-in: %+v, err=%v", stored, err)
	}

	node.srv.Close()
	update.PreserveClientTraffic = new(false)
	if _, _, err := svc.UpdateInbound(&update); err != nil {
		t.Fatalf("turning the option off while the node is unreachable: %v", err)
	}
}
