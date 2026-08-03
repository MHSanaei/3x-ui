package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func readOrphanMark(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read client %q: %v", email, err)
	}
	return row.SyncOrphanedAt
}

func backdateOrphanMark(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	past := time.Now().Add(-2 * syncOrphanReapGrace).UnixMilli()
	if err := db.Model(&model.ClientRecord{}).
		Where("email = ?", email).
		Update("sync_orphaned_at", past).Error; err != nil {
		t.Fatalf("backdate orphan mark: %v", err)
	}
}

// The merge must soft-orphan, not delete: everything stays recoverable until
// the grace period has elapsed and the reaper confirms nothing reclaimed it.
func TestSyncOrphanSurvivesMergeUntilGraceElapses(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	clientSvc := &ClientService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const email = "gone@x"
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, email)
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"enable":true}]}`, email)
	syncNodeWithSettings(t, svc, 1, "n1-in", settings,
		xray.ClientTraffic{Email: email, Up: 5, Down: 5, Enable: true})

	if rec, traf := countClientRows(t, db, email); rec != 1 || traf != 1 {
		t.Fatalf("setup: clients=%d client_traffics=%d, want 1/1", rec, traf)
	}

	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false); err != nil {
		t.Fatalf("orphaning merge: %v", err)
	}
	if rec, traf := countClientRows(t, db, email); rec != 1 || traf != 1 {
		t.Fatalf("merge hard-deleted the client: clients=%d client_traffics=%d, want 1/1", rec, traf)
	}
	if readOrphanMark(t, db, email) <= 0 {
		t.Fatal("merge did not stamp sync_orphaned_at")
	}

	reaped, err := clientSvc.ReapSyncOrphans()
	if err != nil {
		t.Fatalf("reap inside grace: %v", err)
	}
	if reaped != 0 {
		t.Fatalf("reaped %d client(s) inside the grace period, want 0", reaped)
	}
	if rec, _ := countClientRows(t, db, email); rec != 1 {
		t.Fatal("client removed before the grace period elapsed")
	}

	backdateOrphanMark(t, db, email)
	reaped, err = clientSvc.ReapSyncOrphans()
	if err != nil {
		t.Fatalf("reap after grace: %v", err)
	}
	if reaped != 1 {
		t.Fatalf("reaped %d client(s) after the grace period, want 1", reaped)
	}
	rec, traf := countClientRows(t, db, email)
	if rec != 0 || traf != 0 {
		t.Fatalf("after reap: clients=%d client_traffics=%d, want 0/0", rec, traf)
	}
}

// A client the node reports again was never gone: clearing the mark is what
// turns a bad merge into a recoverable blip instead of a delayed deletion.
func TestSyncOrphanMarkClearedOnReattach(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	clientSvc := &ClientService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const email = "flaky@x"
	createNodeInboundWithClient(t, db, 1, "n1-in", 41001, email)
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"enable":true}]}`, email)
	syncNodeWithSettings(t, svc, 1, "n1-in", settings,
		xray.ClientTraffic{Email: email, Up: 5, Down: 5, Enable: true})

	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithoutClients(t, "n1-in"), false); err != nil {
		t.Fatalf("orphaning merge: %v", err)
	}
	if readOrphanMark(t, db, email) <= 0 {
		t.Fatal("setup: expected the merge to mark the client")
	}

	syncNodeWithSettings(t, svc, 1, "n1-in", settings,
		xray.ClientTraffic{Email: email, Up: 6, Down: 6, Enable: true})

	if orphanedAt := readOrphanMark(t, db, email); orphanedAt != 0 {
		t.Fatalf("re-attached client kept its orphan mark: sync_orphaned_at=%d", orphanedAt)
	}

	backdateOrphanMark(t, db, email)
	if reaped, err := clientSvc.ReapSyncOrphans(); err != nil || reaped != 0 {
		t.Fatalf("reaped %d client(s) (err=%v) that the node still reports, want 0", reaped, err)
	}
}

// The reaper is scoped to the node sweep. Orphans from any other cause carry no
// mark and keep their existing manual-cleanup semantics.
func TestReapSyncOrphansIgnoresUnmarkedOrphans(t *testing.T) {
	db := initTrafficTestDB(t)
	clientSvc := &ClientService{}

	const email = "manual@x"
	rec := &model.ClientRecord{Email: email, Enable: true, UUID: "44444444-4444-4444-4444-444444444444"}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	reaped, err := clientSvc.ReapSyncOrphans()
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if reaped != 0 {
		t.Fatalf("reaped %d unmarked orphan(s), want 0", reaped)
	}
	var surviving int64
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Count(&surviving).Error; err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if surviving != 1 {
		t.Fatalf("unmarked orphan was reaped: %d rows survive, want 1", surviving)
	}
}
