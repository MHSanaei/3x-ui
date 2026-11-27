package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSyncOrphanRecoveryScope(t *testing.T) {
	testSyncOrphanRecoveryScope(t, initTrafficTestDB(t))
}

func TestSyncOrphanRecoveryScopePostgres(t *testing.T) {
	testSyncOrphanRecoveryScope(t, durablePostgresDB(t))
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
