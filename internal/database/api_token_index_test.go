package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The bearer-token lookup matches the presented token's SHA-256 hash against
// the stored hash. Without an index on that column it seq-scans every enabled
// token on every authenticated request.
func TestMigrateApiTokenTokenIndexIsIdempotentAndUsed(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if !db.Migrator().HasIndex(&model.ApiToken{}, "idx_api_tokens_token") {
		t.Fatal("idx_api_tokens_token missing after InitDB")
	}
	if err := migrateApiTokenTokenIndex(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !db.Migrator().HasIndex(&model.ApiToken{}, "idx_api_tokens_token") {
		t.Fatal("idx_api_tokens_token vanished after a second run")
	}

	if IsPostgres() {
		return
	}
	var plan []struct{ Detail string }
	if err := db.Raw("EXPLAIN QUERY PLAN SELECT * FROM api_tokens WHERE token = 'x' AND enabled = 1").
		Scan(&plan).Error; err != nil {
		t.Fatalf("explain: %v", err)
	}
	used := false
	for _, row := range plan {
		if strings.Contains(row.Detail, "idx_api_tokens_token") {
			used = true
		}
	}
	if !used {
		t.Errorf("the token-hash lookup does not use idx_api_tokens_token: %+v", plan)
	}
}
