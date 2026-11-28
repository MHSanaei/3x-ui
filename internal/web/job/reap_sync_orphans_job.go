package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ReapSyncOrphansJob repairs legacy activation expiry drift in bounded windows, then reaps
// sync-orphaned clients and the history of emails no client holds.
type ReapSyncOrphansJob struct {
	clientService    service.ClientService
	activationCursor int
}

func NewReapSyncOrphansJob() *ReapSyncOrphansJob {
	return new(ReapSyncOrphansJob)
}

func (j *ReapSyncOrphansJob) Run() {
	if next, err := j.clientService.RepairClientActivationExpiries(j.activationCursor); err != nil {
		logger.Warning("repair client activation expiries failed:", err)
	} else {
		j.activationCursor = next
	}
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
