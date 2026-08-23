package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Deleting an inbound that a sub-balancer selects must strip its id from
// InboundIds, leaving no dangling member reference (#5648 mirrors the hosts
// cascade). With the only member gone the balancer stops emitting a doc.
func TestDelInboundClearsSubBalancerInboundIds(t *testing.T) {
	setupSubBalancerDB(t)
	ib := &model.Inbound{UserId: 1, Tag: "cleanup", Enable: false, Listen: "203.0.113.7", Port: 5001, Protocol: model.VLESS, Remark: "cleanup", Settings: `{}`, StreamSettings: `{}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	balSvc := &SubBalancerService{}
	bal, err := balSvc.Create(&model.SubBalancer{Remark: "bal", Strategy: "random", InboundIds: []int{ib.Id}, SortOrder: 1, Enabled: true})
	if err != nil {
		t.Fatalf("create balancer: %v", err)
	}

	if _, err := (&InboundService{}).DelInbound(ib.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}

	stored, err := balSvc.Get(bal.Id)
	if err != nil {
		t.Fatalf("get balancer: %v", err)
	}
	if len(stored.InboundIds) != 0 {
		t.Fatalf("InboundIds = %v, want empty (no dangling id)", stored.InboundIds)
	}
}
