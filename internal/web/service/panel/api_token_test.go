package panel

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
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
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

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
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := ApiTokenService{}
	if _, err := svc.RecreateByName(strings.Repeat("n", 65)); err == nil {
		t.Fatal("expected a 65-character token name to be rejected")
	}
	if _, err := svc.RecreateByName(strings.Repeat("n", 64)); err != nil {
		t.Fatalf("64 characters is the documented limit, got: %v", err)
	}
}

func TestRecreateByNameKeepsOneToken(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

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
