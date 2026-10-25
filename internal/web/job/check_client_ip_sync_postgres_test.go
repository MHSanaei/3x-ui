package job

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestClientIpWritersPreserveCanonicalBytes(t *testing.T) {
	setupIntegrationDB(t)
	clientIpWritersPreserveCanonicalBytes(t)
}

func clientIpWritersPreserveCanonicalBytes(t *testing.T) {
	t.Helper()
	const email = "canonical-writers@x"
	seedLinkedInboundWithClient(t, "canonical-writers", email, 3)
	now := time.Now().Unix()
	entries := []model.ClientIpEntry{
		{IP: "192.0.2.9", Timestamp: now},
		{IP: "10.0.0.1", Timestamp: now},
		{IP: "203.0.113.8", Timestamp: now - 300},
		{IP: "192.0.2.1", Timestamp: now},
	}
	want := fmt.Sprintf(`[{"ip":"203.0.113.8","timestamp":%d},{"ip":"10.0.0.1","timestamp":%d},{"ip":"192.0.2.1","timestamp":%d},{"ip":"192.0.2.9","timestamp":%d}]`, now-300, now, now, now)
	row := model.InboundClientIps{ClientEmail: email, Ips: want}
	db := database.GetDB()
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	flatWrites, attributionWrites := 0, 0
	const callback = "test:canonical_writer_updates"
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.RowsAffected == 0 {
			return
		}
		if tx.Statement.Schema.Table == "inbound_client_ips" {
			flatWrites++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	if err := db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "node_client_ips" && tx.RowsAffected > 0 {
			attributionWrites++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	j := NewCheckClientIpJob()
	j.allowlist = parseIpLimitAllowlist("10.0.0.1")
	observed := map[string]map[string]int64{email: {"192.0.2.9": now, "10.0.0.1": now, "192.0.2.1": now}}
	for iteration := range 4 {
		blob, err := json.Marshal(entries)
		if err != nil {
			t.Fatal(err)
		}
		if err := (&service.InboundService{}).MergeInboundClientIps([]model.InboundClientIps{{ClientEmail: email, Ips: string(blob)}}); err != nil {
			t.Fatal(err)
		}
		j.processObserved(observed, true, false)
		var got model.InboundClientIps
		if err := db.Where("client_email = ?", email).First(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.Ips != want {
			t.Fatalf("iteration %d canonical bytes = %s, want %s", iteration, got.Ips, want)
		}
	}
	if flatWrites != 0 {
		t.Fatalf("unchanged alternating writers performed %d flat updates, want 0", flatWrites)
	}
	beforeAttribution := attributionWrites
	j.processObserved(map[string]map[string]int64{email: {"192.0.2.9": now + 1}}, true, false)
	if flatWrites != 1 {
		t.Fatalf("advancing observation flat updates = %d, want 1", flatWrites)
	}
	advancedWant := fmt.Sprintf(`[{"ip":"203.0.113.8","timestamp":%d},{"ip":"10.0.0.1","timestamp":%d},{"ip":"192.0.2.1","timestamp":%d},{"ip":"192.0.2.9","timestamp":%d}]`, now-300, now, now, now+1)
	var advanced model.InboundClientIps
	if err := db.Where("client_email = ?", email).First(&advanced).Error; err != nil {
		t.Fatal(err)
	}
	if advanced.Ips != advancedWant {
		t.Fatalf("advancing flat bytes = %s, want %s", advanced.Ips, advancedWant)
	}
	if attributionWrites != beforeAttribution+1 {
		t.Fatalf("advancing attribution writes = %d, want %d", attributionWrites, beforeAttribution+1)
	}
	var attribution model.NodeClientIp
	if err := db.Where("email = ?", email).First(&attribution).Error; err != nil {
		t.Fatal(err)
	}
	var attributed []model.ClientIpEntry
	if err := json.Unmarshal([]byte(attribution.Ips), &attributed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range attributed {
		if entry.IP == "192.0.2.9" {
			found = entry.Timestamp == now+1
		}
	}
	if !found {
		t.Fatalf("advancing attribution = %v, want 192.0.2.9 at %d", attributed, now+1)
	}
}

func TestProcessObserved_CreateConflictPreservesUnion(t *testing.T) {
	setupIntegrationDB(t)
	processObservedCreateConflictPreservesUnion(t)
}

func processObservedCreateConflictPreservesUnion(t *testing.T) {
	t.Helper()
	const email = "create-conflict@x"
	seedLinkedInboundWithClient(t, "create-conflict", email, 3)
	now := time.Now().Unix()
	winning := fmt.Sprintf(`[{"ip":"192.0.2.1","timestamp":%d},{"ip":"203.0.113.8","timestamp":%d}]`, now-30, now)
	db := database.GetDB()
	injected := false
	const callback = "test:scanner_create_conflict"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if injected || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "inbound_client_ips" {
			return
		}
		injected = true
		if err := tx.Session(&gorm.Session{NewDB: true, SkipHooks: true}).Exec("INSERT INTO inbound_client_ips (client_email, ips) VALUES (?, ?)", email, winning).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	NewCheckClientIpJob().processObserved(map[string]map[string]int64{email: {"192.0.2.1": now, "198.51.100.2": now - 5}}, true, true)
	if !injected {
		t.Fatal("missing-row insert was not exercised")
	}
	var row model.InboundClientIps
	if err := db.Where("client_email = ?", email).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`[{"ip":"198.51.100.2","timestamp":%d},{"ip":"192.0.2.1","timestamp":%d},{"ip":"203.0.113.8","timestamp":%d}]`, now-5, now, now)
	if row.Ips != want {
		t.Fatalf("conflict union bytes = %s, want %s", row.Ips, want)
	}
}

func TestClientIpSyncPostgresScanner(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires PostgreSQL")
	}
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"CanonicalWriters", clientIpWritersPreserveCanonicalBytes},
		{"CreateConflict", processObservedCreateConflictPreservesUnion},
		{"ConcurrentWriter", processObservedKeepsNodeSyncWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup, err := testpg.IsolatePackage("job_ip_sync")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cleanup)
			t.Setenv("XUI_LOG_FOLDER", t.TempDir())
			loggerInitOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
			t.Cleanup(func() { _ = database.CloseDB() })
			if err := database.InitDB(""); err != nil {
				t.Fatal(err)
			}
			tc.run(t)
		})
	}
}
