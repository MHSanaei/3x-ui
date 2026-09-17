package testpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	dbTypeEnv = "XUI_DB_TYPE"
	dbDSNEnv  = "XUI_DB_DSN"
)

// IsolatePackage gives one go test package its own PostgreSQL schema. Go runs
// package test binaries concurrently, so sharing public lets independent
// migration suites race even when the database itself is disposable.
func IsolatePackage(packageName string) (func(), error) {
	if os.Getenv(dbTypeEnv) != "postgres" {
		return func() {}, nil
	}
	baseDSN := strings.TrimSpace(os.Getenv(dbDSNEnv))
	if baseDSN == "" {
		return func() {}, nil
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, fmt.Errorf("generate PostgreSQL test schema suffix: %w", err)
	}
	schema := fmt.Sprintf("xui_%s_%d_%s", sanitize(packageName), os.Getpid(), hex.EncodeToString(suffix))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, baseDSN)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL test database: %w", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		return nil, fmt.Errorf("create PostgreSQL test schema: %w", err)
	}
	isolatedDSN, err := withSearchPath(baseDSN, schema)
	if err != nil {
		admin.Close()
		return nil, err
	}
	if err := os.Setenv(dbDSNEnv, isolatedDSN); err != nil {
		admin.Close()
		return nil, fmt.Errorf("set isolated PostgreSQL test DSN: %w", err)
	}

	return func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
		_ = os.Setenv(dbDSNEnv, baseDSN)
	}, nil
}

func withSearchPath(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		query := u.Query()
		query.Set("search_path", schema)
		u.RawQuery = query.Encode()
		return u.String(), nil
	}
	if strings.ContainsAny(schema, " '[]=\\") {
		return "", fmt.Errorf("unsafe PostgreSQL test schema name")
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema, nil
}

func sanitize(value string) string {
	var result strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteByte('_')
		}
	}
	if result.Len() == 0 {
		return "pkg"
	}
	return result.String()
}
