package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func cachedAliasFixture(t *testing.T) (*gorm.DB, *InboundService, *runtime.TrafficSnapshot, int) {
	t.Helper()
	db := initNodeSnapshotDB(t, filepath.Join(t.TempDir(), "alias.db"))
	nodeID := 1
	seedNodeRow(t, db, &model.Node{Id: nodeID, Name: "alias", Guid: "node-guid", Address: "127.0.0.1", Port: 2096, ApiToken: "token", Enable: true})
	settings := `{"clients":[{"email":"alias-client","enable":true,"totalGB":1000,"expiryTime":0}]}`
	central := model.Inbound{UserId: 1, NodeID: &nodeID, OriginNodeGuid: "node-guid", Tag: "desired-name", Enable: true, Port: 8443, Protocol: model.VLESS, Settings: settings}
	if err := db.Create(&central).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(db, central.Id, []model.Client{{Email: "alias-client", Enable: true, TotalGB: 1000}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: central.Id, Email: "alias-client", Enable: true, Total: 1000}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	ib := central
	ib.Id, ib.Tag, ib.NodeID = 77, "already-deployed", nil
	ib.ClientStats = []xray.ClientTraffic{{Email: "alias-client", Enable: true, Total: 1000}}
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{&ib}, StructuralRevision: "alias-revision"}
	for range 4 {
		if _, err := svc.SetRemoteTraffic(nodeID, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if svc.snapshotMerges[nodeID] == nil {
		t.Fatal("converged alias never became cacheable")
	}
	return db, svc, snap, central.Id
}

func TestNodeSnapshotCachedAliasPreservesTraffic(t *testing.T) {
	db, svc, snap, centralID := cachedAliasFixture(t)
	ib := snap.Inbounds[0]
	nodeID := 1
	log := &snapshotReadLog{Interface: logger.Discard}
	oldLog := db.Logger
	db.Logger = log
	t.Cleanup(func() { db.Logger = oldLog })
	for _, up := range []int64{100, 150, 150, 5, 15} {
		ib.Up, ib.Down = up, up*2
		ib.ClientStats[0].Up, ib.ClientStats[0].Down = up, up*2
		if _, err := svc.SetRemoteTraffic(nodeID, snap, false, false); err != nil {
			t.Fatal(err)
		}
		var got model.Inbound
		if err := db.Select("id", "tag", "up", "down").First(&got, centralID).Error; err != nil {
			t.Fatal(err)
		}
		if got.Tag != "desired-name" || got.Up != up || got.Down != up*2 {
			t.Fatalf("alias counters = %+v, want %d/%d on original central row", got, up, up*2)
		}
	}
	assertUpDown(t, readTraffic(t, db, "alias-client"), 160, 320, "cached alias including counter reset")
	for _, q := range log.statements {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "select * from `inbounds`") || strings.Contains(lower, "select * from \"inbounds\"") || strings.Contains(lower, "select * from `clients`") || strings.Contains(lower, "select * from \"clients\"") {
			t.Fatalf("unchanged alias rebuilt catalog: %s", q)
		}
	}
}

func TestNodeSnapshotAliasInvalidCacheFallsBack(t *testing.T) {
	for _, broken := range []string{"missing binding", "wrong target", "extra binding", "missing membership", "master metadata"} {
		t.Run(broken, func(t *testing.T) {
			db, svc, snap, centralID := cachedAliasFixture(t)
			cache := svc.snapshotMerges[1]
			switch broken {
			case "missing binding":
				delete(cache.inboundIDs, 77)
			case "wrong target":
				cache.inboundIDs[77] = centralID + 1000
			case "extra binding":
				cache.inboundIDs[78] = centralID
			case "missing membership":
				delete(cache.attached, 77)
			case "master metadata":
				if err := db.Exec("UPDATE inbounds SET remark = 'master edit' WHERE id = ?", centralID).Error; err != nil {
					t.Fatal(err)
				}
			}
			log := &snapshotReadLog{Interface: logger.Discard}
			previous := db.Logger
			db.Logger = log
			t.Cleanup(func() { db.Logger = previous })
			snap.Inbounds[0].Up, snap.Inbounds[0].ClientStats[0].Up = 90, 90
			if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
				t.Fatal(err)
			}
			assertUpDown(t, readTraffic(t, db, "alias-client"), 90, 0, "fallback preserved alias delta")
			var ib model.Inbound
			if err := db.Select("id", "up").First(&ib, centralID).Error; err != nil {
				t.Fatal(err)
			}
			if ib.Up != 90 {
				t.Fatalf("inbound up=%d, want 90", ib.Up)
			}
			rebuilt := false
			for _, q := range log.statements {
				if strings.Contains(q, "SELECT * FROM `inbounds`") || strings.Contains(q, "SELECT * FROM \"inbounds\"") {
					rebuilt = true
				}
			}
			if !rebuilt {
				t.Fatal("invalid cache did not fall back to catalog reconciliation")
			}
		})
	}
}

func TestNodeSnapshotAliasQuotaDisableAndRenewal(t *testing.T) {
	db, svc, snap, centralID := cachedAliasFixture(t)
	ib := snap.Inbounds[0]
	oldExpiry := time.Now().Add(24 * time.Hour).UnixMilli()
	// A lifecycle change forces a full merge, then the stable disabled state becomes cacheable.
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "alias-client").Update("expiry_time", oldExpiry).Error; err != nil {
		t.Fatal(err)
	}
	ib.ClientStats[0].Up, ib.ClientStats[0].Enable, ib.ClientStats[0].ExpiryTime = 1200, false, oldExpiry
	ib.Settings = fmt.Sprintf(`{"clients":[{"email":"alias-client","enable":false,"totalGB":1000,"expiryTime":%d}]}`, oldExpiry)
	for range 4 {
		if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if got := readTraffic(t, db, "alias-client"); got.Enable || got.Up != 1200 || got.Total != 1000 {
		t.Fatalf("quota latch=%+v", got)
	}
	if svc.snapshotMerges[1] == nil {
		t.Fatal("disabled alias did not converge to cache")
	}
	ib.ClientStats[0].Up = 1250
	if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	if got := readTraffic(t, db, "alias-client"); got.Enable || got.Up != 1250 {
		t.Fatalf("disabled cached usage=%+v", got)
	}
	newExpiry := oldExpiry + int64(24*time.Hour/time.Millisecond)
	ib.ClientStats[0].Reset = 30
	ib.ClientStats[0].Up, ib.ClientStats[0].Enable, ib.ClientStats[0].ExpiryTime, ib.ClientStats[0].Total = 5, true, newExpiry, 2000
	ib.Settings = fmt.Sprintf(`{"clients":[{"email":"alias-client","enable":true,"totalGB":2000,"expiryTime":%d}]}`, newExpiry)
	snap.StructuralRevision = "renewed-alias"
	for range 4 {
		if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if got := readTraffic(t, db, "alias-client"); !got.Enable || got.Up != 5 || got.Total != 2000 || got.ExpiryTime != newExpiry {
		t.Fatalf("renewal=%+v", got)
	}
	if svc.snapshotMerges[1] == nil {
		t.Fatal("renewed alias did not converge to cache")
	}
	ib.ClientStats[0].Up = 25
	ib.Up = 25
	if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	if got := readTraffic(t, db, "alias-client"); !got.Enable || got.Up != 25 {
		t.Fatalf("post-renewal cached usage=%+v", got)
	}
	var count int64
	db.Model(&model.Inbound{}).Where("id = ? AND tag = ?", centralID, "desired-name").Count(&count)
	if count != 1 {
		t.Fatalf("lost adopted central alias: %d", count)
	}
}

func TestNodeSnapshotAliasValidatesEntireMapping(t *testing.T) {
	db, svc, snap, centralID := cachedAliasFixture(t)
	nodeID := 1
	second := model.Inbound{UserId: 1, NodeID: &nodeID, OriginNodeGuid: "node-guid", Tag: "second-desired", Enable: true, Port: 9443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	remote := second
	remote.Id, remote.Tag, remote.NodeID = 88, "second-deployed", nil
	snap.Inbounds = append(snap.Inbounds, &remote)
	for range 4 {
		if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if cache := svc.snapshotMerges[1]; cache == nil || len(cache.inboundIDs) != 2 {
		t.Fatal("two aliases did not converge to cache")
	}
	svc.snapshotMerges[1].inboundIDs[88] = centralID
	snap.Inbounds[0].Up, snap.Inbounds[0].ClientStats[0].Up, remote.Up = 25, 25, 75
	if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		id int
		up int64
	}{{centralID, 25}, {second.Id, 75}} {
		var got model.Inbound
		if err := db.Select("id", "up").First(&got, want.id).Error; err != nil {
			t.Fatal(err)
		}
		if got.Up != want.up {
			t.Fatalf("partial cached merge on inbound %d: up=%d want %d", want.id, got.Up, want.up)
		}
	}
	assertUpDown(t, readTraffic(t, db, "alias-client"), 25, 0, "full fallback preserved first alias")
}
