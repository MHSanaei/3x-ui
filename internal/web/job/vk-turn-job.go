package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/vkturn"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type VKTurnJob struct{ inboundService service.InboundService }

func NewVKTurnJob() *VKTurnJob { return new(VKTurnJob) }

func (j *VKTurnJob) Run() {
	desired, err := j.inboundService.DesiredVKTurnInstances()
	if err != nil {
		logger.Warning("VK TURN: read desired instances: ", err)
		return
	}
	vkturn.GetManager().Reconcile(desired)
}
