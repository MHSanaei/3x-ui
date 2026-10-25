package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type statsSQLMeter struct {
	logger.Interface
	identities atomic.Int64
	traffic    atomic.Int64
	elapsed    atomic.Int64
}

func (m *statsSQLMeter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.HasPrefix(sql, "SELECT") {
		if strings.Contains(sql, `FROM "clients"`) || strings.Contains(sql, "FROM `clients`") {
			m.identities.Add(1)
		}
		if strings.Contains(sql, "client_traffics") {
			m.traffic.Add(1)
		}
		m.elapsed.Add(int64(time.Since(begin)))
	}
	m.Interface.Trace(ctx, begin, fc, err)
}

type statsPGSnapshot struct {
	Calls, Rows, WalRecords, WalFpi, WalBytes, SharedBlksHit, SharedBlksRead, SharedBlksDirtied float64
	TotalExecTime                                                                               float64
}

func snapshotStatsPG(b testing.TB) statsPGSnapshot {
	b.Helper()
	var snapshot statsPGSnapshot
	err := database.GetDB().Raw(`SELECT COALESCE(sum(calls),0) AS calls, COALESCE(sum(rows),0) AS rows,
		COALESCE(sum(total_exec_time),0) AS total_exec_time, COALESCE(sum(wal_records),0) AS wal_records,
		COALESCE(sum(wal_fpi),0) AS wal_fpi, COALESCE(sum(wal_bytes),0) AS wal_bytes,
		COALESCE(sum(shared_blks_hit),0) AS shared_blks_hit, COALESCE(sum(shared_blks_read),0) AS shared_blks_read,
		COALESCE(sum(shared_blks_dirtied),0) AS shared_blks_dirtied
		FROM public.pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		AND query ILIKE 'SELECT %' AND query LIKE '%client_inbounds%' AND query LIKE '%clients%'
		AND query NOT LIKE '%pg_stat_statements%'`).Scan(&snapshot).Error
	if err != nil {
		b.Fatal(err)
	}
	return snapshot
}

func benchmarkStatsFixture(b *testing.B, n int, shared bool) ([]*model.Inbound, int) {
	b.Helper()
	setupScaleDB(b)
	ds := seedScaleDataset(b, n, 8)
	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("1 = 1").Updates(map[string]any{
		"client_host_rule_ids": "[1,2,3]", "reverse": `{"tag":"bench-reverse"}`,
		"wg_private_key": "kEgYjyW0XxZX9OM1cU9e0MF+YV6PM5nT8AgVr05Ey0E=", "wg_public_key": "4qR0/Rg0kG+1AqO1TX4IvxNN82KDBeLnYeqzdIKcQSc=",
		"wg_allowed_ips": `["10.0.0.2/32","fd00::2/128"]`, "wg_keep_alive": 25,
	}).Error; err != nil {
		b.Fatal(err)
	}
	attached := n
	if shared {
		index := 0
		for i, clients := range ds.perInbound {
			for _, client := range clients {
				selected := index%10 == 0
				index++
				if !selected {
					continue
				}
				var rec model.ClientRecord
				if err := db.Where("email = ?", client.Email).First(&rec).Error; err != nil {
					b.Fatal(err)
				}
				if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ds.inboundIds[(i+1)%8]}).Error; err != nil {
					b.Fatal(err)
				}
				attached++
			}
		}
	}
	var inbounds []*model.Inbound
	if err := db.Preload("ClientStats").Order("id ASC").Find(&inbounds).Error; err != nil {
		b.Fatal(err)
	}
	return inbounds, attached
}

func runStatsBenchmark(b *testing.B, listMode string) {
	for _, n := range []int{1000, 10000, 20000} {
		for _, shared := range []bool{false, true} {
			distribution := "disjoint"
			if shared {
				distribution = "shared"
			}
			b.Run(fmt.Sprintf("%d/%s", n, distribution), func(b *testing.B) {
				b.StopTimer()
				base, attached := benchmarkStatsFixture(b, n, shared)
				db := database.GetDB()
				meter := &statsSQLMeter{Interface: db.Logger}
				original := db.Logger
				db.Logger = meter
				defer func() { db.Logger = original }()
				svc := &InboundService{}
				run := func() {
					var inbounds []*model.Inbound
					var err error
					switch listMode {
					case "full":
						inbounds, err = svc.GetInbounds(1)
					case "slim":
						inbounds, err = svc.GetInboundsSlim(1)
					default:
						inbounds = make([]*model.Inbound, len(base))
						for i, ib := range base {
							copyInbound := *ib
							copyInbound.ClientStats = append([]xray.ClientTraffic(nil), ib.ClientStats...)
							inbounds[i] = &copyInbound
						}
						svc.enrichClientStats(db, inbounds)
					}
					if err != nil {
						b.Fatal(err)
					}
					count := 0
					for _, ib := range inbounds {
						count += len(ib.ClientStats)
						if listMode != "slim" {
							for _, st := range ib.ClientStats {
								if st.UUID == "" {
									b.Fatalf("missing identity for %s", st.Email)
								}
							}
						}
					}
					if len(inbounds) != 8 || count != attached {
						b.Fatalf("list returned inbounds=%d stats=%d, want 8/%d", len(inbounds), count, attached)
					}
				}
				run()
				meter.identities.Store(0)
				meter.traffic.Store(0)
				meter.elapsed.Store(0)
				var before statsPGSnapshot
				postgres := db.Name() == "postgres"
				if postgres {
					before = snapshotStatsPG(b)
				}
				b.ReportAllocs()
				b.ResetTimer()
				b.StartTimer()
				for range b.N {
					run()
				}
				b.StopTimer()
				b.ReportMetric(float64(meter.identities.Load())/float64(b.N), "identity-queries/op")
				b.ReportMetric(float64(meter.traffic.Load())/float64(b.N), "traffic-queries/op")
				b.ReportMetric(float64(meter.elapsed.Load())/float64(b.N), "sql-ns/op")
				if postgres {
					after := snapshotStatsPG(b)
					b.Logf("PG delta N=%d iterations=%d warmup=1 calls=%.0f rows=%.0f total_exec_time_ms=%.3f wal_records=%.0f wal_fpi=%.0f wal_bytes=%.0f shared_blks_hit=%.0f shared_blks_read=%.0f shared_blks_dirtied=%.0f", n, b.N, after.Calls-before.Calls, after.Rows-before.Rows, after.TotalExecTime-before.TotalExecTime, after.WalRecords-before.WalRecords, after.WalFpi-before.WalFpi, after.WalBytes-before.WalBytes, after.SharedBlksHit-before.SharedBlksHit, after.SharedBlksRead-before.SharedBlksRead, after.SharedBlksDirtied-before.SharedBlksDirtied)
					var info struct {
						StatsReset string
						Dealloc    int64
					}
					if err := db.Raw("SELECT stats_reset::text, dealloc FROM public.pg_stat_statements_info").Scan(&info).Error; err != nil {
						b.Fatal(err)
					}
					b.Logf("PG stats_reset=%s dealloc=%d", info.StatsReset, info.Dealloc)
				}
			})
		}
	}
}

func BenchmarkInboundClientStats(b *testing.B) { runStatsBenchmark(b, "") }

func BenchmarkInboundListStats(b *testing.B) {
	for _, mode := range []string{"full", "slim"} {
		b.Run(mode, func(b *testing.B) { runStatsBenchmark(b, mode) })
	}
}
