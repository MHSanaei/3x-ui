package panel

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

var errInjectedTokenCreate = errors.New("injected token create failure")

func TestApiTokenCreatedAtSeconds(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		{name: "seconds", in: 1_782_485_394, want: 1_782_485_394},
		{name: "legacy milliseconds", in: 1_782_485_394_270, want: 1_782_485_394},
		{name: "unset", in: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apiTokenCreatedAtSeconds(tt.in); got != tt.want {
				t.Fatalf("apiTokenCreatedAtSeconds(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestRecreateByNamePreservesTokenWhenReplacementFails(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())

	svc := ApiTokenService{}
	first, err := svc.RecreateByName("cli-fallback")
	if err != nil {
		t.Fatalf("first recreate: %v", err)
	}
	db := database.GetDB()
	const callback = "test:fail-token-replacement"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if token, ok := tx.Statement.Dest.(*model.ApiToken); ok && token.Name == "cli-fallback" {
			tx.AddError(errInjectedTokenCreate)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })

	if _, err := svc.RecreateByName("cli-fallback"); !errors.Is(err, errInjectedTokenCreate) {
		t.Fatalf("recreate error = %v, want %v", err, errInjectedTokenCreate)
	}
	var row model.ApiToken
	if err := db.Where("name = ?", "cli-fallback").First(&row).Error; err != nil {
		t.Fatalf("load preserved token: %v", err)
	}
	if !svc.Match(first.Token) {
		t.Fatal("original token was revoked after replacement failure")
	}
}

// Create caps the name at 64 characters; RecreateByName writes the same column
// and now takes operator input from -tokenName, so it must cap it too.
func TestRecreateByNameRejectsOverlongName(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())

	const wantErr = "token name must be 64 characters or fewer"

	svc := ApiTokenService{}
	_, err := svc.RecreateByName(strings.Repeat("n", 65))
	if err == nil {
		t.Fatal("expected a 65-character token name to be rejected")
	}
	if got := strings.TrimSpace(err.Error()); got != wantErr {
		t.Fatalf("error = %q, want %q — any other error would pass a bare nil check", got, wantErr)
	}
	if _, err := svc.RecreateByName(strings.Repeat("n", 64)); err != nil {
		t.Fatalf("64 characters is the documented limit, got: %v", err)
	}
}

// Rotating a monitor token through the CLI silently reissued it as admin,
// because the replacement row took the column default instead of the old scope.
func TestRecreateByNameKeepsReplacedTokenScope(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())

	svc := ApiTokenService{}
	if _, err := svc.Create("grafana", model.ApiScopeMonitor, 0); err != nil {
		t.Fatalf("seed grafana: %v", err)
	}
	rotated, err := svc.RecreateByName("grafana")
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}

	var row model.ApiToken
	if err := database.GetDB().Where("name = ?", "grafana").First(&row).Error; err != nil {
		t.Fatalf("load grafana: %v", err)
	}
	if row.Scope != model.ApiScopeMonitor {
		t.Fatalf("stored scope = %q, want %q", row.Scope, model.ApiScopeMonitor)
	}
	if rotated.Scope != model.ApiScopeMonitor {
		t.Fatalf("returned scope = %q, want %q", rotated.Scope, model.ApiScopeMonitor)
	}
}

// A scope this build does not know, as after a downgrade, must not be guessed
// as admin; the rotation fails and the stored row stays untouched.
func TestRecreateByNameRefusesUnknownStoredScope(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())

	db := database.GetDB()
	stored := model.ApiToken{Name: "remote", Token: "stored-hash", Enabled: true, Scope: "node-admin"}
	if err := db.Create(&stored).Error; err != nil {
		t.Fatalf("seed remote: %v", err)
	}

	const wantErr = `token "remote" has unknown scope "node-admin"`
	_, err := (&ApiTokenService{}).RecreateByName("remote")
	if err == nil || strings.TrimSpace(err.Error()) != wantErr {
		t.Fatalf("error = %v, want %q", err, wantErr)
	}
	var row model.ApiToken
	if err := db.Where("name = ?", "remote").First(&row).Error; err != nil {
		t.Fatalf("load remote: %v", err)
	}
	if row.Id != stored.Id || row.Token != stored.Token || row.Scope != stored.Scope {
		t.Fatalf("row = %+v, want the stored row %+v unchanged", row, stored)
	}
}

func TestRecreateByNameKeepsOneToken(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())

	svc := ApiTokenService{}
	first, err := svc.RecreateByName("cli-fallback")
	if err != nil {
		t.Fatalf("first recreate: %v", err)
	}
	second, err := svc.RecreateByName("cli-fallback")
	if err != nil {
		t.Fatalf("second recreate: %v", err)
	}
	if first.Token == second.Token {
		t.Fatal("second call returned the same plaintext, want a rotated token")
	}

	var count int64
	if err := database.GetDB().Model(model.ApiToken{}).Where("name = ?", "cli-fallback").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("token rows = %d, want 1", count)
	}
}
