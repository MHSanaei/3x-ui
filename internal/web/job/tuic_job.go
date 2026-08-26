package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type TuicJob struct {
	inboundService service.InboundService
}

func NewTuicJob() *TuicJob {
	return new(TuicJob)
}

func (j *TuicJob) Run() {
	desired, err := j.inboundService.DesiredTuicInstances()
	if err != nil {
		logger.Warning("tuic job: get desired instances failed:", err)
		return
	}

	tuic.GetManager().Reconcile(desired)
}
