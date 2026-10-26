package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func BenchmarkClientDepletion(b *testing.B) {
	benchmarkClientDepletion(b, false)
}

func BenchmarkClientDepletionWithReports(b *testing.B) {
	benchmarkClientDepletion(b, true)
}

func benchmarkClientDepletion(b *testing.B, reports bool) {
	for _, n := range []int{1000, 5000, 20000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			dbtest.InitDB(b, filepath.Join(b.TempDir(), "bench.db"))
			db := database.GetDB()
			rows := make([]xray.ClientTraffic, n)
			for i := range rows {
				rows[i] = xray.ClientTraffic{Email: fmt.Sprintf("bench-%d", i), Enable: true, Up: 1, Total: 1000}
			}
			if err := db.CreateInBatches(&rows, 400).Error; err != nil {
				b.Fatal(err)
			}
			if reports {
				globals := make([]model.ClientGlobalTraffic, n)
				nodes := make([]model.NodeClientTraffic, n)
				for i, row := range rows {
					globals[i] = model.ClientGlobalTraffic{MasterGuid: "master", Email: row.Email, Up: 2, Down: 2, UpdatedAt: time.Now().UnixMilli()}
					nodes[i] = model.NodeClientTraffic{NodeId: 1, Email: row.Email, Up: 1, Down: 1}
				}
				if err := db.CreateInBatches(&globals, 200).Error; err != nil {
					b.Fatal(err)
				}
				if err := db.CreateInBatches(&nodes, 200).Error; err != nil {
					b.Fatal(err)
				}
			}
			deltas := make([]*xray.ClientTraffic, n)
			for i, row := range rows {
				deltas[i] = &xray.ClientTraffic{Email: row.Email, Up: 1}
			}
			for _, mode := range []string{"idle poll", "traffic response", "active counters"} {
				b.Run(mode, func(b *testing.B) {
					var queries, writes int64
					const key = "depletion-benchmark"
					if err := db.Callback().Query().After("gorm:query").Register(key, func(*gorm.DB) { queries++ }); err != nil {
						b.Fatal(err)
					}
					if err := db.Callback().Update().After("gorm:update").Register(key, func(*gorm.DB) { writes++ }); err != nil {
						b.Fatal(err)
					}
					if err := db.Callback().Raw().After("gorm:raw").Register(key, func(tx *gorm.DB) {
						if strings.HasPrefix(tx.Statement.SQL.String(), "UPDATE ") {
							writes++
						}
					}); err != nil {
						b.Fatal(err)
					}
					b.Cleanup(func() {
						_ = db.Callback().Query().Remove(key)
						_ = db.Callback().Update().Remove(key)
						_ = db.Callback().Raw().Remove(key)
					})
					svc := &InboundService{}
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						switch mode {
						case "idle poll":
							if _, _, _, err := svc.disableInvalidClients(db, newTrafficMutationBatch()); err != nil {
								b.Fatal(err)
							}
						case "active counters":
							if err := db.Transaction(func(tx *gorm.DB) error { return svc.addClientTraffic(tx, deltas) }); err != nil {
								b.Fatal(err)
							}
						default:
							got, err := svc.GetAllClientTraffics()
							if err != nil || len(got) != n {
								b.Fatalf("rows=%d, error=%v", len(got), err)
							}
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(queries)/float64(b.N), "queries/op")
					b.ReportMetric(float64(writes)/float64(b.N), "updates/op")
				})
			}
		})
	}
}

func TestIdleDepletionPollDoesNotWrite(t *testing.T) {
	db := initDepletionTestDB(t)
	if err := db.Create(&xray.ClientTraffic{Email: "healthy-idle", Enable: true, Total: 100, Up: 1}).Error; err != nil {
		t.Fatal(err)
	}
	writes := 0
	const key = "idle-depletion-write"
	if err := db.Callback().Update().After("gorm:update").Register(key, func(*gorm.DB) { writes++ }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(key) })
	if _, count, _, err := (&InboundService{}).disableInvalidClients(db, newTrafficMutationBatch()); err != nil || count != 0 {
		t.Fatalf("disabled=%d, error=%v", count, err)
	}
	if writes != 0 {
		t.Fatalf("healthy idle poll issued %d UPDATEs, want none", writes)
	}
}

func TestIdleNodeSnapshotDoesNotSweepUnrelatedDepletion(t *testing.T) {
	db := initDepletionTestDB(t)
	createNodeInbound(t, db, 1, "idle-node", 41001)
	svc := &InboundService{}
	cs := xray.ClientTraffic{Email: "idle-node", Enable: true, Total: 100}
	syncNode(t, svc, 1, "idle-node", cs)
	other := xray.ClientTraffic{Email: "unreported", Up: 1, Total: 100, DepletedAt: 1700000000000}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	writes := 0
	const callback = "idle-node-metadata-write"
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "client_traffics" {
			writes++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	syncNode(t, svc, 1, "idle-node", cs)
	if writes != 0 {
		t.Fatalf("idle node snapshot issued %d traffic UPDATEs", writes)
	}
	if got := readTraffic(t, db, other.Email); got.DepletedAt != other.DepletedAt {
		t.Fatalf("idle snapshot swept an unrelated row: %+v", got)
	}
}
