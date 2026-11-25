package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

type lifecycleBenchMetrics struct {
	statements int64
	updates    int64
	rows       int64
	txTime     time.Duration
}

type lifecycleBenchPool struct {
	*sql.DB
	metrics *lifecycleBenchMetrics
}

func (p *lifecycleBenchPool) GetDBConn() (*sql.DB, error) { return p.DB, nil }

func (p *lifecycleBenchPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	if p.metrics == nil {
		return p.DB.BeginTx(ctx, opts)
	}
	start := time.Now()
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &lifecycleBenchTx{Tx: tx, start: start, metrics: p.metrics}, nil
}

func (p *lifecycleBenchPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if p.metrics != nil {
		p.metrics.statements++
	}
	return p.DB.QueryContext(ctx, query, args...)
}

func (p *lifecycleBenchPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if p.metrics != nil {
		p.metrics.statements++
	}
	return p.DB.QueryRowContext(ctx, query, args...)
}

func (p *lifecycleBenchPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if p.metrics != nil {
		p.metrics.statements++
	}
	return p.DB.ExecContext(ctx, query, args...)
}

type lifecycleBenchTx struct {
	*sql.Tx
	start   time.Time
	metrics *lifecycleBenchMetrics
}

func (t *lifecycleBenchTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	t.metrics.statements++
	return t.Tx.QueryContext(ctx, query, args...)
}

func (t *lifecycleBenchTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	t.metrics.statements++
	return t.Tx.QueryRowContext(ctx, query, args...)
}

func (t *lifecycleBenchTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	t.metrics.statements++
	result, err := t.Tx.ExecContext(ctx, query, args...)
	if err == nil && strings.HasPrefix(query, `UPDATE "client_traffics" SET "email"=`) {
		n, err := result.RowsAffected()
		if err != nil {
			return result, err
		}
		t.metrics.updates++
		t.metrics.rows += n
	}
	return result, err
}

func (t *lifecycleBenchTx) Commit() error {
	err := t.Tx.Commit()
	t.metrics.txTime += time.Since(t.start)
	return err
}

func BenchmarkInboundTrafficLifecycleEdit(b *testing.B) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		b.Skip("requires a dedicated disposable PostgreSQL server for WAL measurement")
	}
	for _, size := range []int{1000, 10000, 20000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			cleanup, err := testpg.IsolatePackage("inbound_lifecycle_bench")
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(cleanup)
			setupBulkDB(b)
			db := database.GetDB()
			sqlDB, err := db.DB()
			if err != nil {
				b.Fatal(err)
			}
			clients := make([]model.Client, size)
			for i := range clients {
				clients[i] = model.Client{
					Email: fmt.Sprintf("fixture-%05d@x", i), ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1),
					Enable: true, TotalGB: 1 << 30,
				}
			}
			svc := &InboundService{}
			inbound := seedTrafficLifecycleInbound(b, svc, clients)
			pool := &lifecycleBenchPool{DB: sqlDB}
			original := db.Statement.ConnPool
			db.Statement.ConnPool = pool
			defer func() { db.Statement.ConnPool = original }()
			for _, percent := range []int{0, 10, 100} {
				b.Run(fmt.Sprintf("changed-%d", percent), func(b *testing.B) {
					benchmarkTrafficLifecycleEdits(b, svc, inbound, pool, size, size*percent/100)
				})
			}
		})
	}
}

func benchmarkTrafficLifecycleEdits(b *testing.B, svc *InboundService, inbound *model.Inbound, pool *lifecycleBenchPool, size, changed int) {
	b.Helper()
	prepare := func() {
		b.Helper()
		if err := database.GetDB().Exec("UPDATE client_traffics SET total = 0 WHERE id IN (SELECT id FROM client_traffics ORDER BY id LIMIT ?)", changed).Error; err != nil {
			b.Fatal(err)
		}
	}
	edit := func(remark string) {
		b.Helper()
		form := *inbound
		form.Remark = remark
		if _, _, err := svc.UpdateInbound(&form); err != nil {
			b.Fatal(err)
		}
	}
	// Warm the exact code path and then restore drift before each timed edit.
	prepare()
	edit("warm-up")
	var totals lifecycleBenchMetrics
	var walBytes int64
	iteration := 0
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		prepare()
		var lsn string
		if err := pool.DB.QueryRowContext(b.Context(), "SELECT pg_current_wal_insert_lsn()::text").Scan(&lsn); err != nil {
			b.Fatal(err)
		}
		iteration++
		remark := fmt.Sprintf("edit-%d", iteration)
		var metrics lifecycleBenchMetrics
		pool.metrics = &metrics
		b.StartTimer()
		edit(remark)
		b.StopTimer()
		pool.metrics = nil
		if metrics.txTime <= 0 || metrics.updates != int64(size) || (metrics.rows != int64(changed) && metrics.rows != int64(size)) {
			b.Fatalf("inbound edit at %d/%d changed rows: %+v", changed, size, metrics)
		}
		var wal int64
		if err := pool.DB.QueryRowContext(b.Context(), "SELECT pg_wal_lsn_diff(pg_current_wal_insert_lsn(), $1::pg_lsn)::bigint", lsn).Scan(&wal); err != nil {
			b.Fatal(err)
		}
		walBytes += wal
		totals.statements += metrics.statements
		totals.rows += metrics.rows
		totals.txTime += metrics.txTime
		b.StartTimer()
	}
	b.ReportMetric(float64(totals.statements)/float64(b.N), "sql/op")
	b.ReportMetric(float64(totals.rows)/float64(b.N), "traffic-rows/op")
	b.ReportMetric(float64(walBytes)/float64(b.N), "wal-B/op")
	b.ReportMetric(float64(totals.txTime.Microseconds())/float64(b.N), "tx-us/op")
}
