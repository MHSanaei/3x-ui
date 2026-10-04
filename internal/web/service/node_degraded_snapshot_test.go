package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// linkCount returns how many client_inbounds links a client currently has,
// across every inbound — the value ReapSyncOrphans checks before deleting.
func linkCount(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var n int64
	if err := db.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ?", email).
		Count(&n).Error; err != nil {
		t.Fatalf("count links for %q: %v", email, err)
	}
	return n
}

func orphanMark(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var at int64
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).
		Pluck("sync_orphaned_at", &at).Error; err != nil {
		t.Fatalf("read sync_orphaned_at %q: %v", email, err)
	}
	return at
}

// TestSetRemoteTraffic_EmptySnapshotKeepsClients is the core guard: a managed
// node that comes back reporting zero clients for an inbound the hub still has
// clients on is treated as degraded (just deleted/reset/restarted, or a
// snapshot taken before the config loaded), not as "every client was removed".
// Its links must stay and nothing may be orphan-marked — otherwise SyncInbound
// would strip every link and ReapSyncOrphans would later hard-delete the row.
func TestSetRemoteTraffic_EmptySnapshotKeepsClients(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "svc@x")

	settings := `{"clients":[{"email":"svc@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", settings,
		xray.ClientTraffic{Email: "svc@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if n := linkCount(t, db, "svc@x"); n != 1 {
		t.Fatalf("setup: svc@x links=%d, want 1", n)
	}

	// The node returns an empty snapshot — the trigger that deleted real clients.
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false, false); err != nil {
		t.Fatalf("empty-snapshot sync: %v", err)
	}

	if rec, _ := countClientRows(t, db, "svc@x"); rec != 1 {
		t.Fatalf("empty snapshot deleted the client row: clients=%d, want 1", rec)
	}
	if n := linkCount(t, db, "svc@x"); n != 1 {
		t.Fatalf("empty snapshot stripped the client link: links=%d, want 1", n)
	}
	if at := orphanMark(t, db, "svc@x"); at != 0 {
		t.Fatalf("empty snapshot orphan-marked a live client: sync_orphaned_at=%d, want 0", at)
	}
}

// TestSetRemoteTraffic_EmptySnapshotSurvivesReap closes the loop on the outage
// of 2026-10-04: without the guard an empty/degraded snapshot orphan-marks the
// inbound's clients and ReapSyncOrphans hard-deletes them after the grace
// period. The guard keeps them attached, so even a backdated reap can't take
// them — which is exactly what deleting one node must not do to the clients it
// served.
func TestSetRemoteTraffic_EmptySnapshotSurvivesReap(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	clientSvc := &ClientService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "svc@x")

	settings := `{"clients":[{"email":"svc@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", settings,
		xray.ClientTraffic{Email: "svc@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false, false); err != nil {
		t.Fatalf("empty-snapshot sync: %v", err)
	}

	backdateOrphanMark(t, db, "svc@x") // no-op if unmarked; proves reap can't take it
	if reaped, err := clientSvc.ReapSyncOrphans(); err != nil {
		t.Fatalf("reap: %v", err)
	} else if reaped != 0 {
		t.Fatalf("reaped %d client(s) off an empty snapshot, want 0", reaped)
	}
	if rec, _ := countClientRows(t, db, "svc@x"); rec != 1 {
		t.Fatalf("empty snapshot + reap deleted the client: clients=%d, want 1", rec)
	}
}

// TestSetRemoteTraffic_PartialSnapshotStillPrunes confirms the guard is narrow:
// a snapshot that still carries at least one client is authoritative, so a
// client the node really dropped is still unlinked and left for the orphan
// sweep. Only the all-empty snapshot is treated as degraded.
func TestSetRemoteTraffic_PartialSnapshotStillPrunes(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, "keep@x")

	bothSettings := `{"clients":[{"email":"keep@x","enable":true},{"email":"drop@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", bothSettings,
		xray.ClientTraffic{Email: "keep@x", Enable: true},
		xray.ClientTraffic{Email: "drop@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	if n := linkCount(t, db, "drop@x"); n != 1 {
		t.Fatalf("setup: drop@x links=%d, want 1", n)
	}

	// Node now reports only keep@x — drop@x was genuinely removed there.
	keepOnlySettings := `{"clients":[{"email":"keep@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "n1-in", keepOnlySettings,
		xray.ClientTraffic{Email: "keep@x", Enable: true}), false, false); err != nil {
		t.Fatalf("partial-snapshot sync: %v", err)
	}

	if n := linkCount(t, db, "keep@x"); n != 1 {
		t.Fatalf("partial snapshot dropped a reported client: keep@x links=%d, want 1", n)
	}
	if n := linkCount(t, db, "drop@x"); n != 0 {
		t.Fatalf("partial snapshot kept an unreported client linked: drop@x links=%d, want 0", n)
	}
	if at := orphanMark(t, db, "drop@x"); at <= 0 {
		t.Fatalf("partial snapshot did not orphan-mark the removed client: sync_orphaned_at=%d, want >0", at)
	}
}
