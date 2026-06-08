package service

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// setupClientIpTestDB spins up a throwaway SQLite database (migrations + seeders)
// for a single test, mirroring the harness used by the other service tests.
func setupClientIpTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func marshalIps(t *testing.T, entries ...clientIpEntry) string {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal ips: %v", err)
	}
	return string(b)
}

// readClientIps returns the stored IP entries for an email as a map[ip]timestamp,
// plus whether the row exists at all.
func readClientIps(t *testing.T, email string) (map[string]int64, bool) {
	t.Helper()
	var row model.InboundClientIps
	err := database.GetDB().Where("client_email = ?", email).First(&row).Error
	if database.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("read client ips for %s: %v", email, err)
	}
	var entries []clientIpEntry
	if row.Ips != "" {
		if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
			t.Fatalf("unmarshal stored ips for %s: %v", email, err)
		}
	}
	out := make(map[string]int64, len(entries))
	for _, e := range entries {
		out[e.IP] = e.Timestamp
	}
	return out, true
}

func TestMergeInboundClientIps_CreatesNodeOnlyRowIgnoringRemoteId(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()

	// Local client occupies id 1.
	local := &model.InboundClientIps{ClientEmail: "local@x", Ips: marshalIps(t, clientIpEntry{IP: "1.1.1.1", Timestamp: now})}
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local row: %v", err)
	}

	// Incoming node-only client carries the remote node's id 1, which must not
	// collide with the local row.
	incoming := []model.InboundClientIps{{
		Id:          1,
		ClientEmail: "node@x",
		Ips:         marshalIps(t, clientIpEntry{IP: "2.2.2.2", Timestamp: now}),
	}}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Local row is untouched.
	if ips, ok := readClientIps(t, "local@x"); !ok || ips["1.1.1.1"] != now {
		t.Fatalf("local@x changed unexpectedly: %v (exists=%v)", ips, ok)
	}

	// Node row exists with its own ip and a freshly assigned id (not the remote 1).
	var nodeRow model.InboundClientIps
	if err := db.Where("client_email = ?", "node@x").First(&nodeRow).Error; err != nil {
		t.Fatalf("node@x not created: %v", err)
	}
	if nodeRow.Id == local.Id {
		t.Fatalf("node@x reused local id %d instead of a fresh one", nodeRow.Id)
	}
	if ips, _ := readClientIps(t, "node@x"); ips["2.2.2.2"] != now {
		t.Fatalf("node@x missing expected ip: %v", ips)
	}
}

func TestMergeInboundClientIps_DedupKeepsMaxTimestamp(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()

	if err := db.Create(&model.InboundClientIps{
		ClientEmail: "a@x",
		Ips:         marshalIps(t, clientIpEntry{IP: "1.1.1.1", Timestamp: now - 100}),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := []model.InboundClientIps{{
		ClientEmail: "a@x",
		Ips: marshalIps(t,
			clientIpEntry{IP: "1.1.1.1", Timestamp: now - 50}, // newer than stored -> wins
			clientIpEntry{IP: "2.2.2.2", Timestamp: now - 10},
		),
	}}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}

	ips, _ := readClientIps(t, "a@x")
	if len(ips) != 2 {
		t.Fatalf("want 2 ips, got %v", ips)
	}
	if ips["1.1.1.1"] != now-50 {
		t.Fatalf("1.1.1.1 should keep max timestamp %d, got %d", now-50, ips["1.1.1.1"])
	}
	if ips["2.2.2.2"] != now-10 {
		t.Fatalf("2.2.2.2 missing/incorrect: %d", ips["2.2.2.2"])
	}
}

func TestMergeInboundClientIps_DropsStaleIps(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()

	if err := db.Create(&model.InboundClientIps{
		ClientEmail: "a@x",
		Ips: marshalIps(t,
			clientIpEntry{IP: "old", Timestamp: now - 3600}, // > 30m -> stale
			clientIpEntry{IP: "fresh", Timestamp: now - 60},
		),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := []model.InboundClientIps{{
		ClientEmail: "a@x",
		Ips: marshalIps(t,
			clientIpEntry{IP: "incStale", Timestamp: now - 4000}, // > 30m -> stale
			clientIpEntry{IP: "incFresh", Timestamp: now - 10},
		),
	}}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}

	ips, _ := readClientIps(t, "a@x")
	if len(ips) != 2 {
		t.Fatalf("want only fresh ips, got %v", ips)
	}
	if _, ok := ips["old"]; ok {
		t.Fatalf("stale local ip not dropped: %v", ips)
	}
	if _, ok := ips["incStale"]; ok {
		t.Fatalf("stale incoming ip not dropped: %v", ips)
	}
	if ips["fresh"] != now-60 || ips["incFresh"] != now-10 {
		t.Fatalf("fresh ips wrong: %v", ips)
	}
}

func TestMergeInboundClientIps_SkipsAllStaleCreate(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()

	incoming := []model.InboundClientIps{{
		ClientEmail: "b@x",
		Ips:         marshalIps(t, clientIpEntry{IP: "1.1.1.1", Timestamp: now - 9999}),
	}}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if _, ok := readClientIps(t, "b@x"); ok {
		t.Fatalf("all-stale node-only client should not create a row")
	}
}

func TestMergeInboundClientIps_SkipsBlankRows(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()

	incoming := []model.InboundClientIps{
		{ClientEmail: "", Ips: marshalIps(t, clientIpEntry{IP: "1.1.1.1", Timestamp: now})},
		{ClientEmail: "c@x", Ips: ""},
	}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.InboundClientIps{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("blank rows should be skipped, but %d row(s) created", count)
	}
}
