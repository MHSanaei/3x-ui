package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

type clientIpPGStats struct {
	Calls             float64
	Rows              float64
	TotalExecTime     float64
	WalRecords        float64
	WalFpi            float64
	WalBytes          float64
	SharedBlksHit     float64
	SharedBlksRead    float64
	SharedBlksDirtied float64
}

func clientIpPGSnapshot(t *testing.T) clientIpPGStats {
	t.Helper()
	var out clientIpPGStats
	db := database.GetDB()
	if db.Name() != "postgres" {
		return out
	}
	err := db.Raw(`SELECT COALESCE(sum(calls),0) AS calls, COALESCE(sum(rows),0) AS rows,
		COALESCE(sum(total_exec_time),0) AS total_exec_time, COALESCE(sum(wal_records),0) AS wal_records,
		COALESCE(sum(wal_fpi),0) AS wal_fpi, COALESCE(sum(wal_bytes),0) AS wal_bytes,
		COALESCE(sum(shared_blks_hit),0) AS shared_blks_hit, COALESCE(sum(shared_blks_read),0) AS shared_blks_read,
		COALESCE(sum(shared_blks_dirtied),0) AS shared_blks_dirtied
		FROM public.pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		AND (query ILIKE '%inbound_client_ips%' OR query ILIKE '%node_client_ips%')
		AND query NOT ILIKE '%pg_stat_statements%'`).Scan(&out).Error
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func logClientIpPGDelta(t *testing.T, before, after clientIpPGStats) {
	t.Helper()
	if database.GetDB().Name() != "postgres" {
		return
	}
	t.Logf("PG delta calls=%.0f rows=%.0f total_exec_time_ms=%.3f wal_records=%.0f wal_fpi=%.0f wal_bytes=%.0f shared_blks_hit=%.0f read=%.0f dirtied=%.0f",
		after.Calls-before.Calls, after.Rows-before.Rows, after.TotalExecTime-before.TotalExecTime,
		after.WalRecords-before.WalRecords, after.WalFpi-before.WalFpi, after.WalBytes-before.WalBytes,
		after.SharedBlksHit-before.SharedBlksHit, after.SharedBlksRead-before.SharedBlksRead, after.SharedBlksDirtied-before.SharedBlksDirtied)
}

func logClientIpPGContext(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	if db.Name() != "postgres" {
		return
	}
	var info struct {
		StatsReset string
		Dealloc    int64
	}
	if err := db.Raw("SELECT stats_reset::text, dealloc FROM public.pg_stat_statements_info").Scan(&info).Error; err != nil {
		t.Fatal(err)
	}
	var version string
	if err := db.Raw("SELECT version()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		FullPageWrites    string
		CheckpointTimeout string
	}
	if err := db.Raw("SELECT current_setting('full_page_writes') AS full_page_writes, current_setting('checkpoint_timeout') AS checkpoint_timeout").Scan(&settings).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("PG %s stats_reset=%s dealloc=%d full_page_writes=%s checkpoint_timeout=%s", version, info.StatsReset, info.Dealloc, settings.FullPageWrites, settings.CheckpointTimeout)
}

func TestClientIpSyncScale(t *testing.T) {
	for _, n := range scaleSizes(t, 1000, 10000, 20000) {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			cleanup, err := testpg.IsolatePackage("client_ip_sync_scale")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cleanup)
			setupScaleDB(t)
			db := database.GetDB()
			resetScaleTables(t, db, "inbound_client_ips", "node_client_ips")
			logClientIpPGContext(t)
			now := time.Now().Unix()
			makeReport := func(node int, advance int64) ([]model.InboundClientIps, map[string][]model.ClientIpEntry) {
				rows := make([]model.InboundClientIps, n)
				attribution := make(map[string][]model.ClientIpEntry, n)
				for i := range n {
					email := fmt.Sprintf("sync-%07d@scale", i)
					entries := []model.ClientIpEntry{{IP: fmt.Sprintf("2001:db8:%x::%x", node+1, i+1), Timestamp: now + int64(node) + advance}}
					b, err := json.Marshal(entries)
					if err != nil {
						t.Fatal(err)
					}
					rows[i] = model.InboundClientIps{ClientEmail: email, Ips: string(b)}
					attribution[email] = entries
				}
				return rows, attribution
			}
			apply := func(node int, rows []model.InboundClientIps, attrs map[string][]model.ClientIpEntry) error {
				if err := (&InboundService{}).MergeInboundClientIps(rows); err != nil {
					return err
				}
				return (&InboundService{}).RecordLocalClientIps(fmt.Sprintf("bench-node-%d", node), attrs)
			}
			for node := range 4 {
				rows, attrs := makeReport(node, 0)
				if err := apply(node, rows, attrs); err != nil {
					t.Fatal(err)
				}
			}
			flat := countClientIpSQL(t, "inbound_client_ips")
			attr := countClientIpSQL(t, "node_client_ips")
			for phase, advance := range []int64{0, 10, 20} {
				t.Run([]string{"unchanged", "advancing", "contention"}[phase], func(t *testing.T) {
					reports := make([][]model.InboundClientIps, 4)
					attrs := make([]map[string][]model.ClientIpEntry, 4)
					for node := range 4 {
						reports[node], attrs[node] = makeReport(node, advance)
					}
					before := clientIpPGSnapshot(t)
					fs, fu, fa := flat.selects.Load(), flat.updates.Load(), flat.affected.Load()
					as, ac, aa := attr.selects.Load(), attr.creates.Load(), attr.affected.Load()
					start := time.Now()
					if phase == 2 {
						var wg sync.WaitGroup
						errs := make(chan error, 4)
						for node := range 4 {
							wg.Go(func() { errs <- apply(node, reports[node], attrs[node]) })
						}
						wg.Wait()
						close(errs)
						for err := range errs {
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						for node := range 4 {
							if err := apply(node, reports[node], attrs[node]); err != nil {
								t.Fatal(err)
							}
						}
					}
					elapsed := time.Since(start)
					after := clientIpPGSnapshot(t)
					t.Logf("N=%d reports=4 phase=%s elapsed=%v flat_SELECTs=%d CAS_attempts=%d CAS_misses=%d flat_affected=%d attribution_SELECTs=%d upsert_attempts=%d attribution_affected=%d", n, t.Name(), elapsed,
						flat.selects.Load()-fs, flat.updates.Load()-fu, flat.updates.Load()-fu-(flat.affected.Load()-fa), flat.affected.Load()-fa,
						attr.selects.Load()-as, attr.creates.Load()-ac, attr.affected.Load()-aa)
					logClientIpPGDelta(t, before, after)
					var actual []model.InboundClientIps
					if err := db.Find(&actual).Error; err != nil {
						t.Fatal(err)
					}
					if len(actual) != n {
						t.Fatalf("flat rows=%d, want %d", len(actual), n)
					}
					for _, row := range actual {
						var entries []model.ClientIpEntry
						if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
							t.Fatal(err)
						}
						got := make(map[string]int64, len(entries))
						for _, entry := range entries {
							got[entry.IP] = entry.Timestamp
						}
						want := make(map[string]int64, 4)
						for node := range 4 {
							for _, entry := range attrs[node][row.ClientEmail] {
								want[entry.IP] = entry.Timestamp
							}
						}
						if len(entries) != 4 || !reflect.DeepEqual(got, want) {
							t.Fatalf("%s entries=%v, want %v", row.ClientEmail, entries, want)
						}
					}
					trees, err := (&InboundService{}).GetClientIpsByGuid()
					if err != nil {
						t.Fatal(err)
					}
					for node := range 4 {
						if !reflect.DeepEqual(trees[fmt.Sprintf("bench-node-%d", node)], attrs[node]) {
							t.Fatalf("node %d attribution does not match exact report", node)
						}
					}
				})
			}
			logClientIpPGContext(t)
		})
	}
}
