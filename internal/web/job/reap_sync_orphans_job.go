package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ReapSyncOrphansJob hard-deletes the clients the node-snapshot sweep only soft-orphaned,
// then the usage and change history of every email no client holds any more.
type ReapSyncOrphansJob struct {
	clientService service.ClientService
}

func NewReapSyncOrphansJob() *ReapSyncOrphansJob {
	return new(ReapSyncOrphansJob)
}

func (j *ReapSyncOrphansJob) Run() {
	if reaped, err := j.clientService.ReapSyncOrphans(); err != nil {
		logger.Warning("reap sync orphans failed:", err)
	} else if reaped > 0 {
		logger.Infof("reap sync orphans: removed %d client(s)", reaped)
	}
	// After the client reap, so a client it just removed loses its history in the same run.
	if rows, err := j.clientService.ReapOrphanClientHistory(); err != nil {
		logger.Warning("reap orphan client history failed:", err)
	} else if rows > 0 {
		logger.Infof("reap orphan client history: removed %d row(s)", rows)
	}
}
