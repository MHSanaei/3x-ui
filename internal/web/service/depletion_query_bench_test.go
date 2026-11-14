package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Keep the pre-7008 SQL here so measurements remain reproducible after the fix.
const depletionLegacyCond = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?)
	OR (total > 0 AND EXISTS (
		SELECT 1 FROM client_global_traffics g
		WHERE g.email = client_traffics.email AND g.updated_at >= ?
			AND g.up + g.down >= client_traffics.total)))`

const depletionSetCond = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?)
	OR id IN (SELECT c.id FROM client_traffics c
		JOIN client_global_traffics g ON g.email = c.email
		WHERE c.total > 0 AND g.updated_at >= ? AND g.up + g.down >= c.total))`

const depletionUnionCond = `id IN (
	SELECT id FROM client_traffics WHERE (total > 0 AND up + down >= total)
		OR (expiry_time > 0 AND expiry_time <= ?)
	UNION SELECT c.id FROM client_traffics c
		JOIN client_global_traffics g ON g.email = c.email
		WHERE c.total > 0 AND g.updated_at >= ? AND g.up + g.down >= c.total)`

// TestDepletionQueryMeasurements is an opt-in, read-only query comparison on
// disposable fixtures. PostgreSQL uses the existing isolated-schema helper.
// XUI_DEPLETION_MEASURE_DIR retains CSV metrics and complete query plans.
// Run with XUI_DEPLETION_MEASURE=1; use XUI_DB_TYPE/XUI_DB_DSN for PostgreSQL.
func TestDepletionQueryMeasurements(t *testing.T) {
	if os.Getenv("XUI_DEPLETION_MEASURE") != "1" {
		t.Skip("set XUI_DEPLETION_MEASURE=1 to run the disposable query measurements")
	}
	db := initGlobalHeartbeatDB(t).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	db.NowFunc = func() time.Time { return now }
	dir := os.Getenv("XUI_DEPLETION_MEASURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var indexes []struct{ Name, SQL string }
	indexSQL := "SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name IN ('client_traffics', 'client_global_traffics') ORDER BY name"
	if db.Name() == "postgres" {
		indexSQL = "SELECT indexname AS name, indexdef AS sql FROM pg_indexes WHERE schemaname = current_schema() AND tablename IN ('client_traffics', 'client_global_traffics') ORDER BY indexname"
	}
	if err := db.Raw(indexSQL).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	indexData, err := json.MarshalIndent(indexes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, db.Name()+"-indexes.json"), indexData, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, db.Name()+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer func() {
		w.Flush()
		if err := w.Error(); err != nil {
			t.Error(err)
		}
	}()
	if err := w.Write([]string{"rows", "scenario", "variant", "matches", "client_ms_p50", "client_ms_p95", "server_ms", "shared_hits", "shared_reads", "plan_rows", "actual_rows", "max_subplan_loops", "window_calls", "window_server_ms", "window_shared_hits"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1000, 10000, 20000} {
		for _, scenario := range []string{"local-only", "empty", "stale", "fresh", "multi-master", "mixed"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				seedDepletionMeasurements(t, db, n, scenario, now)
				fresh := globalTrafficFreshSince(now)
				for _, variant := range []string{"legacy", "set", "union", "current"} {
					cond, args := depletionLegacyCond, []any{now.UnixMilli(), fresh}
					switch variant {
					case "set":
						cond = depletionSetCond
					case "union":
						cond = depletionUnionCond
					}
					probe := func() {
						if scenario == "local-only" {
							cond, args = depletedClientsCondLocal, []any{now.UnixMilli()}
							return
						}
						if variant == "current" {
							cond, args = depletedCondWithGlobal(db, depletedClientsScanCond)
							return
						}
						var count int64
						if err := db.Model(&model.ClientGlobalTraffic{}).Where("updated_at >= ?", fresh).Limit(1).Count(&count).Error; err != nil {
							t.Fatal(err)
						}
						if count == 0 {
							cond, args = depletedClientsCondLocal, []any{now.UnixMilli()}
						}
					}
					var rows []xray.ClientTraffic
					query := func() {
						probe()
						rows = nil
						if err := db.Where(cond, args...).Where("enable = ?", true).Find(&rows).Error; err != nil {
							t.Fatal(err)
						}
					}
					for range 5 {
						query()
					}
					var samples []float64
					before := depletionStatementTotals(t, db)
					for range 30 {
						start := time.Now()
						query()
						samples = append(samples, float64(time.Since(start).Nanoseconds())/1e6)
					}
					after := depletionStatementTotals(t, db)
					slices.Sort(samples)
					// Compare exact candidate identities, not just counts.
					var want []int
					if err := db.Model(&xray.ClientTraffic{}).Where(depletionLegacyCond, now.UnixMilli(), fresh).Where("enable = ?", true).Order("id").Pluck("id", &want).Error; err != nil {
						t.Fatal(err)
					}
					got := make([]int, 0, len(rows))
					for _, row := range rows {
						got = append(got, row.Id)
					}
					slices.Sort(got)
					if !slices.Equal(got, want) {
						t.Fatalf("%s candidate mismatch: %v want %v", variant, got, want)
					}
					stmt := db.Session(&gorm.Session{DryRun: true}).Where(cond, args...).Where("enable = ?", true).Find(&[]xray.ClientTraffic{}).Statement
					metrics := []string{"", "", "", "", "", ""}
					stem := fmt.Sprintf("%s-%d-%s-%s", db.Name(), n, scenario, variant)
					if db.Name() == "postgres" {
						var plan string
						if err := db.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+stmt.SQL.String(), stmt.Vars...).Row().Scan(&plan); err != nil {
							t.Fatal(err)
						}
						var parsed []map[string]any
						if err := json.Unmarshal([]byte(plan), &parsed); err != nil {
							t.Fatal(err)
						}
						root := parsed[0]["Plan"].(map[string]any)
						maxLoops := float64(0)
						var walk func(map[string]any)
						walk = func(p map[string]any) {
							if _, ok := p["Subplan Name"]; ok {
								maxLoops = max(maxLoops, p["Actual Loops"].(float64))
							}
							children, _ := p["Plans"].([]any)
							for _, c := range children {
								walk(c.(map[string]any))
							}
						}
						walk(root)
						metrics = []string{fmt.Sprint(parsed[0]["Execution Time"]), fmt.Sprint(root["Shared Hit Blocks"]), fmt.Sprint(root["Shared Read Blocks"]), fmt.Sprint(root["Plan Rows"]), fmt.Sprint(root["Actual Rows"]), fmt.Sprint(maxLoops)}
						if err := os.WriteFile(filepath.Join(dir, stem+".json"), []byte(plan), 0o644); err != nil {
							t.Fatal(err)
						}
					} else {
						var plan []struct{ Detail string }
						if err := db.Raw("EXPLAIN QUERY PLAN "+stmt.SQL.String(), stmt.Vars...).Scan(&plan).Error; err != nil {
							t.Fatal(err)
						}
						data, err := json.MarshalIndent(plan, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dir, stem+".json"), data, 0o644); err != nil {
							t.Fatal(err)
						}
					}
					record := []string{strconv.Itoa(n), scenario, variant, strconv.Itoa(len(rows)), fmt.Sprintf("%.4f", samples[15]), fmt.Sprintf("%.4f", samples[28])}
					record = append(record, metrics...)
					record = append(record, depletionWindowMetrics(db, before, after)...)
					if err := w.Write(record); err != nil {
						t.Fatal(err)
					}
					t.Logf("%s: %s", stem, record)
				}
			})
		}
	}
}

type depletionSQLTotals struct {
	Calls int64
	MS    float64
	Hits  int64
}

func depletionWindowMetrics(db *gorm.DB, before, after depletionSQLTotals) []string {
	if db.Name() != "postgres" || os.Getenv("XUI_DEPLETION_PG_STATS") != "1" {
		return []string{"", "", ""}
	}
	return []string{fmt.Sprint(after.Calls - before.Calls), fmt.Sprintf("%.4f", after.MS-before.MS), fmt.Sprint(after.Hits - before.Hits)}
}

// Opt-in only: the benchmark database must already have pg_stat_statements.
// Snapshot deltas avoid resetting any statistics, including unrelated queries.
func depletionStatementTotals(t *testing.T, db *gorm.DB) depletionSQLTotals {
	t.Helper()
	var stats depletionSQLTotals
	if db.Name() != "postgres" || os.Getenv("XUI_DEPLETION_PG_STATS") != "1" {
		return stats
	}
	if err := db.Raw(`SELECT COALESCE(SUM(calls), 0) AS calls,
		COALESCE(SUM(total_exec_time), 0) AS ms, COALESCE(SUM(shared_blks_hit), 0) AS hits
		FROM public.pg_stat_statements
		WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		AND query LIKE 'SELECT%' AND query NOT LIKE '%pg_stat_statements%'
		AND (query LIKE '%client_traffics%' OR query LIKE '%client_global_traffics%')`).Scan(&stats).Error; err != nil {
		t.Fatal(err)
	}
	return stats
}

func TestDepletionProbeMeasurements(t *testing.T) {
	if os.Getenv("XUI_DEPLETION_MEASURE") != "1" {
		t.Skip("set XUI_DEPLETION_MEASURE=1 to run the disposable query measurements")
	}
	db := initGlobalHeartbeatDB(t).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	dir := os.Getenv("XUI_DEPLETION_MEASURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, db.Name()+"-probe.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer func() {
		w.Flush()
		if err := w.Error(); err != nil {
			t.Error(err)
		}
	}()
	if err := w.Write([]string{"rows", "scenario", "variant", "client_ms_p50", "client_ms_p95", "window_calls", "window_server_ms", "window_shared_hits"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1000, 10000, 20000} {
		for _, scenario := range []string{"empty", "stale", "fresh", "multi-master"} {
			seedDepletionMeasurements(t, db, n, scenario, now)
			for _, variant := range []string{"count", "exists"} {
				sql := "SELECT count(*) FROM client_global_traffics WHERE updated_at >= ? LIMIT 1"
				if variant == "exists" {
					sql = "SELECT EXISTS (SELECT 1 FROM client_global_traffics WHERE updated_at >= ?)"
				}
				query := func() {
					var value int64
					var exists bool
					var err error
					if variant == "exists" {
						err = db.Raw(sql, globalTrafficFreshSince(now)).Scan(&exists).Error
					} else {
						err = db.Raw(sql, globalTrafficFreshSince(now)).Scan(&value).Error
						exists = value > 0
					}
					if err != nil {
						t.Fatal(err)
					}
					want := scenario == "fresh" || scenario == "multi-master"
					if exists != want {
						t.Fatalf("%s/%s existence=%v want %v", scenario, variant, exists, want)
					}
				}
				for range 5 {
					query()
				}
				before := depletionStatementTotals(t, db)
				var samples []float64
				for range 30 {
					start := time.Now()
					query()
					samples = append(samples, float64(time.Since(start).Nanoseconds())/1e6)
				}
				after := depletionStatementTotals(t, db)
				slices.Sort(samples)
				record := []string{strconv.Itoa(n), scenario, variant, fmt.Sprintf("%.4f", samples[15]), fmt.Sprintf("%.4f", samples[28])}
				if err := w.Write(append(record, depletionWindowMetrics(db, before, after)...)); err != nil {
					t.Fatal(err)
				}
				if db.Name() == "postgres" {
					var plan string
					if err := db.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+sql, globalTrafficFreshSince(now)).Row().Scan(&plan); err != nil {
						t.Fatal(err)
					}
					name := fmt.Sprintf("postgres-probe-%d-%s-%s.json", n, scenario, variant)
					if err := os.WriteFile(filepath.Join(dir, name), []byte(plan), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}

func seedDepletionMeasurements(t *testing.T, db *gorm.DB, n int, scenario string, now time.Time) {
	t.Helper()
	if db.Name() == "postgres" {
		if err := db.Exec("TRUNCATE client_traffics, client_global_traffics RESTART IDENTITY").Error; err != nil {
			t.Fatal(err)
		}
	} else {
		for _, table := range []string{"client_traffics", "client_global_traffics"} {
			if err := db.Exec("DELETE FROM " + table).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	clients := make([]xray.ClientTraffic, 0, n)
	var globals []model.ClientGlobalTraffic
	for i := range n {
		row := xray.ClientTraffic{Email: fmt.Sprintf("client-%06d@example.test", i), Enable: i%5 != 0, InboundId: i%8 + 1, Up: int64(i%1000) * 1000, Down: int64(i%1000) * 2000, Total: 10 << 30, ExpiryTime: now.Add(30 * 24 * time.Hour).UnixMilli()}
		if i%10 == 9 {
			row.Total = 0
		}
		if scenario == "mixed" {
			if i%200 == 1 {
				row.Up = row.Total
			}
			if i%200 == 2 {
				row.ExpiryTime = now.UnixMilli()
			}
		}
		clients = append(clients, row)
		if scenario == "local-only" || scenario == "empty" || i%10 >= 7 {
			continue
		}
		masters := 1
		if scenario == "multi-master" {
			masters = 3
		}
		for master := range masters {
			g := model.ClientGlobalTraffic{MasterGuid: fmt.Sprintf("master-%d", master), Email: row.Email, Up: row.Up + 100, Down: row.Down + 200, UpdatedAt: now.UnixMilli()}
			if scenario == "stale" || master == 2 {
				g.UpdatedAt = now.Add(-25 * time.Hour).UnixMilli()
			}
			if scenario == "mixed" && i%200 == 3 {
				g.Up = row.Total
			}
			globals = append(globals, g)
		}
	}
	if err := db.CreateInBatches(clients, 200).Error; err != nil {
		t.Fatal(err)
	}
	if len(globals) > 0 {
		if err := db.CreateInBatches(globals, 200).Error; err != nil {
			t.Fatal(err)
		}
	}
	if db.Name() == "postgres" {
		for _, table := range []string{"client_traffics", "client_global_traffics"} {
			if err := db.Exec("VACUUM ANALYZE " + table).Error; err != nil {
				t.Fatal(err)
			}
		}
	} else if err := db.Exec("ANALYZE").Error; err != nil {
		t.Fatal(err)
	}
}
