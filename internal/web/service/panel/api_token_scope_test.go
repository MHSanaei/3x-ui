package panel

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func setupAPITokenTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatalf("CloseDB: %v", err)
		}
	})
}

func TestAPITokenScopeExpiryAndExpectedRevoke(t *testing.T) {
	setupAPITokenTestDB(t)
	svc := &ApiTokenService{}

	future := time.Now().Add(time.Hour).UnixMilli()
	created, err := svc.Create("node-1", model.ApiScopeNodeSync, future)
	if err != nil {
		t.Fatalf("create node-sync token: %v", err)
	}
	row, ok := svc.MatchToken(created.Token)
	if !ok {
		t.Fatal("fresh node-sync token did not match")
	}
	if row.Scope != model.ApiScopeNodeSync || row.ExpiresAt != future {
		t.Fatalf("matched row scope/expiry = %s/%d, want node-sync/%d", row.Scope, row.ExpiresAt, future)
	}

	if _, err := svc.Create("bad-scope", "superuser", 0); err == nil {
		t.Fatal("unknown scope must be rejected on create")
	}
	if _, err := svc.Create("past", model.ApiScopeAdmin, time.Now().Add(-time.Minute).UnixMilli()); err == nil {
		t.Fatal("past expiry must be rejected on create")
	}

	const expiredPlain = "expired-token"
	if err := database.GetDB().Create(&model.ApiToken{
		Name:      "expired",
		Token:     crypto.HashTokenSHA256(expiredPlain),
		Enabled:   true,
		Scope:     model.ApiScopeAdmin,
		ExpiresAt: time.Now().Add(-time.Minute).UnixMilli(),
	}).Error; err != nil {
		t.Fatalf("seed expired token: %v", err)
	}
	if _, ok := svc.MatchToken(expiredPlain); ok {
		t.Fatal("expired token must fail closed")
	}

	const unknownPlain = "unknown-scope-token"
	if err := database.GetDB().Create(&model.ApiToken{
		Name:    "unknown",
		Token:   crypto.HashTokenSHA256(unknownPlain),
		Enabled: true,
		Scope:   "root",
	}).Error; err != nil {
		t.Fatalf("seed unknown-scope token: %v", err)
	}
	if _, ok := svc.MatchToken(unknownPlain); ok {
		t.Fatal("unknown token scope must fail closed")
	}

	if err := svc.DisableExpectedScope(created.Id, model.ApiScopeAdmin); err == nil {
		t.Fatal("expected-scope revoke must refuse a node-sync token when admin was expected")
	}
	if _, ok := svc.MatchToken(created.Token); !ok {
		t.Fatal("wrong expected-scope revoke disabled the token")
	}
	if err := svc.DisableExpectedScope(created.Id, model.ApiScopeNodeSync); err != nil {
		t.Fatalf("disable expected node-sync token: %v", err)
	}
	if _, ok := svc.MatchToken(created.Token); ok {
		t.Fatal("disabled token still matched")
	}
}

func TestAPITokenAdditiveDefaultsPreserveLegacyAccess(t *testing.T) {
	setupAPITokenTestDB(t)
	const plaintext = "legacy-token"
	if err := database.GetDB().Exec(
		"INSERT INTO api_tokens (name, token, enabled, created_at) VALUES (?, ?, ?, ?)",
		"legacy", crypto.HashTokenSHA256(plaintext), true, time.Now().Unix(),
	).Error; err != nil {
		t.Fatalf("insert legacy-shaped token: %v", err)
	}
	row, ok := (&ApiTokenService{}).MatchToken(plaintext)
	if !ok {
		t.Fatal("legacy-shaped token no longer authenticates")
	}
	if row.Scope != model.ApiScopeAdmin || row.ExpiresAt != 0 {
		t.Fatalf("legacy defaults = scope %q expiry %d, want admin/0", row.Scope, row.ExpiresAt)
	}
}
