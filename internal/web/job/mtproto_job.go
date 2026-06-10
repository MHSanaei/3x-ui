package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// MtprotoJob reconciles the running mtg sidecar processes against the enabled
// mtproto inbounds in the database, restarts any that crashed, and folds the
// per-inbound traffic scraped from each mtg metrics endpoint into the usual
// inbound traffic accounting.
type MtprotoJob struct {
	inboundService service.InboundService
}

// NewMtprotoJob creates a new mtproto reconcile/traffic job instance.
func NewMtprotoJob() *MtprotoJob {
	return new(MtprotoJob)
}

// Run reconciles desired mtproto inbounds with running mtg processes and
// records traffic deltas.
func (j *MtprotoJob) Run() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("mtproto job: get inbounds failed:", err)
		return
	}

	var desired []mtproto.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.MTProto || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := mtproto.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}

	mgr := mtproto.GetManager()
	mgr.Reconcile(desired)

	deltas := mgr.CollectTraffic()
	if len(deltas) == 0 {
		return
	}
	traffics := make([]*xray.Traffic, 0, len(deltas))
	for _, d := range deltas {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       d.Tag,
			Up:        d.Up,
			Down:      d.Down,
		})
	}
	if _, _, err := j.inboundService.AddTraffic(traffics, nil); err != nil {
		logger.Warning("mtproto job: add traffic failed:", err)
	}
}
