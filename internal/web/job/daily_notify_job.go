package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
)

// DailyNotifyJob runs the bot's customer-facing daily notices. It is scheduled
// hourly and the pass itself decides whether this is the admin's chosen hour.
type DailyNotifyJob struct {
	tgbotService tgbot.Tgbot
}

// NewDailyNotifyJob creates a new daily customer notice job instance.
func NewDailyNotifyJob() *DailyNotifyJob {
	return new(DailyNotifyJob)
}

// Run sends the day's customer notices if the configured hour has arrived.
func (j *DailyNotifyJob) Run() {
	j.tgbotService.RunDailyPass()
}
