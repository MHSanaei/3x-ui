package service

import (
	"strings"
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

// A degraded node reporting zero clients for an inbound the hub populates must
// keep its links and never orphan-mark, or SyncInbound/ReapSyncOrphans delete the row.
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
	if at := readOrphanMark(t, db, "svc@x"); at != 0 {
		t.Fatalf("empty snapshot orphan-marked a live client: sync_orphaned_at=%d, want 0", at)
	}
	// The hub must keep the client in the inbound's settings, or reconcile re-pushes
	// an empty blob to the node and the clients never come back (#6734).
	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("read central inbound: %v", err)
	}
	if !strings.Contains(ib.Settings, "svc@x") {
		t.Fatalf("empty snapshot blanked the inbound settings: %q", ib.Settings)
	}
}

// The guard is narrow: a snapshot still carrying a client is authoritative, so a
// client the node really dropped is unlinked and orphan-marked; only all-empty is degraded.
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
	if at := readOrphanMark(t, db, "drop@x"); at <= 0 {
		t.Fatalf("partial snapshot did not orphan-mark the removed client: sync_orphaned_at=%d, want >0", at)
	}
}
