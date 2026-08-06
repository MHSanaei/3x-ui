package panel

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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
