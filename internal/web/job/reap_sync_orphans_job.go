package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ReapSyncOrphansJob hard-deletes the clients the node-snapshot sweep only
// soft-orphaned, once enough clean merges have agreed they are really gone.
type ReapSyncOrphansJob struct {
	clientService service.ClientService
}

func NewReapSyncOrphansJob() *ReapSyncOrphansJob {
	return new(ReapSyncOrphansJob)
}

func (j *ReapSyncOrphansJob) Run() {
	reaped, err := j.clientService.ReapSyncOrphans()
	if err != nil {
		logger.Warning("reap sync orphans failed:", err)
		return
	}
	if reaped > 0 {
		logger.Infof("reap sync orphans: removed %d client(s)", reaped)
	}
}
