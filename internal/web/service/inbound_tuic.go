package service

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
)

func (s *InboundService) DesiredTuicInstances() ([]tuic.Instance, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("protocol = ? AND enable = ? AND node_id IS NULL", model.TUIC, true).
		Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return nil, nil
	}

	instances := make([]tuic.Instance, 0, len(inbounds))
	for _, ib := range inbounds {
		inst, ok := tuic.InstanceFromInbound(ib)
		if !ok {
			continue
		}
		instances = append(instances, inst)
	}
	emails := make([]string, 0)
	for _, inst := range instances {
		for _, e := range inst.Clients {
			emails = append(emails, e.Email)
		}
	}
	disabled, err := trafficDisabledEmails(db, emails)
	if err != nil {
		return nil, err
	}
	served := instances[:0]
	for _, inst := range instances {
		kept := make([]tuic.TuicClientSettings, 0, len(inst.Clients))
		for _, e := range inst.Clients {
			if _, off := disabled[e.Email]; !off {
				kept = append(kept, e)
			}
		}
		inst.Clients = kept
		if len(kept) > 0 {
			served = append(served, inst)
		}
	}
	return served, nil
}

func (s *InboundService) applyLocalTuic(inboundId int) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil || inbound == nil || inbound.Protocol != model.TUIC || inbound.NodeID != nil {
		return
	}
	rt, err := s.runtimeFor(inbound)
	if err != nil {
		return
	}
	payload := inbound
	if inbound.Enable {
		if built, bErr := s.buildInboundForLocalRuntime(database.GetDB(), inbound); bErr == nil {
			payload = built
		}
	}
	if err := rt.UpdateInbound(context.Background(), inbound, payload); err != nil {
		logger.Debug("tuic: immediate client apply failed for inbound", inboundId, ":", err)
	}
}
