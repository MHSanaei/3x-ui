package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	noderuntime "github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type snapshotQueryMeter struct {
	logger.Interface
	mu                    sync.Mutex
	count, rows           int64
	readRows, writtenRows int64
	fullSettingsReads     int64
	elapsed               time.Duration
}

func (m *snapshotQueryMeter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, rows := fc()
	if q == "BEGIN DEFERRED" || q == "COMMIT" || q == "ROLLBACK" {
		return
	}
	if strings.Contains(q, "SELECT * FROM `inbounds`") || strings.Contains(q, "SELECT * FROM \"inbounds\"") {
		m.fullSettingsReads++
	}
	m.count++
	m.rows += max(rows, 0)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(q)), "SELECT") {
		m.readRows += max(rows, 0)
	} else {
		m.writtenRows += max(rows, 0)
	}
	m.elapsed += time.Since(begin)
}

// TestNodeSnapshotScale compares identical local-counter fixtures; opt in with XUI_SCALE_TEST=1.
func TestNodeSnapshotScale(t *testing.T) {
	setupScaleDB(t)
	for _, n := range scaleSizes(t, 1000, 10000, 20000) {
		for _, inboundCount := range []int{1, 8} {
			t.Run(fmt.Sprintf("%d/%d", n, inboundCount), func(t *testing.T) {
				ds := seedScaleDataset(t, n, inboundCount)
				db := database.GetDB()
				shareSnapshotScaleClients(t, db, ds, n, inboundCount)
				svc := &InboundService{}
				for _, mode := range []string{"legacy", "compact"} {
					if os.Getenv("XUI_SNAPSHOT_BASELINE") == "1" && mode != "legacy" {
						continue
					}
					meter := &snapshotQueryMeter{Interface: logger.Discard}
					measured := db.Session(&gorm.Session{Logger: meter})
					revision := ""
					load := func() any {
						if mode == "legacy" {
							rows, err := svc.getInboundsFromDB(measured, 1)
							if err != nil {
								t.Fatal(err)
							}
							return rows
						}
						snap, err := nodeTrafficSnapshot(measured, 1, revision, "scale-node")
						if err != nil {
							t.Fatal(err)
						}
						revision = snap.Revision
						return snap
					}
					_ = load()
					var times []float64
					var bytes, allocs, allocated uint64
					var encode, decode time.Duration
					*meter = snapshotQueryMeter{Interface: logger.Discard}
					for range 12 {
						runtime.GC()
						var before, after runtime.MemStats
						runtime.ReadMemStats(&before)
						start := time.Now()
						value := load()
						at := time.Now()
						wire, err := json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
						encode += time.Since(at)
						at = time.Now()
						if mode == "legacy" {
							var dst []*model.Inbound
							if err := json.Unmarshal(wire, &dst); err != nil {
								t.Fatal(err)
							}
						} else {
							var dst noderuntime.NodeTrafficSnapshotV1
							if err := json.Unmarshal(wire, &dst); err != nil {
								t.Fatal(err)
							}
						}
						decode += time.Since(at)
						times = append(times, float64(time.Since(start).Microseconds())/1000)
						runtime.ReadMemStats(&after)
						bytes += uint64(len(wire))
						allocs += after.Mallocs - before.Mallocs
						allocated += after.TotalAlloc - before.TotalAlloc
					}
					sort.Float64s(times)
					t.Logf("SNAPSHOT mode=%s n=%d inbounds=%d samples=12 sql=%.1f rows=%.0f query_scan_ms=%.2f bytes=%d encode_ms=%.2f decode_ms=%.2f allocs=%d allocated=%d p50_ms=%.2f p95_ms=%.2f", mode, n, inboundCount, float64(meter.count)/12, float64(meter.rows)/12, float64(meter.elapsed.Microseconds())/12000, bytes/12, float64(encode.Microseconds())/12000, float64(decode.Microseconds())/12000, allocs/12, allocated/12, times[5], times[11])
				}
			})
		}
	}
}

func shareSnapshotScaleClients(t testing.TB, db *gorm.DB, ds scaleDataset, n, inbounds int) {
	t.Helper()
	if inbounds <= 1 {
		return
	}
	var links []model.ClientInbound
	if err := db.Find(&links).Error; err != nil {
		t.Fatal(err)
	}
	for i := range links {
		links[i].InboundId = ds.inboundIds[(i/(n/inbounds)+1)%inbounds]
	}
	if err := db.CreateInBatches(links, 1000).Error; err != nil {
		t.Fatal(err)
	}
	for i, id := range ds.inboundIds {
		clients := append(append([]model.Client(nil), ds.perInbound[i]...), ds.perInbound[(i+inbounds-1)%inbounds]...)
		if err := db.Model(&model.Inbound{}).Where("id = ?", id).Update("settings", clientsSettings(t, clients)).Error; err != nil {
			t.Fatal(err)
		}
	}
}
