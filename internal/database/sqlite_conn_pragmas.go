package database

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
)

// cache_size, mmap_size and temp_store are per-connection settings, but
// database/sql hands out pooled connections. Executing them once on the pool
// only configured the single connection the driver returned for that call, so
// XUI_DB_CACHE_MB and XUI_DB_MMAP_MB were effectively inert for every other
// connection - and for every connection handed out after ConnMaxLifetime
// recycled the tuned one. Applying them from the driver's ConnectHook covers
// each connection as it is opened.
const sqliteConnDriver = "sqlite3-xui-conn"

// sqliteConnPragmasMinKB keeps a divided-down cache budget usable; SQLite's own
// default page cache is 2 MiB.
const sqliteConnPragmasMinKB = 1024

var sqliteConnPragmas atomic.Pointer[[]string]

var sqliteConnDriverOnce sync.Once

// sqliteConnDriverName returns the driver name registered with a connect hook
// that installs the current connection PRAGMAs. The hook reads the pragmas at
// connect time, so reopening the database with a different budget reconfigures
// connections opened afterwards.
func sqliteConnDriverName() string {
	sqliteConnDriverOnce.Do(func() {
		sql.Register(sqliteConnDriver, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				pragmas := sqliteConnPragmas.Load()
				if pragmas == nil {
					return nil
				}
				for _, pragma := range *pragmas {
					if _, err := conn.Exec(pragma, nil); err != nil {
						return fmt.Errorf("apply %s: %w", pragma, err)
					}
				}
				return nil
			},
		})
	})
	return sqliteConnDriver
}

func setSQLiteConnPragmas(pragmas []string) {
	applied := append([]string(nil), pragmas...)
	sqliteConnPragmas.Store(&applied)
}

// sqliteConnPragmaStatements builds the per-connection PRAGMA list. The page
// cache budget is divided across the pool instead of being granted to each
// connection, so raising XUI_DB_MAX_OPEN_CONNS does not multiply memory use.
func sqliteConnPragmaStatements(sync string, maxOpen int) []string {
	if maxOpen < 1 {
		maxOpen = 1
	}
	cacheKB := envInt("XUI_DB_CACHE_MB", 32) * 1024 / maxOpen
	if cacheKB < sqliteConnPragmasMinKB {
		cacheKB = sqliteConnPragmasMinKB
	}
	return []string{
		"PRAGMA busy_timeout=10000",
		"PRAGMA synchronous=" + sync,
		fmt.Sprintf("PRAGMA cache_size=-%d", cacheKB),
		fmt.Sprintf("PRAGMA mmap_size=%d", int64(envInt("XUI_DB_MMAP_MB", 256))*1024*1024),
		"PRAGMA temp_store=MEMORY",
	}
}
