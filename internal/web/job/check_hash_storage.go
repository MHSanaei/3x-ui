package job

import "github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"

// CheckHashStorageJob periodically cleans up expired hash entries from the Telegram bot's hash storage.
type CheckHashStorageJob struct {
	tgbotService tgbot.Tgbot
}

// NewCheckHashStorageJob creates a new hash storage cleanup job instance.
func NewCheckHashStorageJob() *CheckHashStorageJob {
	return new(CheckHashStorageJob)
}

// Run removes expired hash entries from the Telegram bot's hash storage.
func (j *CheckHashStorageJob) Run() {
	storage := j.tgbotService.GetHashStorage()
	if storage == nil {
		return
	}
	storage.RemoveExpiredHashes()
}
