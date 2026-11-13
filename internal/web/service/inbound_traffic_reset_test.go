package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
