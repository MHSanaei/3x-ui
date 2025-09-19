package job

import (
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type CheckHashStorageJob struct {
	tgbotService service.Tgbot
}

func NewCheckHashStorageJob() *CheckHashStorageJob {
	return new(CheckHashStorageJob)
}

// Here Run is an interface method of the Job interface
func (j *CheckHashStorageJob) Run() {
	// Remove expired hashes from storage
	j.tgbotService.GetHashStorage().RemoveExpiredHashes()
}
