package job

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// RealityWatchJob checks the target of every watched REALITY inbound and replaces a dead one.
type RealityWatchJob struct {
	realityWatchService service.RealityWatchService
}

func NewRealityWatchJob() *RealityWatchJob {
	return &RealityWatchJob{}
}

func (j *RealityWatchJob) Run() {
	j.realityWatchService.Tick(context.Background())
}
