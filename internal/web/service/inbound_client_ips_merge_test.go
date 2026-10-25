package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// setupClientIpTestDB spins up a throwaway SQLite database (migrations + seeders)
// for a single test, mirroring the harness used by the other service tests.
func setupClientIpTestDB(t *testing.T) {
	t.Helper()
	cleanup, err := testpg.IsolatePackage("client_ip_sync")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
}

func marshalIps(t *testing.T, entries ...model.ClientIpEntry) string {
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
	var entries []model.ClientIpEntry
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
	local := &model.InboundClientIps{ClientEmail: "local@x", Ips: marshalIps(t, model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now})}
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local row: %v", err)
	}

	// Incoming node-only client carries the remote node's id 1, which must not
	// collide with the local row.
	incoming := []model.InboundClientIps{{
		Id:          1,
		ClientEmail: "node@x",
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "2.2.2.2", Timestamp: now}),
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
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now - 100}),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := []model.InboundClientIps{{
		ClientEmail: "a@x",
		Ips: marshalIps(t,
			model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now - 50}, // newer than stored -> wins
			model.ClientIpEntry{IP: "2.2.2.2", Timestamp: now - 10},
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
			model.ClientIpEntry{IP: "old", Timestamp: now - 3600}, // > 30m -> stale
			model.ClientIpEntry{IP: "fresh", Timestamp: now - 60},
		),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := []model.InboundClientIps{{
		ClientEmail: "a@x",
		Ips: marshalIps(t,
			model.ClientIpEntry{IP: "incStale", Timestamp: now - 4000}, // > 30m -> stale
			model.ClientIpEntry{IP: "incFresh", Timestamp: now - 10},
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
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now - 9999}),
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
		{ClientEmail: "", Ips: marshalIps(t, model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now})},
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

func TestCasUpdateInboundClientIps_MatchAndMismatch(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()

	seed := &model.InboundClientIps{
		ClientEmail: "cas@x",
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "1.1.1.1", Timestamp: now}),
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	next := marshalIps(t, model.ClientIpEntry{IP: "2.2.2.2", Timestamp: now})
	ok, err := CasUpdateInboundClientIps(db, seed.Id, "not-the-blob", next)
	if err != nil {
		t.Fatalf("stale CAS: %v", err)
	}
	if ok {
		t.Fatalf("CAS with wrong expected must not update")
	}
	ips, _ := readClientIps(t, "cas@x")
	if ips["1.1.1.1"] != now || len(ips) != 1 {
		t.Fatalf("row changed on stale CAS: %v", ips)
	}

	ok, err = CasUpdateInboundClientIps(db, seed.Id, seed.Ips, next)
	if err != nil {
		t.Fatalf("fresh CAS: %v", err)
	}
	if !ok {
		t.Fatalf("CAS with matching expected must update")
	}
	ips, _ = readClientIps(t, "cas@x")
	if ips["2.2.2.2"] != now || len(ips) != 1 {
		t.Fatalf("fresh CAS did not land: %v", ips)
	}
}

// A job write landing between the merge's read and its Update must not drop the
// node's report (#6587); a Before(update) hook injects it, since SQLite serializes writers.
func TestMergeInboundClientIps_RetriesAfterConcurrentWriter(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()

	seed := &model.InboundClientIps{
		ClientEmail: "race@x",
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "10.0.0.1", Timestamp: now - 30}),
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	jobBlob := marshalIps(t, model.ClientIpEntry{IP: "10.0.0.2", Timestamp: now - 10})
	const callback = "test:inbound_client_ips_cas_inject"
	injected := false
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if injected {
			return
		}
		table := tx.Statement.Table
		if table == "" && tx.Statement.Schema != nil {
			table = tx.Statement.Schema.Table
		}
		if table != "inbound_client_ips" {
			return
		}
		injected = true
		// Same connection, SkipHooks: simulate the job committing a different
		// blob before this merge's CAS Update runs.
		if err := tx.Session(&gorm.Session{SkipHooks: true}).
			Model(&model.InboundClientIps{}).
			Where("id = ?", seed.Id).
			Update("ips", jobBlob).Error; err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })

	incoming := []model.InboundClientIps{{
		ClientEmail: "race@x",
		Ips:         marshalIps(t, model.ClientIpEntry{IP: "10.0.0.3", Timestamp: now}),
	}}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !injected {
		t.Fatalf("inject callback never fired; CAS path untested")
	}

	ips, _ := readClientIps(t, "race@x")
	// After the injected job write (only .2) and the node's .3 report, both
	// must survive. .1 was only in the pre-job snapshot and is correctly gone.
	if _, ok := ips["10.0.0.2"]; !ok {
		t.Fatalf("job IP lost after merge retry: %v", ips)
	}
	if _, ok := ips["10.0.0.3"]; !ok {
		t.Fatalf("node IP lost (the #6587 failure mode): %v", ips)
	}
	if _, ok := ips["10.0.0.1"]; ok {
		t.Fatalf("pre-job IP should not resurrect after job replaced the blob: %v", ips)
	}
}

type clientIpSQLCounts struct {
	selects  atomic.Int64
	updates  atomic.Int64
	creates  atomic.Int64
	affected atomic.Int64
}

func countClientIpSQL(t *testing.T, table string) *clientIpSQLCounts {
	t.Helper()
	db := database.GetDB()
	counts := &clientIpSQLCounts{}
	name := "test:ip_sql_" + table
	query := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			counts.selects.Add(1)
		}
	}
	update := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			counts.updates.Add(1)
			counts.affected.Add(tx.RowsAffected)
		}
	}
	create := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			counts.creates.Add(1)
			counts.affected.Add(tx.RowsAffected)
		}
	}
	if err := db.Callback().Query().After("gorm:query").Register(name, query); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().After("gorm:update").Register(name, update); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Create().After("gorm:create").Register(name, create); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Update().Remove(name)
		_ = db.Callback().Create().Remove(name)
	})
	return counts
}

func rawClientIpBlob(t *testing.T, entries ...model.ClientIpEntry) string {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func requireClientIpSet(t *testing.T, email string, want map[string]int64) {
	t.Helper()
	got, exists := readClientIps(t, email)
	if !exists || !reflect.DeepEqual(got, want) {
		t.Fatalf("%s IPs = %v (exists=%v), want %v", email, got, exists, want)
	}
}

func TestClientIpSyncSeededSnapshot(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed=%v", changed), func(t *testing.T) {
			setupClientIpTestDB(t)
			now := time.Now().Unix()
			rows := make([]model.InboundClientIps, 3)
			incoming := make([]model.InboundClientIps, 3)
			for i := range rows {
				rows[i] = model.InboundClientIps{ClientEmail: fmt.Sprintf("snapshot-%d@x", i), Ips: rawClientIpBlob(t,
					model.ClientIpEntry{IP: "192.0.2.1", Timestamp: now - 10}, model.ClientIpEntry{IP: "192.0.2.2", Timestamp: now})}
				incoming[i] = rows[i]
				if changed {
					incoming[i].Ips = rawClientIpBlob(t, model.ClientIpEntry{IP: "192.0.2.2", Timestamp: now + 1})
				}
			}
			if err := database.GetDB().Create(&rows).Error; err != nil {
				t.Fatal(err)
			}
			counts := countClientIpSQL(t, "inbound_client_ips")
			if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
				t.Fatal(err)
			}
			wantUpdates := int64(0)
			if changed {
				wantUpdates = 3
			}
			if counts.selects.Load() != 1 || counts.updates.Load() != wantUpdates {
				t.Fatalf("SELECTs/CAS attempts = %d/%d, want 1/%d", counts.selects.Load(), counts.updates.Load(), wantUpdates)
			}
			for _, row := range rows {
				ts := now
				if changed {
					ts++
				}
				requireClientIpSet(t, row.ClientEmail, map[string]int64{"192.0.2.1": now - 10, "192.0.2.2": ts})
			}
		})
	}
}

func TestClientIpSyncDuplicatePayloadsKeepUnion(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	seed := model.InboundClientIps{ClientEmail: "existing@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "192.0.2.1", Timestamp: now})}
	if err := database.GetDB().Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	var incoming []model.InboundClientIps
	for _, email := range []string{"new@x", "existing@x"} {
		for i := range 3 {
			incoming = append(incoming, model.InboundClientIps{Id: 99, ClientEmail: email, Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: fmt.Sprintf("192.0.2.%d", i+1), Timestamp: now + int64(i)})})
		}
	}
	if err := (&InboundService{}).MergeInboundClientIps(incoming); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"new@x", "existing@x"} {
		requireClientIpSet(t, email, map[string]int64{"192.0.2.1": now, "192.0.2.2": now + 1, "192.0.2.3": now + 2})
	}
}

func TestClientIpSyncCreateConflictMergesWinner(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()
	winner := rawClientIpBlob(t, model.ClientIpEntry{IP: "198.51.100.1", Timestamp: now})
	const callback = "test:ip_create_winner"
	injected := false
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		row, ok := tx.Statement.Dest.(*model.InboundClientIps)
		if injected || tx.Statement.Table != "inbound_client_ips" || !ok || row.ClientEmail != "winner@x" {
			return
		}
		injected = true
		if err := tx.Exec("INSERT INTO inbound_client_ips (client_email, ips) VALUES (?, ?)", row.ClientEmail, winner).Error; err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	if err := (&InboundService{}).MergeInboundClientIps([]model.InboundClientIps{{ClientEmail: "winner@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "203.0.113.1", Timestamp: now + 1})}}); err != nil {
		t.Fatal(err)
	}
	if !injected {
		t.Fatal("create conflict was not injected")
	}
	requireClientIpSet(t, "winner@x", map[string]int64{"198.51.100.1": now, "203.0.113.1": now + 1})
}

func TestClientIpSyncCASExhaustionRollsBack(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()
	now := time.Now().Unix()
	rows := []model.InboundClientIps{
		{ClientEmail: "a@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "192.0.2.1", Timestamp: now})},
		{ClientEmail: "z@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "192.0.2.2", Timestamp: now})},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	counts := countClientIpSQL(t, "inbound_client_ips")
	const callback = "test:ip_exhaustion"
	attempts := 0
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "inbound_client_ips" {
			return
		}
		where, ok := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		if !ok || len(where.Exprs) != 1 {
			return
		}
		predicate, ok := where.Exprs[0].(clause.Expr)
		if !ok || len(predicate.Vars) != 2 || predicate.Vars[0] != rows[1].Id {
			return
		}
		attempts++
		blob := rawClientIpBlob(t, model.ClientIpEntry{IP: "198.51.100.9", Timestamp: now + int64(attempts)})
		if err := tx.Exec("UPDATE inbound_client_ips SET ips = ? WHERE id = ?", blob, rows[1].Id).Error; err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	err := (&InboundService{}).MergeInboundClientIps([]model.InboundClientIps{
		{ClientEmail: "z@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "203.0.113.8", Timestamp: now})},
		{ClientEmail: "a@x", Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "203.0.113.7", Timestamp: now})},
	})
	want := fmt.Sprintf("inbound_client_ips id=%d: exhausted CAS retries merging client IPs", rows[1].Id)
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if attempts != 8 || counts.selects.Load() != 8 || counts.updates.Load() != 9 {
		t.Fatalf("attempts/selects/updates = %d/%d/%d, want 8/8/9", attempts, counts.selects.Load(), counts.updates.Load())
	}
	for _, row := range rows {
		var got model.InboundClientIps
		if err := db.First(&got, row.Id).Error; err != nil {
			t.Fatal(err)
		}
		if got.Ips != row.Ips {
			t.Fatalf("rollback changed %s: %s, want %s", row.ClientEmail, got.Ips, row.Ips)
		}
	}
}

func TestNodeClientIpCanonicalUnchangedSkipsUpsert(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	report := map[string][]model.ClientIpEntry{"u@x": {{IP: "192.0.2.1", Timestamp: now - 1}, {IP: "192.0.2.2", Timestamp: now}}}
	row := model.NodeClientIp{NodeGuid: "guid", Email: "u@x", Ips: rawClientIpBlob(t, report["u@x"]...)}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	counts := countClientIpSQL(t, "node_client_ips")
	if err := (&InboundService{}).RecordLocalClientIps("guid", report); err != nil {
		t.Fatal(err)
	}
	if counts.selects.Load() != 1 || counts.creates.Load() != 0 || counts.updates.Load() != 0 {
		t.Fatalf("unchanged attribution SELECT/create/update = %d/%d/%d, want 1/0/0", counts.selects.Load(), counts.creates.Load(), counts.updates.Load())
	}
	report["u@x"][1].Timestamp++
	if err := (&InboundService{}).RecordLocalClientIps("guid", report); err != nil {
		t.Fatal(err)
	}
	if counts.creates.Load() != 1 || counts.affected.Load() != 1 {
		t.Fatalf("advancing attribution attempts/affected = %d/%d, want 1/1", counts.creates.Load(), counts.affected.Load())
	}
	trees, err := (&InboundService{}).GetClientIpsByGuid()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(trees["guid"]["u@x"], report["u@x"]) {
		t.Fatalf("attribution = %v, want %v", trees, report)
	}
}

func TestClientIpSyncRawSnapshotAndSingleRetry(t *testing.T) {
	for _, contend := range []bool{false, true} {
		t.Run(fmt.Sprintf("contention=%v", contend), func(t *testing.T) {
			setupClientIpTestDB(t)
			db := database.GetDB()
			now := time.Now().Unix()
			raw := fmt.Sprintf("[ {\"ip\":\"192.0.2.2\",\"timestamp\":%d}, {\"ip\":\"192.0.2.1\",\"timestamp\":%d} ]", now, now-1)
			row := model.InboundClientIps{ClientEmail: "raw@x", Ips: raw}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			counts := countClientIpSQL(t, "inbound_client_ips")
			injected := false
			if contend {
				const callback = "test:ip_one_miss"
				if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
					if injected || tx.Statement.Table != "inbound_client_ips" {
						return
					}
					where, ok := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
					if !ok || len(where.Exprs) != 1 {
						return
					}
					predicate, ok := where.Exprs[0].(clause.Expr)
					if !ok || len(predicate.Vars) != 2 || predicate.Vars[0] != row.Id {
						return
					}
					injected = true
					winner := rawClientIpBlob(t, model.ClientIpEntry{IP: "198.51.100.1", Timestamp: now + 2})
					if err := tx.Exec("UPDATE inbound_client_ips SET ips = ? WHERE id = ?", winner, row.Id).Error; err != nil {
						tx.AddError(err)
					}
				}); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
			}
			if err := (&InboundService{}).MergeInboundClientIps([]model.InboundClientIps{{ClientEmail: row.ClientEmail, Ips: rawClientIpBlob(t, model.ClientIpEntry{IP: "203.0.113.1", Timestamp: now + 1})}}); err != nil {
				t.Fatal(err)
			}
			wantCount := int64(1)
			want := map[string]int64{"192.0.2.1": now - 1, "192.0.2.2": now, "203.0.113.1": now + 1}
			if contend {
				wantCount = 2
				want = map[string]int64{"198.51.100.1": now + 2, "203.0.113.1": now + 1}
				if !injected {
					t.Fatal("competing SQL update was not injected")
				}
			}
			if counts.selects.Load() != wantCount || counts.updates.Load() != wantCount {
				t.Fatalf("SELECTs/CAS=%d/%d, want %d/%d", counts.selects.Load(), counts.updates.Load(), wantCount, wantCount)
			}
			requireClientIpSet(t, row.ClientEmail, want)
			var got model.InboundClientIps
			if err := db.First(&got, row.Id).Error; err != nil {
				t.Fatal(err)
			}
			var entries []model.ClientIpEntry
			if err := json.Unmarshal([]byte(got.Ips), &entries); err != nil {
				t.Fatal(err)
			}
			for i := 1; i < len(entries); i++ {
				if entries[i-1].Timestamp > entries[i].Timestamp || (entries[i-1].Timestamp == entries[i].Timestamp && entries[i-1].IP > entries[i].IP) {
					t.Fatalf("noncanonical persisted blob %s", got.Ips)
				}
			}
		})
	}
}
