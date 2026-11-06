package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type TunnelJob struct {
	ctx context.Context
	mu  sync.Mutex
}

func NewTunnelJob(ctx context.Context) *TunnelJob { return &TunnelJob{ctx: ctx} }
func (j *TunnelJob) Run() {
	if !j.mu.TryLock() {
		return
	}
	defer j.mu.Unlock()
	ctx, cancel := context.WithTimeout(j.ctx, 30*time.Second)
	defer cancel()
	if err := (&service.TunnelService{}).Reconcile(ctx); err != nil && ctx.Err() == nil {
		logger.Warning("tunnel reconciliation failed:", err)
	}
}
