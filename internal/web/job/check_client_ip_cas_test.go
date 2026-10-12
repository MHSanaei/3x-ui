package job

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// A node sync landing between the scan's read and its write must keep its remote
// IP (#6587); the hook injects that write, since SQLite serializes real writers.
func TestProcessObserved_KeepsNodeSyncWriteThatLandsMidScan(t *testing.T) {
	setupIntegrationDB(t)
	db := database.GetDB()

	const email = "cas-scan@x"
	seedLinkedInboundWithClient(t, "cas-scan", email, 3)
	now := time.Now().Unix()
	seedClientIps(t, email, []IPWithTimestamp{{IP: "198.51.100.1", Timestamp: now - 60}})

	nodeBlob, err := json.Marshal([]IPWithTimestamp{
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
		injected = true
		if err := tx.Session(&gorm.Session{SkipHooks: true, NewDB: true}).
			Model(&model.InboundClientIps{}).
			Where("client_email = ?", email).
			Update("ips", string(nodeBlob)).Error; err != nil {
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
	if _, ok := got["203.0.113.77"]; !ok {
		t.Fatalf("scan overwrote the node's remote IP (the #6587 lost update): %v", got)
	}
	if got["198.51.100.1"] != now {
		t.Fatalf("scan's own observation missing or stale: %v", got)
	}
}
