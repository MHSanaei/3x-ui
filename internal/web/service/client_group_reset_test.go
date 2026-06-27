package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func groupByName(t *testing.T, svc *ClientService, name string) GroupSummary {
	t.Helper()
	rows, err := svc.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	for _, g := range rows {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("group %q not found in %v", name, rows)
	return GroupSummary{}
}

func seedGroupedClient(t *testing.T, email, group string, up, down int64) {
	t.Helper()
	if err := database.GetDB().Create(&model.ClientRecord{Email: email, Enable: true, Group: group}).Error; err != nil {
		t.Fatalf("seed client record %q: %v", email, err)
	}
	seedClientRow(t, email, 1, up, down, 0)
}

func TestResetGroupTraffic_ZeroesGroupButKeepsClients(t *testing.T) {
	initTrafficTestDB(t)
	svc := &ClientService{}

	seedGroupedClient(t, "alice", "vip", 100, 200)
	seedGroupedClient(t, "bob", "vip", 50, 50)

	before := groupByName(t, svc, "vip")
	if before.Up != 150 || before.Down != 250 || before.TrafficUsed != 400 || before.ClientCount != 2 {
		t.Fatalf("before reset: got %+v, want up=150 down=250 used=400 count=2", before)
	}

	if err := svc.ResetGroupTraffic("vip"); err != nil {
		t.Fatalf("ResetGroupTraffic: %v", err)
	}

	after := groupByName(t, svc, "vip")
	if after.Up != 0 || after.Down != 0 || after.TrafficUsed != 0 {
		t.Fatalf("after reset: got %+v, want up=0 down=0 used=0", after)
	}
	if after.ClientCount != 2 {
		t.Fatalf("after reset: client count changed to %d, want 2", after.ClientCount)
	}

	var alice xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "alice").First(&alice).Error; err != nil {
		t.Fatalf("load alice traffic: %v", err)
	}
	if alice.Up != 100 || alice.Down != 200 {
		t.Fatalf("client counter modified by group reset: alice up=%d down=%d, want 100/200", alice.Up, alice.Down)
	}
}

func TestResetGroupTraffic_NewTrafficAccumulatesAboveBaseline(t *testing.T) {
	initTrafficTestDB(t)
	svc := &ClientService{}

	seedGroupedClient(t, "carol", "team", 100, 100)
	if err := svc.ResetGroupTraffic("team"); err != nil {
		t.Fatalf("ResetGroupTraffic: %v", err)
	}
	if g := groupByName(t, svc, "team"); g.Up != 0 || g.Down != 0 {
		t.Fatalf("after reset: got %+v, want up=0 down=0", g)
	}

	if err := database.GetDB().Table("client_traffics").
		Where("email = ?", "carol").
		Updates(map[string]any{"up": 130, "down": 100}).Error; err != nil {
		t.Fatalf("bump carol traffic: %v", err)
	}

	g := groupByName(t, svc, "team")
	if g.Up != 30 || g.Down != 0 || g.TrafficUsed != 30 {
		t.Fatalf("post-bump: got %+v, want up=30 down=0 used=30", g)
	}
}

func TestResetGroupTraffic_CreatesRowForDerivedGroup(t *testing.T) {
	initTrafficTestDB(t)
	svc := &ClientService{}

	seedGroupedClient(t, "dave", "adhoc", 70, 30)

	var rows int64
	if err := database.GetDB().Model(&model.ClientGroup{}).Where("name = ?", "adhoc").Count(&rows).Error; err != nil {
		t.Fatalf("count client_groups: %v", err)
	}
	if rows != 0 {
		t.Fatalf("precondition: derived group should have no client_groups row, got %d", rows)
	}

	if err := svc.ResetGroupTraffic("adhoc"); err != nil {
		t.Fatalf("ResetGroupTraffic: %v", err)
	}

	var stored model.ClientGroup
	if err := database.GetDB().Where("name = ?", "adhoc").First(&stored).Error; err != nil {
		t.Fatalf("client_groups row not created: %v", err)
	}
	if stored.ResetUp != 70 || stored.ResetDown != 30 {
		t.Fatalf("baseline not snapshotted: got up=%d down=%d, want 70/30", stored.ResetUp, stored.ResetDown)
	}
	if g := groupByName(t, svc, "adhoc"); g.Up != 0 || g.Down != 0 {
		t.Fatalf("after reset: got %+v, want up=0 down=0", g)
	}
}

func TestResetGroupTraffic_EmptyNameRejected(t *testing.T) {
	initTrafficTestDB(t)
	svc := &ClientService{}
	if err := svc.ResetGroupTraffic("   "); err == nil {
		t.Fatal("ResetGroupTraffic(blank) = nil, want error")
	}
}
