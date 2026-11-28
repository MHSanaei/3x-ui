package service

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestActivationRepairIsScoped(t *testing.T) {
	testActivationRepairIsScoped(t, initTrafficTestDB(t))
}

func TestActivationRepairIsScopedPostgres(t *testing.T) {
	testActivationRepairIsScoped(t, durablePostgresDB(t))
}

func TestNodeActivationRepairsExpiryWithoutClientReconciliation(t *testing.T) {
	testNodeActivationRepairsExpiryWithoutClientReconciliation(t, initTrafficTestDB(t))
}

func TestNodeActivationRepairsExpiryWithoutClientReconciliationPostgres(t *testing.T) {
	testNodeActivationRepairsExpiryWithoutClientReconciliation(t, durablePostgresDB(t))
}

func testNodeActivationRepairsExpiryWithoutClientReconciliation(t *testing.T, db *gorm.DB) {
	t.Helper()
	const duration = -60_000
	deadline := time.Now().Add(24 * time.Hour).UnixMilli()
	for _, tc := range []struct {
		name         string
		storedExpiry int64
		renewal      bool
	}{
		{name: "first traffic row"},
		{name: "first use", storedExpiry: duration},
		{name: "renewal", storedExpiry: deadline - time.Hour.Milliseconds(), renewal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tag := fmt.Sprintf("expiry-without-reconcile-%d", time.Now().UnixNano())
			email := tag + "@x"
			node := model.Node{Name: tag, Address: "127.0.0.1", Port: 2096, Enable: true}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			createNodeInboundWithClient(t, db, node.Id, tag, 42533, email)
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).UpdateColumn("expiry_time", duration).Error; err != nil {
				t.Fatal(err)
			}

			reported := xray.ClientTraffic{Email: email, ExpiryTime: deadline, Up: 1, Enable: true}
			if tc.storedExpiry != 0 {
				var inbound model.Inbound
				if err := db.Where("tag = ?", tag).First(&inbound).Error; err != nil {
					t.Fatal(err)
				}
				stored := xray.ClientTraffic{Email: email, InboundId: inbound.Id, ExpiryTime: tc.storedExpiry, Enable: true}
				if tc.renewal {
					stored.Reset, stored.ResetCount = 30, 1
					reported.Reset, reported.ResetCount = 30, 2
					if err := db.Create(&model.NodeClientTraffic{NodeId: node.Id, Email: email}).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := db.Create(&stored).Error; err != nil {
					t.Fatal(err)
				}
			}

			// The traffic snapshot is usable, but this settings payload cannot rebuild client records.
			snapshot := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
				Tag: tag, Settings: `{"clients":{}}`, ClientStats: []xray.ClientTraffic{reported},
			}}}
			if _, err := (&InboundService{}).setRemoteTrafficLocked(node.Id, snapshot, false, false); err != nil {
				t.Fatal(err)
			}
			traffic := readTraffic(t, db, email)
			if traffic.ExpiryTime != deadline || traffic.ResetCount != reported.ResetCount {
				t.Fatalf("central traffic expiry/count = %d/%d, want %d/%d", traffic.ExpiryTime, traffic.ResetCount, deadline, reported.ResetCount)
			}
			var record model.ClientRecord
			if err := db.Where("email = ?", email).First(&record).Error; err != nil {
				t.Fatal(err)
			}
			if record.ExpiryTime != deadline {
				t.Fatalf("client record expiry = %d, want %d", record.ExpiryTime, deadline)
			}
		})
	}
}

func testActivationRepairIsScoped(t *testing.T, db *gorm.DB) {
	t.Helper()
	tag := fmt.Sprintf("expiry-scope-%d", time.Now().UnixNano())
	email, other := tag+"@x", tag+"-other@x"
	node := &model.Node{Name: tag, Address: "127.0.0.1", Port: 2096, Enable: true}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	createNodeInboundWithClient(t, db, node.Id, tag, 42533, email)
	deadline := time.Now().Add(24 * time.Hour).UnixMilli()
	const duration = -60_000
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).UpdateColumn("expiry_time", duration).Error; err != nil {
		t.Fatal(err)
	}
	unrelated := &model.ClientRecord{Email: other, ExpiryTime: duration}
	if err := db.Create(unrelated).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: other, ExpiryTime: deadline, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	readExpiry := func(email string) int64 {
		t.Helper()
		var rec model.ClientRecord
		if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
			t.Fatal(err)
		}
		return rec.ExpiryTime
	}
	svc := &InboundService{}
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"enable":true,"expiryTime":%d}]}`, email, duration)
	syncNodeWithSettings(t, svc, node.Id, tag, settings, xray.ClientTraffic{Email: email, ExpiryTime: duration, Enable: true})
	syncNodeWithSettings(t, svc, node.Id, tag, settings, xray.ClientTraffic{Email: email, ExpiryTime: deadline, Up: 1, Enable: true})
	if got := readExpiry(email); got != deadline {
		t.Fatalf("activated expiry = %d, want %d", got, deadline)
	}
	if got := readExpiry(other); got != duration {
		t.Fatalf("merge changed unrelated expiry to %d", got)
	}

	// Start at the preceding primary key so it is covered even on a large database.
	next, err := (&ClientService{}).RepairClientActivationExpiries(unrelated.Id - 1)
	if err != nil || next < unrelated.Id {
		t.Fatalf("repair cursor = %d, err = %v", next, err)
	}
	if got := readExpiry(other); got != deadline {
		t.Fatalf("repaired expiry = %d, want %d", got, deadline)
	}
}
