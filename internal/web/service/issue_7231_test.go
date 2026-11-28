package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSyncOrphanRecoveryScope(t *testing.T) {
	testSyncOrphanRecoveryScope(t, initTrafficTestDB(t))
}

func TestSyncOrphanRecoveryScopePostgres(t *testing.T) {
	testSyncOrphanRecoveryScope(t, durablePostgresDB(t))
}

func TestLegacySyncOrphanMarksAcrossLinkChanges(t *testing.T) {
	testLegacySyncOrphanMarksAcrossLinkChanges(t, initTrafficTestDB(t))
}

func TestLegacySyncOrphanMarksAcrossLinkChangesPostgres(t *testing.T) {
	cleanup, err := testpg.IsolatePackage("legacy_sync_orphan_marks")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	testLegacySyncOrphanMarksAcrossLinkChanges(t, durablePostgresDB(t))
}

func testLegacySyncOrphanMarksAcrossLinkChanges(t *testing.T, db *gorm.DB) {
	t.Helper()
	clientSvc, inboundSvc := &ClientService{}, &InboundService{}
	for _, action := range []string{"detach client", "delete inbound", "reattach client"} {
		t.Run(action, func(t *testing.T) {
			client := model.Client{Email: fmt.Sprintf("legacy-mark-%d@x", time.Now().UnixNano())}
			ib := durableTestInbound(nil, client.Email, 42534)
			ib.Enable = false
			if action != "reattach client" {
				ib.Settings = clientsSettings(t, []model.Client{client})
			}
			if err := db.Create(ib).Error; err != nil {
				t.Fatal(err)
			}
			if action == "reattach client" {
				record := model.ClientRecord{Email: client.Email}
				if err := db.Create(&record).Error; err != nil {
					t.Fatal(err)
				}
			} else if err := clientSvc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
				t.Fatal(err)
			}
			backdateOrphanMark(t, db, client.Email)

			switch action {
			case "detach client":
				if _, err := clientSvc.DetachByEmail(inboundSvc, ib.Id, client.Email); err != nil {
					t.Fatal(err)
				}
			case "delete inbound":
				if _, err := inboundSvc.DelInbound(ib.Id); err != nil {
					t.Fatal(err)
				}
			case "reattach client":
				writes := 0
				const callback = "test:reattach-mark-writes"
				if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
					sql := strings.NewReplacer("`", "", "\"", "").Replace(tx.Statement.SQL.String())
					if strings.Contains(sql, "SET sync_orphaned_at=") {
						writes++
					}
				}); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
				if err := clientSvc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
					t.Fatal(err)
				}
				if writes != 1 {
					t.Errorf("reattachment issued %d mark clears, want 1", writes)
				}
			}

			if reaped, err := clientSvc.ReapSyncOrphans(); err != nil || reaped != 0 {
				t.Fatalf("legacy mark after %s: reaped=%d err=%v", action, reaped, err)
			}
			if got := readOrphanMark(t, db, client.Email); got != 0 {
				t.Fatalf("legacy mark after %s was %d, want zero", action, got)
			}
		})
	}
}

func testSyncOrphanRecoveryScope(t *testing.T, db *gorm.DB) {
	t.Helper()
	suffix := time.Now().UnixNano()
	tag, unrelatedTag := fmt.Sprintf("scope-node-%d", suffix), fmt.Sprintf("scope-local-%d", suffix)
	email, otherEmail := tag+"@x", unrelatedTag+"@x"
	node := &model.Node{Name: tag, Address: "127.0.0.1", Port: 2096, Enable: true}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	createNodeInboundWithClient(t, db, node.Id, tag, 42531, email)
	createNodeInboundWithClient(t, db, node.Id+10000, unrelatedTag, 42532, otherEmail)
	backdateOrphanMark(t, db, email)
	backdateOrphanMark(t, db, otherEmail)
	writes := 0
	const callback = "test:orphan-scope"
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "sync_orphaned_by") && strings.Contains(tx.Statement.SQL.String(), "EXISTS") {
			writes++
			if !strings.Contains(tx.Statement.SQL.String(), "id IN") {
				t.Error("unbounded orphan recovery update")
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })

	service := &InboundService{}
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"enable":true}]}`, email)
	syncNodeWithSettings(t, service, node.Id, tag, settings, xray.ClientTraffic{Email: email, Enable: true})
	if readOrphanMark(t, db, email) != 0 {
		t.Fatal("touched client still marked")
	}
	if readOrphanMark(t, db, otherEmail) <= 0 {
		t.Fatal("merge repaired an unrelated mark")
	}
	writes = 0
	syncNodeWithSettings(t, service, node.Id, tag, settings, xray.ClientTraffic{Email: email, Up: 1, Enable: true})
	if writes != 0 {
		t.Fatalf("unchanged snapshot issued %d orphan updates", writes)
	}
	if _, err := (&ClientService{}).ReapSyncOrphans(); err != nil {
		t.Fatal(err)
	}
	if readOrphanMark(t, db, otherEmail) != 0 {
		t.Fatal("maintenance failed to repair attached legacy mark")
	}
}
