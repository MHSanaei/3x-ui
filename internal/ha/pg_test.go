package ha

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalPGOnSinglePrimary(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to a reachable Postgres to run this test")
	}
	ctx := context.Background()
	pg, err := OpenLocalPG(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)

	rec, err := pg.IsInRecovery(ctx)
	if err != nil || rec {
		t.Fatalf("a standalone server is a primary: recovery=%v err=%v", rec, err)
	}
	names, err := pg.StreamingStandbys(ctx, 15*time.Second)
	if err != nil || len(names) != 0 {
		t.Fatalf("no standbys stream from a standalone server: %v %v", names, err)
	}
}

func TestLocalPGRefusesUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pg, err := OpenLocalPG(ctx, "postgres://x:y@127.0.0.1:1/db?connect_timeout=1")
	if err != nil {
		t.Fatalf("opening is lazy, the probe reports the error: %v", err)
	}
	defer pg.Close()
	if _, err := pg.IsInRecovery(ctx); err == nil {
		t.Fatal("an unreachable server must surface as an error (role unknown)")
	}
}
