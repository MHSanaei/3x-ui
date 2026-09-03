package main

// GetApiToken rotates a credential rather than displaying one, so these pin
// which token name it destroys — the whole point of the -tokenName flag.

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
)

func newTokenCLIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func tokenNames(t *testing.T) []string {
	t.Helper()
	tokens, err := (&panel.ApiTokenService{}).List()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	names := make([]string, 0, len(tokens))
	for _, token := range tokens {
		names = append(names, token.Name)
	}
	return names
}

func hasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// The bug: two callers sharing one hardcoded slot silently revoke each other.
// A named token must leave an differently-named one authenticating.
func TestGetApiTokenRotatesOnlyTheNamedToken(t *testing.T) {
	newTokenCLIEnv(t)

	svc := panel.ApiTokenService{}
	weekly, err := svc.RecreateByName("weekly-report")
	if err != nil {
		t.Fatalf("seed weekly-report: %v", err)
	}

	GetApiToken(true, "ci-bot")

	names := tokenNames(t)
	if !hasName(names, "ci-bot") {
		t.Fatalf("token names = %v, want ci-bot among them", names)
	}
	if !svc.Match(weekly.Token) {
		t.Fatal("weekly-report was revoked by a call naming ci-bot")
	}
}

func TestGetApiTokenUsesGivenNameOnEmptyDatabase(t *testing.T) {
	newTokenCLIEnv(t)

	GetApiToken(true, "ci-bot")

	names := tokenNames(t)
	if !hasName(names, "ci-bot") {
		t.Fatalf("token names = %v, want ci-bot among them", names)
	}
}

// install.sh runs `x-ui setting -getApiToken true` with no name, on both a
// fresh and an already-populated database. Both must land on one slot.
func TestGetApiTokenDefaultsToFallbackName(t *testing.T) {
	newTokenCLIEnv(t)

	GetApiToken(true, "")
	names := tokenNames(t)
	if !hasName(names, cliFallbackTokenName) {
		t.Fatalf("token names = %v, want %s among them", names, cliFallbackTokenName)
	}

	GetApiToken(true, "")
	names = tokenNames(t)
	if len(names) != 1 || names[0] != cliFallbackTokenName {
		t.Fatalf("token names = %v, want exactly [%s] after a repeat call", names, cliFallbackTokenName)
	}
}

func TestGetApiTokenTrimsName(t *testing.T) {
	newTokenCLIEnv(t)

	GetApiToken(true, "   ")

	names := tokenNames(t)
	if !hasName(names, cliFallbackTokenName) {
		t.Fatalf("token names = %v, want a whitespace-only name to fall back to %s", names, cliFallbackTokenName)
	}
}
