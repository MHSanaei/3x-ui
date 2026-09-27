package job

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/discord"
)

// DiscordNotifyJob sends periodic status reports and database backups via Discord bot.
type DiscordNotifyJob struct {
	xrayService    service.XrayService
	serverService  service.ServerService
	inboundService service.InboundService
	settingService service.SettingService
	discordService *discord.DiscordService
}

// NewDiscordNotifyJob creates a new Discord notification job instance.
func NewDiscordNotifyJob(discordService *discord.DiscordService) *DiscordNotifyJob {
	return &DiscordNotifyJob{
		discordService: discordService,
	}
}

// Run executes the periodic status report if Discord bot is enabled and Xray is running.
func (j *DiscordNotifyJob) Run() {
	if j.discordService == nil {
		return
	}
	enabled, err := j.settingService.GetDiscordBotEnable()
	if err != nil || !enabled {
		return
	}
	if !j.xrayService.IsXrayRunning() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := j.discordService.SendReport(ctx, &j.serverService, &j.inboundService); err != nil {
		logger.Warning("DiscordNotifyJob: failed to send status report: ", err)
	}
}
