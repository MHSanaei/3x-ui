package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// pgSchemaDSN gives the test its own schema on XUI_TEST_PG_DSN.
func pgSchemaDSN(t *testing.T) string {
	t.Helper()
	base := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if base == "" {
		t.Skip("set XUI_TEST_PG_DSN to a reachable Postgres to run this test")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	schema := "xui_ro_" + hex.EncodeToString(suffix)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseDB()
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = conn.Close(context.Background())
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("XUI_TEST_PG_DSN must be a URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// An HA standby sits on a hot-standby replica: opening the database there
// must not write, while the migrations of InitDB must stay master-only.
func TestOpenDBDoesNotWriteOnReadOnlyServer(t *testing.T) {
	dsn := pgSchemaDSN(t)
	t.Setenv("XUI_DB_TYPE", "postgres")
	t.Setenv("XUI_DB_DSN", dsn)
	if err := InitDB(""); err != nil {
		t.Fatalf("migrate on the primary: %v", err)
	}

	u, _ := url.Parse(dsn)
	q := u.Query()
	q.Set("default_transaction_read_only", "on")
	u.RawQuery = q.Encode()
	t.Setenv("XUI_DB_DSN", u.String())

	if err := OpenDB(""); err != nil {
		t.Fatalf("OpenDB on a read-only server: %v", err)
	}
	var seeders int64
	if err := GetDB().Table("history_of_seeders").Count(&seeders).Error; err != nil || seeders == 0 {
		t.Fatalf("reads of the migrated data must work after OpenDB: seeders=%d err=%v", seeders, err)
	}
	err := InitDB("")
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("InitDB writes on every start, so it must fail on a read-only server; got %v", err)
	}
}
