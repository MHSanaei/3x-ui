package job

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/discord"
)

func TestDiscordNotifyJob_NilServiceNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DiscordNotifyJob.Run panicked with nil discordService: %v", r)
		}
	}()
	NewDiscordNotifyJob(nil).Run()
}

func TestDiscordNotifyJob_DisabledNoPanic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	settingService := service.SettingService{}
	_ = settingService.SetDiscordBotEnable(false)

	discordService := discord.NewDiscordService(settingService)
	job := NewDiscordNotifyJob(discordService)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DiscordNotifyJob.Run panicked when disabled: %v", r)
		}
	}()
	job.Run()
}
