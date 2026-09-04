package main

// GetApiToken rotates a credential rather than displaying one, so these pin
// which token name it destroys — the whole point of the -tokenName flag.

import (
	"flag"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

func tokenRow(t *testing.T, name string) model.ApiToken {
	t.Helper()
	var row model.ApiToken
	if err := database.GetDB().Where("name = ?", name).First(&row).Error; err != nil {
		t.Fatalf("load token %q: %v", name, err)
	}
	return row
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

// An explicit name has to win on both branches, or the same command would
// produce ci-bot on a populated panel and "install" on a fresh one.
func TestGetApiTokenUsesGivenNameOnEmptyDatabase(t *testing.T) {
	newTokenCLIEnv(t)

	GetApiToken(true, "ci-bot")

	names := tokenNames(t)
	if !hasName(names, "ci-bot") {
		t.Fatalf("token names = %v, want ci-bot among them", names)
	}
	if hasName(names, installTokenName) {
		t.Fatalf("token names = %v, want no %s when a name was given", names, installTokenName)
	}
}

// install.sh records the token it gets on a fresh panel. A later bare
// -getApiToken must rotate the fallback slot and leave that record valid.
func TestGetApiTokenPreservesInstallTokenWhenRotating(t *testing.T) {
	newTokenCLIEnv(t)

	GetApiToken(true, "")
	installed := tokenRow(t, installTokenName)

	GetApiToken(true, "")

	names := tokenNames(t)
	if !hasName(names, cliFallbackTokenName) {
		t.Fatalf("token names = %v, want %s among them", names, cliFallbackTokenName)
	}
	if got := tokenRow(t, installTokenName); got.Id != installed.Id {
		t.Fatalf("%s row id = %d, want %d — the installer's token was replaced", installTokenName, got.Id, installed.Id)
	}
	if got := tokenRow(t, installTokenName); got.Token != installed.Token {
		t.Fatalf("the %s token hash changed, so the recorded credential stopped working", installTokenName)
	}
}

// `-getApiToken true -tokenName ci-bot` parses tokenName as "", because flag
// stops at the positional. The command must not then rotate the shared slot.
func TestGetApiTokenWarnsOnIgnoredPositionalArgs(t *testing.T) {
	set := flag.NewFlagSet("setting", flag.ContinueOnError)
	var getApiToken bool
	var tokenName string
	set.BoolVar(&getApiToken, "getApiToken", false, "")
	set.StringVar(&tokenName, "tokenName", "", "")

	if err := set.Parse([]string{"-getApiToken", "true", "-tokenName", "ci-bot"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tokenName != "" {
		t.Fatalf("tokenName = %q; this test guards the case where flag drops it", tokenName)
	}
	if got := set.Args(); len(got) == 0 {
		t.Fatal("leftover arguments must be visible so the CLI can warn instead of silently rotating cli-fallback")
	}
}

func TestGetApiTokenTrimsName(t *testing.T) {
	newTokenCLIEnv(t)

	if _, err := (&panel.ApiTokenService{}).RecreateByName("seed"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	GetApiToken(true, "   ")

	names := tokenNames(t)
	if !hasName(names, cliFallbackTokenName) {
		t.Fatalf("token names = %v, want a whitespace-only name to fall back to %s", names, cliFallbackTokenName)
	}
}
