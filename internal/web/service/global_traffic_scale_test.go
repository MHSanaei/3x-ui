package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type globalTrafficPGStats struct {
	Calls, Rows, WalRecords, WalFpi                  int64
	TotalExecTime, WalBytes                          float64
	SharedBlksHit, SharedBlksRead, SharedBlksDirtied int64
}

func globalTrafficStats(t *testing.T, db *gorm.DB) globalTrafficPGStats {
	t.Helper()
	var stats globalTrafficPGStats
	if db.Name() != "postgres" {
		return stats
	}
	if err := db.Raw(`SELECT COALESCE(sum(calls),0) AS calls, COALESCE(sum(rows),0) AS rows,
		COALESCE(sum(total_exec_time),0) AS total_exec_time, COALESCE(sum(wal_records),0) AS wal_records,
		COALESCE(sum(wal_fpi),0) AS wal_fpi, COALESCE(sum(wal_bytes),0)::float8 AS wal_bytes,
		COALESCE(sum(shared_blks_hit),0) AS shared_blks_hit, COALESCE(sum(shared_blks_read),0) AS shared_blks_read,
		COALESCE(sum(shared_blks_dirtied),0) AS shared_blks_dirtied
		FROM public.pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database())
		AND query ILIKE 'INSERT INTO "client_global_traffics"%'`).Scan(&stats).Error; err != nil {
		t.Fatal(err)
	}
	var info struct {
		StatsReset time.Time
		Dealloc    int64
	}
	if err := db.Raw("SELECT stats_reset, dealloc FROM public.pg_stat_statements_info").Scan(&info).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("pg_stat_statements_info stats_reset=%s dealloc=%d cumulative=%+v", info.StatsReset.Format(time.RFC3339Nano), info.Dealloc, stats)
	return stats
}

func TestAcceptGlobalTrafficScale(t *testing.T) {
	if os.Getenv("XUI_SCALE_TEST") != "1" {
		t.Skip("set XUI_SCALE_TEST=1")
	}
	setupScaleDB(t)
	db := database.GetDB()
	for _, workload := range []string{"unchanged", "sparse", "all", "reset"} {
		t.Run(workload, func(t *testing.T) {
			resetScaleTables(t, db, "client_global_traffics", "client_traffics")
			clock := globalTrafficClock(t, db, true)
			const clients, ticks = 6708, 240
			locals := make([]xray.ClientTraffic, clients)
			payload := make([]*xray.ClientTraffic, clients)
			for i := range clients {
				email := fmt.Sprintf("heartbeat-scale-%05d", i)
				locals[i] = xray.ClientTraffic{Email: email, InboundId: 1, Enable: true}
				payload[i] = &xray.ClientTraffic{Email: email, Up: 1000, Down: 2000}
			}
			if err := db.CreateInBatches(&locals, 200).Error; err != nil {
				t.Fatal(err)
			}
			pushGlobalTraffic(t, "scale-master", payload...)
			before := globalTrafficStats(t, db)
			writes := countGlobalTrafficWrites(t, db)
			startClock := clock.Load()
			start := time.Now()
			for tick := 1; tick <= ticks; tick++ {
				clock.Store(startClock + int64(tick)*30_000)
				for i, row := range payload {
					switch workload {
					case "sparse":
						if (i+tick)%100 == 0 {
							row.Up++
							row.Down++
						}
					case "all":
						row.Up++
						row.Down++
					case "reset":
						if tick == 120 {
							row.Up, row.Down = 0, 0
						}
					}
				}
				pushGlobalTraffic(t, "scale-master", payload...)
			}
			elapsed := time.Since(start)
			after := globalTrafficStats(t, db)
			delta := globalTrafficPGStats{
				Calls: after.Calls - before.Calls, Rows: after.Rows - before.Rows,
				TotalExecTime: after.TotalExecTime - before.TotalExecTime,
				WalRecords:    after.WalRecords - before.WalRecords, WalFpi: after.WalFpi - before.WalFpi, WalBytes: after.WalBytes - before.WalBytes,
				SharedBlksHit: after.SharedBlksHit - before.SharedBlksHit, SharedBlksRead: after.SharedBlksRead - before.SharedBlksRead,
				SharedBlksDirtied: after.SharedBlksDirtied - before.SharedBlksDirtied,
			}
			t.Logf("dialect=%s clients=%d ticks=%d warmup=one initial push workload=%s elapsed=%s affected_rows=%d pg_delta=%+v", db.Name(), clients, ticks, workload, elapsed, writes.Load(), delta)
			var rows []model.ClientGlobalTraffic
			if err := db.Where("master_guid = ?", "scale-master").Order("email").Find(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if len(rows) != clients {
				t.Fatalf("rows=%d want %d", len(rows), clients)
			}
			for i, got := range rows {
				if got.Up != payload[i].Up || got.Down != payload[i].Down {
					t.Fatalf("client %d counters=%d/%d want %d/%d", i, got.Up, got.Down, payload[i].Up, payload[i].Down)
				}
			}
		})
	}
}
