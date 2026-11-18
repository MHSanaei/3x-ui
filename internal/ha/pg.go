package ha

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LocalPG probes the local PostgreSQL over its own tiny pool, so a saturated
// panel pool cannot stall the role decision.
type LocalPG struct {
	pool *pgxpool.Pool
}

// OpenLocalPG builds the probe pool; connections are made lazily.
func OpenLocalPG(ctx context.Context, dsn string) (*LocalPG, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("ha: parse database DSN: %w", err)
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ha: open probe pool: %w", err)
	}
	return &LocalPG{pool: pool}, nil
}

// Close releases the pool.
func (p *LocalPG) Close() { p.pool.Close() }

// IsInRecovery returns pg_is_in_recovery().
func (p *LocalPG) IsInRecovery(ctx context.Context) (bool, error) {
	var rec bool
	if err := p.pool.QueryRow(ctx, "SELECT pg_is_in_recovery()").Scan(&rec); err != nil {
		return false, fmt.Errorf("pg_is_in_recovery: %w", err)
	}
	return rec, nil
}

// StreamingStandbys lists standbys (Patroni names them by member) that replied
// within maxAge; needs pg_monitor to see other sessions in pg_stat_replication.
func (p *LocalPG) StreamingStandbys(ctx context.Context, maxAge time.Duration) ([]string, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT application_name FROM pg_stat_replication
		 WHERE state = 'streaming'
		   AND reply_time > now() - make_interval(secs => $1)
		 ORDER BY application_name`, maxAge.Seconds())
	if err != nil {
		return nil, fmt.Errorf("pg_stat_replication: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("pg_stat_replication: scan: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
