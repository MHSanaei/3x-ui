package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func countClientRows(t *testing.T, db *gorm.DB, email string) (records, traffics int64) {
	t.Helper()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Count(&records).Error; err != nil {
		t.Fatalf("count clients %q: %v", email, err)
	}
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Count(&traffics).Error; err != nil {
		t.Fatalf("count client_traffics %q: %v", email, err)
	}
	return records, traffics
}

func seedNodeRow(t *testing.T, db *gorm.DB, n *model.Node) {
	t.Helper()
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
}

func snapshotWithClients(t *testing.T, tag, settings string, stats ...xray.ClientTraffic) *runtime.TrafficSnapshot {
	t.Helper()
	return &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{Tag: tag, Settings: settings, ClientStats: stats}},
	}
}

func snapshotWithoutClients(t *testing.T, tag string) *runtime.TrafficSnapshot {
	t.Helper()
	return snapshotWithClients(t, tag, `{"clients":[]}`)
}

func snapshotWithTwoInbounds(t *testing.T, tagA, settingsA, emailA, tagB, settingsB, emailB string) *runtime.TrafficSnapshot {
	t.Helper()
	return &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{
			{Tag: tagA, Settings: settingsA, ClientStats: []xray.ClientTraffic{{Email: emailA, Enable: true}}},
			{Tag: tagB, Settings: settingsB, ClientStats: []xray.ClientTraffic{{Email: emailB, Enable: true}}},
		},
	}
}

// The job samples config_dirty before the snapshot round-trip; a client added
// in that window is deleted again unless the merge re-reads the flag itself.
func TestSetRemoteTrafficRereadsConfigDirty(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const email = "carol"
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, email)
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"enable":true}]}`, email)
	syncNodeWithSettings(t, svc, 1, "n1-in", settings,
		xray.ClientTraffic{Email: email, Up: 1, Down: 1, Enable: true})

	if rec, _ := countClientRows(t, db, email); rec != 1 {
		t.Fatalf("setup: client not attached, got %d rows", rec)
	}

	if err := db.Model(model.Node{}).Where("id = ?", 1).Update("config_dirty", true).Error; err != nil {
		t.Fatalf("mark node dirty: %v", err)
	}

	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	rec, traf := countClientRows(t, db, email)
	if rec != 1 || traf != 1 {
		t.Fatalf("stale dirty=false wiped a client added mid-flight: clients=%d client_traffics=%d, want 1/1", rec, traf)
	}
}

// FilterNodeSnapshot stops reporting a deselected tag; reading that absence as
// "the node deleted it" wiped the master's copy of an inbound still running.
func TestDeselectedTagIsNotSwept(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{
		Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true,
		InboundSyncMode: "selected", InboundTags: []string{"keep"},
	})

	createNodeInboundWithClient(t, db, 1, "keep", 41001, "kept@x")
	createNodeInboundWithClient(t, db, 1, "drop", 41002, "dropped@x")

	keepSettings := `{"clients":[{"email":"kept@x","enable":true}]}`
	dropSettings := `{"clients":[{"email":"dropped@x","enable":true}]}`
	snap := snapshotWithTwoInbounds(t, "keep", keepSettings, "kept@x", "drop", dropSettings, "dropped@x")
	if _, err := svc.setRemoteTrafficLocked(1, snap, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if rec, traf := countClientRows(t, db, "dropped@x"); rec != 1 || traf != 1 {
		t.Fatalf("setup: dropped@x not seeded, clients=%d client_traffics=%d", rec, traf)
	}

	keepOnly := snapshotWithClients(t, "keep", keepSettings, xray.ClientTraffic{Email: "kept@x", Enable: true})
	if _, err := svc.setRemoteTrafficLocked(1, keepOnly, false); err != nil {
		t.Fatalf("post-deselect sync: %v", err)
	}

	rec, traf := countClientRows(t, db, "dropped@x")
	if rec != 1 || traf != 1 {
		t.Fatalf("deselecting a tag deleted its clients: clients=%d client_traffics=%d, want 1/1", rec, traf)
	}
	var inbounds int64
	if err := db.Model(model.Inbound{}).Where("tag = ?", "drop").Count(&inbounds).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if inbounds != 1 {
		t.Fatalf("deselecting a tag deleted the inbound the node still serves: got %d rows, want 1", inbounds)
	}
}

func TestSyncInboundStoresTrimmedEmail(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &ClientService{}

	ib := &model.Inbound{UserId: 1, Tag: "trim-in", Enable: true, Port: 41501, Protocol: model.VLESS}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	padded := "bob "
	clients := []model.Client{{Email: padded, Enable: true}}
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("first SyncInbound: %v", err)
	}

	var stored []string
	if err := db.Model(&model.ClientRecord{}).Pluck("email", &stored).Error; err != nil {
		t.Fatalf("read clients: %v", err)
	}
	if len(stored) != 1 || stored[0] != "bob" {
		t.Fatalf("stored email = %q, want the trimmed %q — the lookup key must match what is written", stored, "bob")
	}

	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("second SyncInbound must not hit a unique-constraint on the untrimmed row: %v", err)
	}
	var links int64
	if err := db.Model(&model.ClientInbound{}).Where("inbound_id = ?", ib.Id).Count(&links).Error; err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 1 {
		t.Fatalf("links after re-sync = %d, want 1", links)
	}
}
