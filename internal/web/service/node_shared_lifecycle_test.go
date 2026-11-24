package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeSharedLifecycle_DoesNotOscillateAcrossNodes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		noStats bool
	}{
		{"enabled master ignores stale disable", true, false},
		{"blocked master ignores stale enable", false, false},
		{"settings without stats keep master enable", true, true},
		{"settings without stats keep master block", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enabled := tc.enabled
			db := initTrafficTestDB(t)
			const email = "shared-lifecycle"
			createNodeInboundWithClient(t, db, 1, "n1-shared", 41001, email)
			createNodeInboundWithClient(t, db, 2, "n2-shared", 41002, email)
			svc := &InboundService{}
			client := model.Client{Email: email, Enable: true, ExpiryTime: lateAbs, TotalGB: 1000}
			stats := xray.ClientTraffic{Email: email, Enable: true, ExpiryTime: lateAbs, Total: 1000}
			settings := clientsSettings(t, []model.Client{client})
			syncNodeWithSettings(t, svc, 1, "n1-shared", settings, stats)
			var second model.Inbound
			if err := db.Where("tag = ?", "n2-shared").First(&second).Error; err != nil {
				t.Fatal(err)
			}
			if err := svc.clientService.SyncInbound(db, second.Id, []model.Client{client}); err != nil {
				t.Fatal(err)
			}
			syncNodeWithSettings(t, svc, 2, "n2-shared", settings, stats)
			owner := readTraffic(t, db, email).InboundId
			if owner == second.Id {
				t.Fatal("fixture must home the shared traffic row on the other node")
			}
			if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("enable", enabled).Error; err != nil {
				t.Fatal(err)
			}

			for tick, nodeID := range []int{2, 1, 2, 1} {
				tag := "n1-shared"
				client.Enable = enabled
				if nodeID == 2 {
					tag = "n2-shared"
					client.Enable = !enabled
				}
				stats.Enable = client.Enable
				reported := []xray.ClientTraffic{stats}
				if tc.noStats && nodeID == 2 {
					reported = nil
				}
				syncNodeWithSettings(t, svc, nodeID, tag, clientsSettings(t, []model.Client{client}), reported...)

				var record model.ClientRecord
				if err := db.Where("email = ?", email).First(&record).Error; err != nil {
					t.Fatal(err)
				}
				if record.Enable != enabled || record.ExpiryTime != lateAbs {
					t.Fatalf("tick %d node %d: client enable/expiry = %v/%d, want %v/%d", tick, nodeID, record.Enable, record.ExpiryTime, enabled, lateAbs)
				}
				traffic := readTraffic(t, db, email)
				if traffic.Enable != enabled || traffic.InboundId != owner || traffic.Total != 1000 || traffic.Up != 0 || traffic.Down != 0 {
					t.Fatalf("tick %d changed the shared traffic lifecycle or accounting: %+v", tick, traffic)
				}
				var inbound model.Inbound
				if err := db.Where("tag = ?", tag).First(&inbound).Error; err != nil {
					t.Fatal(err)
				}
				copies, err := svc.GetClients(&inbound)
				if err != nil {
					t.Fatal(err)
				}
				if len(copies) != 1 || copies[0].Email != email || copies[0].Enable != enabled || copies[0].ExpiryTime != lateAbs {
					t.Fatalf("tick %d retained a stale inbound lifecycle: %+v", tick, copies)
				}
			}
		})
	}
}

func TestNodeSharedLifecycle_RenewalOnNonOwnerResetsSharedPeriod(t *testing.T) {
	db := initTrafficTestDB(t)
	const email = "shared-renewal"
	createNodeInboundWithClient(t, db, 1, "n1-renew", 41001, email)
	createNodeInboundWithClient(t, db, 2, "n2-renew", 41002, email)
	svc := &InboundService{}
	client := model.Client{Email: email, Enable: true, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, TotalGB: 10000}
	stats := xray.ClientTraffic{Email: email, Enable: true, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays, Total: 10000}
	settings := clientsSettings(t, []model.Client{client})
	syncNodeWithSettings(t, svc, 1, "n1-renew", settings, stats)
	stats.Up, stats.Down = 900, 100
	syncNodeWithSettings(t, svc, 1, "n1-renew", settings, stats)
	stats.Up, stats.Down = 400, 50
	var second model.Inbound
	if err := db.Where("tag = ?", "n2-renew").First(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.clientService.SyncInbound(db, second.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	syncNodeWithSettings(t, svc, 2, "n2-renew", settings, stats)
	owner := readTraffic(t, db, email).InboundId
	if owner == second.Id {
		t.Fatal("fixture must home the shared traffic row on the other node")
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "other-master", Email: email, Up: 250}).Error; err != nil {
		t.Fatal(err)
	}

	client.ExpiryTime, stats.ExpiryTime = renewSecondExpiry, renewSecondExpiry
	stats.Up, stats.Down, stats.ResetCount = 5, 2, 1
	syncNodeWithSettings(t, svc, 2, "n2-renew", clientsSettings(t, []model.Client{client}), stats)
	got := readTraffic(t, db, email)
	if !got.Enable || got.ExpiryTime != renewSecondExpiry || got.Up != 5 || got.Down != 2 || got.ResetCount != 1 || got.InboundId != owner {
		t.Fatalf("non-owner renewal did not replace the shared period: %+v", got)
	}
	var globals int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", email).Count(&globals).Error; err != nil {
		t.Fatal(err)
	}
	if globals != 0 {
		t.Fatalf("non-owner renewal retained %d stale global traffic rows", globals)
	}
}
