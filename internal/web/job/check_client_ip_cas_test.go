package job

import (
	"maps"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// A node sync landing between the scan's read and its write must keep its remote
// IP (#6587); the hook injects that write, since SQLite serializes real writers.
func TestProcessObserved_KeepsNodeSyncWriteThatLandsMidScan(t *testing.T) {
	setupIntegrationDB(t)
	processObservedKeepsNodeSyncWrite(t)
}

func processObservedKeepsNodeSyncWrite(t *testing.T) {
	t.Helper()
	db := database.GetDB()

	const email = "cas-scan@x"
	seedLinkedInboundWithClient(t, "cas-scan", email, 3)
	now := time.Now().Unix()
	seed := seedClientIps(t, email, []model.ClientIpEntry{{IP: "198.51.100.1", Timestamp: now - 60}})

	nodeBlob, err := model.MarshalClientIpEntries([]model.ClientIpEntry{
		{IP: "198.51.100.1", Timestamp: now - 60},
		{IP: "203.0.113.77", Timestamp: now - 5},
	})
	if err != nil {
		t.Fatalf("marshal node blob: %v", err)
	}
	const callback = "test:client_ip_scan_cas_inject"
	injected := false
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if injected || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "inbound_client_ips" {
			return
		}
		where, ok := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		if !ok || len(where.Exprs) != 1 {
			return
		}
		predicate, ok := where.Exprs[0].(clause.Expr)
		if !ok || len(predicate.Vars) != 2 || predicate.Vars[0] != seed.Id {
			return
		}
		injected = true
		if err := tx.Session(&gorm.Session{SkipHooks: true, NewDB: true}).
			Model(&model.InboundClientIps{}).
			Where("client_email = ?", email).
			Update("ips", nodeBlob).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })

	NewCheckClientIpJob().processObserved(map[string]map[string]int64{
		email: {"198.51.100.1": now},
	}, true, true)
	if !injected {
		t.Fatal("inject callback never fired; the scan's write path is untested")
	}

	got := ipSet(readClientIps(t, email))
	want := map[string]int64{"203.0.113.77": now - 5, "198.51.100.1": now}
	if !maps.Equal(got, want) {
		t.Fatalf("scan/node union = %v, want %v", got, want)
	}
}
