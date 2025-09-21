package job

import (
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// CheckHashStorageJob periodically cleans up expired hash entries from the Telegram bot's hash storage.
type CheckHashStorageJob struct {
	tgbotService service.Tgbot
}

// NewCheckHashStorageJob creates a new hash storage cleanup job instance.
func NewCheckHashStorageJob() *CheckHashStorageJob {
	return new(CheckHashStorageJob)
}

// Run removes expired hash entries from the Telegram bot's hash storage.
func (j *CheckHashStorageJob) Run() {
	// Remove expired hashes from storage
	j.tgbotService.GetHashStorage().RemoveExpiredHashes()
}
