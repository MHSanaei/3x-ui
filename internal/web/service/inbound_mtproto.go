package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// DesiredMtprotoInstances derives the mtg sidecar configs this panel should be
// running: one instance per enabled local mtproto inbound, serving only the
// secrets of clients that are both enabled in the inbound settings and not
// depletion-disabled in client_traffics. That is the same effective client set
// buildRuntimeInboundForAPI pushes on interactive edits, so the reconcile job
// and the push paths agree on one fingerprint — a disagreement would surface
// as a needless mtg restart, and a job that read only the raw settings would
// keep serving depleted clients until an unrelated restart. Inbounds whose
// every secret is filtered away are omitted so Reconcile stops their sidecar.
func (s *InboundService) DesiredMtprotoInstances() ([]mtproto.Instance, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("protocol = ? AND enable = ? AND node_id IS NULL", model.MTProto, true).
		Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return nil, nil
	}

	ids := make([]int, 0, len(inbounds))
	for _, ib := range inbounds {
		ids = append(ids, ib.Id)
	}
	var disabledRows []xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).
		Where("inbound_id IN ? AND enable = ?", ids, false).
		Select("inbound_id", "email").
		Find(&disabledRows).Error
	if err != nil {
		return nil, err
	}
	disabled := make(map[int]map[string]struct{}, len(disabledRows))
	for _, row := range disabledRows {
		if disabled[row.InboundId] == nil {
			disabled[row.InboundId] = map[string]struct{}{}
		}
		disabled[row.InboundId][row.Email] = struct{}{}
	}

	instances := make([]mtproto.Instance, 0, len(inbounds))
	for _, ib := range inbounds {
		inst, ok := mtproto.InstanceFromInbound(ib)
		if !ok {
			continue
		}
		if off := disabled[ib.Id]; len(off) > 0 {
			kept := make([]mtproto.SecretEntry, 0, len(inst.Secrets))
			for _, sec := range inst.Secrets {
				if _, skip := off[sec.Name]; !skip {
					kept = append(kept, sec)
				}
			}
			inst.Secrets = kept
		}
		if len(inst.Secrets) == 0 {
			continue
		}
		instances = append(instances, inst)
	}
	return instances, nil
}
