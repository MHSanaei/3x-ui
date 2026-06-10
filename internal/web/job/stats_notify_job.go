package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
)

// LoginStatus represents the status of a login attempt.
type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1 // Successful login
	LoginFail    LoginStatus = 0 // Failed login attempt
)

// StatsNotifyJob sends periodic statistics reports via Telegram bot.
type StatsNotifyJob struct {
	xrayService  service.XrayService
	tgbotService tgbot.Tgbot
}

// NewStatsNotifyJob creates a new statistics notification job instance.
func NewStatsNotifyJob() *StatsNotifyJob {
	return new(StatsNotifyJob)
}

// Run sends a statistics report via Telegram bot if Xray is running.
func (j *StatsNotifyJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	j.tgbotService.SendReport()
}
