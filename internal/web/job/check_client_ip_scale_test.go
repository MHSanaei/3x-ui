package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

// setupScaleJobDB mirrors the service package's scale gating: Postgres via
// XUI_DB_TYPE/XUI_DB_DSN, SQLite via XUI_SCALE_TEST=1, skip otherwise.
func setupScaleJobDB(t *testing.T) {
	t.Helper()
	loggerInitOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())

	if os.Getenv("XUI_DB_TYPE") == "postgres" && strings.TrimSpace(os.Getenv("XUI_DB_DSN")) != "" {
		cleanup, err := testpg.IsolatePackage("job_ip_scale")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(cleanup)
		t.Cleanup(func() { _ = database.CloseDB() })
		if err := database.InitDB(""); err != nil {
			t.Fatalf("InitDB(postgres): %v", err)
		}
		return
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("XUI_SCALE_TEST"))) {
	case "1", "true", "yes":
		dbtest.InitDB(t, filepath.Join(t.TempDir(), "scale.db"))
		return
	}
	t.Skip("set XUI_SCALE_TEST=1 (sqlite) or XUI_DB_TYPE=postgres + XUI_DB_DSN (postgres) to run the scale benchmark")
}

func scaleJobSizes(t *testing.T, def ...int) []int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("XUI_SCALE_SIZES"))
	if raw == "" {
		return def
	}
	var out []int
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			t.Fatalf("XUI_SCALE_SIZES: invalid size %q", part)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func resetScaleJobTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if config.GetDBKind() == "postgres" {
		if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate: %v", err)
		}
	} else {
		for _, tbl := range []string{"inbounds", "clients", "client_inbounds"} {
			if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
				t.Fatalf("delete %s: %v", tbl, err)
			}
		}
		db.Exec("DELETE FROM sqlite_sequence")
	}
	if err := db.Where("1 = 1").Delete(&model.InboundClientIps{}).Error; err != nil {
		t.Fatalf("clear inbound client ips: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&model.NodeClientIp{}).Error; err != nil {
		t.Fatalf("clear node client ips: %v", err)
	}
}

// seedScaleIPDataset seeds n clients across numInbounds inbounds. Every client
// in the LAST inbound carries limitIp=3 (and 0 elsewhere), so hasLimitIp pays
// its full scan cost before finding a hit, and the returned emails all resolve
// to that last inbound for the processObserved measurement.
func seedScaleIPDataset(t *testing.T, n, numInbounds int) []string {
	t.Helper()
	db := database.GetDB()
	resetScaleJobTables(t, db)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin seed tx: %v", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	var limitedEmails []string
	per := n / numInbounds
	for i := range numInbounds {
		lo, hi := i*per, (i+1)*per
		if i == numInbounds-1 {
			hi = n
		}
		limitIp := 0
		if i == numInbounds-1 {
			limitIp = 3
		}
		clients := make([]model.Client, 0, hi-lo)
		records := make([]*model.ClientRecord, 0, hi-lo)
		for j := lo; j < hi; j++ {
			email := fmt.Sprintf("user-%07d@ipscale", j)
			clients = append(clients, model.Client{Email: email, LimitIP: limitIp, Enable: true})
			records = append(records, &model.ClientRecord{Email: email, LimitIP: limitIp, Enable: true})
			if limitIp > 0 {
				limitedEmails = append(limitedEmails, email)
			}
		}
		settings, err := json.Marshal(map[string][]model.Client{"clients": clients})
		if err != nil {
			t.Fatalf("marshal settings: %v", err)
		}
		ib := &model.Inbound{
			UserId:   1,
			Tag:      fmt.Sprintf("ipscale-%d-%d", n, i),
			Enable:   true,
			Port:     42000 + i,
			Protocol: model.VLESS,
			Settings: string(settings),
		}
		if err := tx.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound %d: %v", i, err)
		}
		if err := tx.CreateInBatches(records, 500).Error; err != nil {
			t.Fatalf("seed clients %d: %v", i, err)
		}
		links := make([]model.ClientInbound, len(records))
		for j := range records {
			links[j] = model.ClientInbound{ClientId: records[j].Id, InboundId: ib.Id}
		}
		if err := tx.CreateInBatches(links, 1000).Error; err != nil {
			t.Fatalf("seed client_inbounds %d: %v", i, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}
	committed = true
	db.Exec("ANALYZE")
	return limitedEmails
}

// TestCheckClientIpScale measures the gate and two scans over an identical fixture.
func TestCheckClientIpScale(t *testing.T) {
	shapes := []struct {
		name     string
		inbounds int
		observed int
	}{{"single", 1, 50}, {"spread50", 50, 1000}}

	for _, n := range scaleJobSizes(t, 10000, 100000) {
		for _, shape := range shapes {
			t.Run(fmt.Sprintf("N=%d_%s", n, shape.name), func(t *testing.T) {
				setupScaleJobDB(t)
				limited := seedScaleIPDataset(t, n, shape.inbounds)
				m := min(shape.observed, len(limited))

				j := NewCheckClientIpJob()
				const reps = 3
				start := time.Now()
				for range reps {
					if !j.hasLimitIp() {
						t.Fatal("hasLimitIp = false, want true")
					}
				}
				t.Logf("N=%-7d shape=%-8s hasLimitIp=%v/call", n, shape.name, (time.Since(start) / reps).Round(time.Millisecond))

				now := time.Now().Unix()
				observed := make(map[string]map[string]int64, m)
				for i := range m {
					observed[limited[i]] = map[string]int64{
						fmt.Sprintf("10.0.%d.%d", i/250, i%250+1): now,
					}
				}
				for i := range m / 2 {
					seedClientIps(t, limited[i], []model.ClientIpEntry{{IP: "10.99.0.1", Timestamp: now - 60}})
				}

				counts := countScannerScaleSQL(t)
				before := captureScannerScaleStats(t)
				var memoryBefore, memoryAfter runtime.MemStats
				runtime.ReadMemStats(&memoryBefore)
				start = time.Now()
				j.processObserved(observed, true, true)
				firstScan := time.Since(start)
				firstCounts := counts.snapshot()
				start = time.Now()
				j.processObserved(observed, true, true)
				secondScan := time.Since(start)
				runtime.ReadMemStats(&memoryAfter)
				finalCounts := counts.snapshot()
				t.Logf("first scan SQL flat_SELECTs=%d per_ID_SELECTs=%d CAS_attempts=%d CAS_misses=%d flat_creates=%d flat_affected=%d attribution_SELECTs=%d upsert_attempts=%d attribution_affected=%d", firstCounts[0], firstCounts[1], firstCounts[2], firstCounts[2]-firstCounts[3], firstCounts[4], firstCounts[3]+firstCounts[5], firstCounts[6], firstCounts[7], firstCounts[8])
				t.Logf("second scan SQL flat_SELECTs=%d per_ID_SELECTs=%d CAS_attempts=%d CAS_misses=%d flat_creates=%d flat_affected=%d attribution_SELECTs=%d upsert_attempts=%d attribution_affected=%d", finalCounts[0]-firstCounts[0], finalCounts[1]-firstCounts[1], finalCounts[2]-firstCounts[2], finalCounts[2]-firstCounts[2]-(finalCounts[3]-firstCounts[3]), finalCounts[4]-firstCounts[4], finalCounts[3]-firstCounts[3]+finalCounts[5]-firstCounts[5], finalCounts[6]-firstCounts[6], finalCounts[7]-firstCounts[7], finalCounts[8]-firstCounts[8])
				after := captureScannerScaleStats(t)
				t.Logf("scan allocations=%d bytes=%d", memoryAfter.Mallocs-memoryBefore.Mallocs, memoryAfter.TotalAlloc-memoryBefore.TotalAlloc)
				logScannerScaleStats(t, before, after)
				t.Logf("N=%-7d shape=%-8s processObserved M=%-5d first=%-10v second=%-10v (%.1fms/email)",
					n, shape.name, m, firstScan.Round(time.Millisecond), secondScan.Round(time.Millisecond),
					float64(secondScan.Milliseconds())/float64(m))

				var rows int64
				if err := database.GetDB().Model(&model.InboundClientIps{}).Count(&rows).Error; err != nil {
					t.Fatalf("count ip rows: %v", err)
				}
				if rows != int64(m) {
					t.Fatalf("inbound_client_ips rows = %d, want %d", rows, m)
				}
				for i := range m {
					want := map[string]int64{fmt.Sprintf("10.0.%d.%d", i/250, i%250+1): now}
					if i < m/2 {
						want["10.99.0.1"] = now - 60
					}
					if got := ipSet(readClientIps(t, limited[i])); !reflect.DeepEqual(got, want) {
						t.Fatalf("retained %s = %v, want %v", limited[i], got, want)
					}
				}
			})
		}
	}
}

type scannerScaleStats struct {
	Calls, Rows, WALRecords, WALFpi, WALBytes, SharedBlksHit, SharedBlksRead, SharedBlksDirtied int64
	TotalExecTime                                                                               float64
	StatsReset                                                                                  string
	Dealloc                                                                                     int64
}

func captureScannerScaleStats(t *testing.T) scannerScaleStats {
	t.Helper()
	var result scannerScaleStats
	if config.GetDBKind() != "postgres" {
		return result
	}
	db := database.GetDB()
	if err := db.Raw(`SELECT COALESCE(sum(calls),0) AS calls, COALESCE(sum(rows),0) AS rows,
		COALESCE(sum(total_exec_time),0) AS total_exec_time,
		COALESCE(sum(wal_records),0) AS wal_records, COALESCE(sum(wal_fpi),0) AS wal_fpi, COALESCE(sum(wal_bytes),0) AS wal_bytes,
		COALESCE(sum(shared_blks_hit),0) AS shared_blks_hit, COALESCE(sum(shared_blks_read),0) AS shared_blks_read,
		COALESCE(sum(shared_blks_dirtied),0) AS shared_blks_dirtied
		FROM public.pg_stat_statements
		WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		AND (query ILIKE '%inbound_client_ips%' OR query ILIKE '%node_client_ips%')
		AND query NOT ILIKE '%pg_stat_statements%'`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	var info struct {
		StatsReset string
		Dealloc    int64
	}
	if err := db.Raw("SELECT stats_reset::text AS stats_reset, dealloc FROM public.pg_stat_statements_info").Scan(&info).Error; err != nil {
		t.Fatal(err)
	}
	result.StatsReset, result.Dealloc = info.StatsReset, info.Dealloc
	return result
}

func logScannerScaleStats(t *testing.T, before, after scannerScaleStats) {
	t.Helper()
	if config.GetDBKind() != "postgres" {
		return
	}
	t.Logf("PG own-db IP statements delta calls=%d rows=%d total_exec_time_ms=%.3f wal_records=%d wal_fpi=%d wal_bytes=%d shared_blks_hit=%d shared_blks_read=%d shared_blks_dirtied=%d stats_reset=%q->%q dealloc=%d->%d",
		after.Calls-before.Calls, after.Rows-before.Rows, after.TotalExecTime-before.TotalExecTime,
		after.WALRecords-before.WALRecords, after.WALFpi-before.WALFpi, after.WALBytes-before.WALBytes,
		after.SharedBlksHit-before.SharedBlksHit, after.SharedBlksRead-before.SharedBlksRead, after.SharedBlksDirtied-before.SharedBlksDirtied,
		before.StatsReset, after.StatsReset, before.Dealloc, after.Dealloc)
}

type scannerScaleCounts [9]atomic.Int64

func (c *scannerScaleCounts) snapshot() [9]int64 {
	var out [9]int64
	for i := range out {
		out[i] = c[i].Load()
	}
	return out
}

func countScannerScaleSQL(t *testing.T) *scannerScaleCounts {
	t.Helper()
	db := database.GetDB()
	counts := &scannerScaleCounts{}
	const name = "test:scanner_scale_sql"
	if err := db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "inbound_client_ips":
			counts[0].Add(1)
			if strings.Contains(tx.Statement.SQL.String(), "WHERE id =") {
				counts[1].Add(1)
			}
		case "node_client_ips":
			counts[6].Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "inbound_client_ips" {
			counts[2].Add(1)
			counts[3].Add(tx.RowsAffected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Create().After("gorm:create").Register(name, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "inbound_client_ips":
			counts[4].Add(1)
			counts[5].Add(tx.RowsAffected)
		case "node_client_ips":
			counts[7].Add(1)
			counts[8].Add(tx.RowsAffected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Update().Remove(name)
		_ = db.Callback().Create().Remove(name)
	})
	return counts
}
