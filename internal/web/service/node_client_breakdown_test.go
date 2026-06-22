package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The node-page client breakdown must classify by EMAIL, not by
// client_traffics.inbound_id — that column goes stale after an inbound is
// delete+recreated, so filtering by it drops almost every row and undercounts
// active/disabled/ended. Here the traffic rows carry a bogus inbound_id (999)
// yet must still be matched to the node's clients by email.
func TestGetAll_ClientBreakdownMatchesByEmailNotStaleInboundId(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	svc := NodeService{}

	if err := db.Create(&model.Node{Id: 1, Name: "n", Address: "10.0.0.1", Port: 2053, ApiToken: "t", Guid: "g"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nid := 1
	ib := &model.Inbound{Tag: "n1-in", Enable: true, Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	mk := func(id int, email string, enable bool) {
		if err := db.Create(&model.ClientRecord{Id: id, Email: email, Enable: enable}).Error; err != nil {
			t.Fatalf("create client %s: %v", email, err)
		}
		if !enable {
			// Enable has gorm:"default:true", so a zero-value (false) create is
			// dropped and the DB applies true — force the disabled state explicitly.
			if err := db.Model(&model.ClientRecord{}).Where("id = ?", id).Update("enable", false).Error; err != nil {
				t.Fatalf("disable client %s: %v", email, err)
			}
		}
		if err := db.Create(&model.ClientInbound{ClientId: id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("attach client %s: %v", email, err)
		}
	}
	mk(1, "active@x", true)
	mk(2, "disabled@x", false)
	mk(3, "depleted@x", true)

	// Traffic rows carry a STALE inbound_id (999), unrelated to the node inbound.
	const staleInboundID = 999
	rows := []*xray.ClientTraffic{
		{InboundId: staleInboundID, Email: "active@x", Enable: true},
		{InboundId: staleInboundID, Email: "disabled@x", Enable: false},
		{InboundId: staleInboundID, Email: "depleted@x", Enable: true, Total: 100, Up: 60, Down: 60}, // exhausted
	}
	for _, r := range rows {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("create traffic %s: %v", r.Email, err)
		}
	}

	nodes, err := svc.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	var n *model.Node
	for _, x := range nodes {
		if x.Id == 1 {
			n = x
		}
	}
	if n == nil {
		t.Fatal("node 1 not found")
	}
	if n.ClientCount != 3 {
		t.Errorf("ClientCount = %d, want 3", n.ClientCount)
	}
	if n.ActiveCount != 1 {
		t.Errorf("ActiveCount = %d, want 1 (matched by email despite stale inbound_id)", n.ActiveCount)
	}
	if n.DisabledCount != 1 {
		t.Errorf("DisabledCount = %d, want 1", n.DisabledCount)
	}
	if n.DepletedCount != 1 {
		t.Errorf("DepletedCount = %d, want 1", n.DepletedCount)
	}
}
