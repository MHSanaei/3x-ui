package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	noderuntime "github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func snapshotSourceFixture(t *testing.T, n, inbounds int) (*gorm.DB, scaleDataset) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "node.db")
	t.Setenv("XUI_SCALE_DB_PATH", path)
	setupScaleDB(t)
	ds := seedScaleDataset(t, n, inbounds)
	source := database.GetDB()
	shareSnapshotScaleClients(t, source, ds, n, inbounds)
	reader, err := gorm.Open(source.Dialector, &gorm.Config{Logger: logger.Discard, DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := reader.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	// The node has its own connection pool; replacing the global DB below opens the master.
	if source.Name() == database.DialectPostgres {
		schema := fmt.Sprintf("snapshot_master_%d", time.Now().UnixNano())
		if err := reader.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reader.Exec("DROP SCHEMA " + schema + " CASCADE") })
		t.Setenv("XUI_DB_DSN", os.Getenv("XUI_DB_DSN")+" search_path="+schema)
	}
	if err := database.InitDB(filepath.Join(t.TempDir(), "master.db")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.EnsureFirstUser(); err != nil {
		t.Fatal(err)
	}
	return reader, ds
}

func seedSnapshotMaster(t *testing.T, source *gorm.DB, nodeCount int) {
	t.Helper()
	db := database.GetDB()
	var inbounds []model.Inbound
	var records []model.ClientRecord
	var links []model.ClientInbound
	var stats []xray.ClientTraffic
	for _, step := range []struct {
		table string
		dst   any
	}{{"inbounds", &inbounds}, {"clients", &records}, {"client_inbounds", &links}, {"client_traffics", &stats}} {
		if err := source.Table(step.table).Find(step.dst).Error; err != nil {
			t.Fatal(err)
		}
	}
	sourceRecordIDs := make([]int, len(records))
	for i := range records {
		sourceRecordIDs[i] = records[i].Id
		records[i].Id = 0
	}
	if err := db.CreateInBatches(&records, 500).Error; err != nil {
		t.Fatal(err)
	}
	clientIDs := make(map[int]int, len(records))
	for i := range records {
		clientIDs[sourceRecordIDs[i]] = records[i].Id
	}
	for node := 1; node <= nodeCount; node++ {
		nid := node
		seedNodeRow(t, db, &model.Node{Id: node, Name: fmt.Sprint("bench-", node), Address: "127.0.0.1", Port: 2096, ApiToken: "token", Enable: true, Guid: fmt.Sprint("bench-guid-", node)})
		mapping := map[int]int{}
		for _, ib := range inbounds {
			old := ib.Id
			ib.Id = 0
			ib.NodeID = &nid
			ib.OriginNodeGuid = fmt.Sprint("bench-guid-", node)
			ib.Tag = fmt.Sprintf("n%d-%s", node, ib.Tag)
			if err := db.Create(&ib).Error; err != nil {
				t.Fatal(err)
			}
			mapping[old] = ib.Id
		}
		localLinks := append([]model.ClientInbound(nil), links...)
		for i := range localLinks {
			localLinks[i].InboundId = mapping[localLinks[i].InboundId]
			localLinks[i].ClientId = clientIDs[localLinks[i].ClientId]
		}
		if err := db.CreateInBatches(localLinks, 1000).Error; err != nil {
			t.Fatal(err)
		}
		if node == 1 {
			for i := range stats {
				stats[i].Id = 0
				stats[i].InboundId = mapping[stats[i].InboundId]
				stats[i].Up = 0
				stats[i].Down = 0
			}
			if err := db.CreateInBatches(stats, 1000).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Opt-in scale run exercises HTTP, Remote decoding and the real serialized traffic writer.
func TestNodeSnapshotCycleScale(t *testing.T) {
	for _, n := range scaleSizes(t, 1000, 10000, 20000) {
		for _, inbounds := range []int{1, 8} {
			for _, nodes := range []int{1, 2} {
				t.Run(fmt.Sprintf("%d/%d/%d", n, inbounds, nodes), func(t *testing.T) {
					source, _ := snapshotSourceFixture(t, n, inbounds)
					db := database.GetDB()
					for _, mode := range []string{"legacy", "compact"} {
						if selected := os.Getenv("XUI_SNAPSHOT_MODE"); selected != "" && mode != selected {
							continue
						}
						resetScaleTables(t, db, "inbounds", "clients", "client_inbounds", "client_traffics", "nodes", "node_client_traffics")
						if err := source.Exec("UPDATE client_traffics SET up = 0, down = 0").Error; err != nil {
							t.Fatal(err)
						}
						seedSnapshotMaster(t, source, nodes)
						svc := &InboundService{}
						nodeSQL := &snapshotQueryMeter{Interface: logger.Discard}
						masterSQL := &snapshotQueryMeter{Interface: logger.Discard}
						oldLogger := db.Logger
						db.Logger = masterSQL
						var bytes, full, encodeNanos atomic.Int64
						var servers []*httptest.Server
						var remotes []*noderuntime.Remote
						for node := 1; node <= nodes; node++ {
							guid := fmt.Sprint("bench-guid-", node)
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
								if mode == "compact" {
									w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
								}
								measured := source.Session(&gorm.Session{Logger: nodeSQL})
								var value any
								var err error
								switch req.URL.Path {
								case "/panel/api/inbounds/list":
									var rows []*model.Inbound
									rows, err = svc.getInboundsFromDB(measured, 1)
									for _, ib := range rows {
										ib.OriginNodeGuid = guid
									}
									value = rows
									full.Add(1)
								case "/panel/api/inbounds/trafficSnapshot":
									var snap *noderuntime.NodeTrafficSnapshotV1
									snap, err = nodeTrafficSnapshot(measured, 1, req.URL.Query().Get("revision"), guid)
									value = snap
									if err == nil && snap.Structure != nil {
										full.Add(1)
									}
								case "/panel/api/clients/lastOnline":
									value, err = clientsLastOnline(measured)
								case "/panel/api/clients/onlines":
									value = []string{}
								default:
									value = map[string][]string{}
								}
								if err != nil {
									t.Errorf("snapshot HTTP: %v", err)
									http.Error(w, "snapshot failed", http.StatusInternalServerError)
									return
								}
								start := time.Now()
								wire, err := json.Marshal(struct {
									Success bool `json:"success"`
									Obj     any  `json:"obj"`
								}{true, value})
								encodeNanos.Add(time.Since(start).Nanoseconds())
								if err != nil {
									t.Error(err)
									return
								}
								bytes.Add(int64(len(wire)))
								_, _ = w.Write(wire)
							}))
							servers = append(servers, server)
							host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
							number, _ := strconv.Atoi(port)
							remotes = append(remotes, noderuntime.NewRemote(&model.Node{Id: node, Address: host, Port: number, AllowPrivateAddress: true, Scheme: "http", BasePath: "/", ApiToken: "token"}, nil))
						}
						cycle := func() {
							snapshots := make([]*noderuntime.TrafficSnapshot, nodes)
							errors := make([]error, nodes)
							var wg sync.WaitGroup
							for i, remote := range remotes {
								wg.Go(func() { snapshots[i], errors[i] = remote.FetchTrafficSnapshot(context.Background()) })
							}
							wg.Wait()
							for i, snap := range snapshots {
								if errors[i] != nil {
									t.Fatal(errors[i])
								}
								if _, err := svc.SetRemoteTraffic(i+1, snap, false, false); err != nil {
									t.Fatal(err)
								}
							}
						}
						for range 5 {
							cycle()
						}
						measurePG := source.Name() == database.DialectPostgres && os.Getenv("XUI_SNAPSHOT_PG_STATS") == "1"
						for _, scenario := range []string{"idle", "counters"} {
							if measurePG {
								if err := source.Exec("SELECT public.pg_stat_statements_reset()").Error; err != nil {
									t.Fatal(err)
								}
							}
							beforeStructural := masterSQL.fullSettingsReads
							beforeMasterRead, beforeMasterWritten := masterSQL.readRows, masterSQL.writtenRows
							beforeNodeCount, beforeNodeRows, beforeNodeTime := nodeSQL.count, nodeSQL.rows, nodeSQL.elapsed
							beforeMasterCount, beforeMasterRows, beforeMasterTime := masterSQL.count, masterSQL.rows, masterSQL.elapsed
							beforeBytes, beforeFull, beforeEncode := bytes.Load(), full.Load(), encodeNanos.Load()
							var profile *os.File
							if dir := os.Getenv("XUI_SNAPSHOT_PROFILE_DIR"); dir != "" {
								if err := os.MkdirAll(dir, 0o700); err != nil {
									t.Fatal(err)
								}
								var err error
								profile, err = os.Create(filepath.Join(dir, fmt.Sprintf("%s-%s-%d-%d-%d.cpu", mode, scenario, n, inbounds, nodes)))
								if err != nil {
									t.Fatal(err)
								}
								if err := pprof.StartCPUProfile(profile); err != nil {
									t.Fatal(err)
								}
							}
							var times []float64
							var allocated, allocs uint64
							for range 12 {
								if scenario == "counters" {
									if err := source.Exec("/*snapshot-fixture*/ UPDATE client_traffics SET up = up + 100, down = down + 200 WHERE id IN (SELECT id FROM client_traffics ORDER BY id LIMIT 100)").Error; err != nil {
										t.Fatal(err)
									}
								}
								runtime.GC()
								var before, after runtime.MemStats
								runtime.ReadMemStats(&before)
								start := time.Now()
								cycle()
								times = append(times, float64(time.Since(start).Microseconds())/1000)
								runtime.ReadMemStats(&after)
								allocated += after.TotalAlloc - before.TotalAlloc
								allocs += after.Mallocs - before.Mallocs
							}
							if profile != nil {
								pprof.StopCPUProfile()
								_ = profile.Close()
							}
							sort.Float64s(times)
							serverMS := 0.0
							serverMetric := "n/a"
							if measurePG {
								if err := source.Raw("SELECT COALESCE(SUM(total_exec_time),0) FROM public.pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND query NOT LIKE '%pg_stat_statements%' AND query NOT LIKE '%snapshot-fixture%'").Scan(&serverMS).Error; err != nil {
									t.Fatal(err)
								}
								serverMS /= 12
								serverMetric = fmt.Sprintf("%.2f", serverMS)
							}
							t.Logf("CYCLE scenario=%s mode=%s n=%d inbounds=%d nodes=%d samples=12 node_sql=%.1f node_rows=%.0f node_query_scan_ms=%.2f master_sql=%.1f master_rows=%.0f master_rows_read=%.0f master_rows_affected=%.0f master_query_scan_ms=%.2f bytes=%d encode_ms=%.2f allocs=%d allocated=%d full_fetches=%d structural_polls=%d pg_exec_ms=%s p50_ms=%.2f p95_ms=%.2f", scenario, mode, n, inbounds, nodes, float64(nodeSQL.count-beforeNodeCount)/12, float64(nodeSQL.rows-beforeNodeRows)/12, float64((nodeSQL.elapsed-beforeNodeTime).Microseconds())/12000, float64(masterSQL.count-beforeMasterCount)/12, float64(masterSQL.rows-beforeMasterRows)/12, float64(masterSQL.readRows-beforeMasterRead)/12, float64(masterSQL.writtenRows-beforeMasterWritten)/12, float64((masterSQL.elapsed-beforeMasterTime).Microseconds())/12000, (bytes.Load()-beforeBytes)/12, float64(encodeNanos.Load()-beforeEncode)/12000000, allocs/12, allocated/12, full.Load()-beforeFull, masterSQL.fullSettingsReads-beforeStructural, serverMetric, times[5], times[11])
						}

						var eventInbound model.Inbound
						if err := source.Order("id").First(&eventInbound).Error; err != nil {
							t.Fatal(err)
						}
						var config struct {
							Clients []model.Client `json:"clients"`
						}
						if err := json.Unmarshal([]byte(eventInbound.Settings), &config); err != nil {
							t.Fatal(err)
						}
						added := model.Client{Email: "snapshot-event-added", ID: "00000000-0000-4000-8000-000000000777", Enable: true}
						for _, event := range []string{"add", "remove", "config"} {
							if err := source.Transaction(func(tx *gorm.DB) error {
								switch event {
								case "add":
									record := added.ToRecord()
									if err := tx.Create(record).Error; err != nil {
										return err
									}
									if err := tx.Create(&model.ClientInbound{ClientId: record.Id, InboundId: eventInbound.Id}).Error; err != nil {
										return err
									}
									if err := tx.Create(&xray.ClientTraffic{Email: added.Email, InboundId: eventInbound.Id, Enable: true}).Error; err != nil {
										return err
									}
									return tx.Model(&model.Inbound{}).Where("id = ?", eventInbound.Id).Update("settings", clientsSettings(t, append(append([]model.Client(nil), config.Clients...), added))).Error
								case "remove":
									if err := tx.Where("client_id IN (?)", tx.Model(&model.ClientRecord{}).Select("id").Where("email = ?", added.Email)).Delete(&model.ClientInbound{}).Error; err != nil {
										return err
									}
									if err := tx.Where("email = ?", added.Email).Delete(&model.ClientRecord{}).Error; err != nil {
										return err
									}
									if err := tx.Where("email = ?", added.Email).Delete(&xray.ClientTraffic{}).Error; err != nil {
										return err
									}
									return tx.Model(&model.Inbound{}).Where("id = ?", eventInbound.Id).Update("settings", eventInbound.Settings).Error
								default:
									return tx.Model(&model.Inbound{}).Where("id = ?", eventInbound.Id).Update("remark", eventInbound.Remark+" changed").Error
								}
							}); err != nil {
								t.Fatal(err)
							}
							previousFull, previousBytes := full.Load(), bytes.Load()
							previousSQL := nodeSQL.count + masterSQL.count
							start := time.Now()
							cycle()
							if full.Load()-previousFull != int64(nodes) {
								t.Fatalf("%s did not refresh every affected peer: full=%d", event, full.Load()-previousFull)
							}
							t.Logf("EVENT mode=%s n=%d inbounds=%d nodes=%d change=%s latency_ms=%.2f bytes=%d sql=%d full_fetches=%d", mode, n, inbounds, nodes, event, float64(time.Since(start).Microseconds())/1000, bytes.Load()-previousBytes, nodeSQL.count+masterSQL.count-previousSQL, full.Load()-previousFull)
						}
						if err := source.Model(&model.Inbound{}).Where("id = ?", eventInbound.Id).Update("remark", eventInbound.Remark).Error; err != nil {
							t.Fatal(err)
						}

						for _, server := range servers {
							server.Close()
						}
						db.Logger = oldLogger
					}
				})
			}
		}
	}
}
