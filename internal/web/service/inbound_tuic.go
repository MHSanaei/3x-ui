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
	return s.desiredTuicInstances(inbounds)
}

func configuredTuicInstances(inbounds []*model.Inbound) []tuic.Instance {
	instances := make([]tuic.Instance, 0)
	for _, inbound := range inbounds {
		if inbound.Protocol != model.TUIC || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		inst, ok := tuic.ConfiguredInstanceFromInbound(inbound)
		if !ok || tuic.ValidateClients(inst.Clients) != nil {
			logger.Warningf("tuic: skipping inbound %d: invalid settings or clients", inbound.Id)
			continue
		}
		inst.Clients = supportedTuicRelayClients(inst)
		instances = append(instances, inst)
	}
	return instances
}

func supportedTuicRelayClients(inst tuic.Instance) []tuic.TuicClientSettings {
	clients := make([]tuic.TuicClientSettings, 0, len(inst.Clients))
	for _, client := range inst.Clients {
		if tuic.ValidateRelayEmail(client.Email) == nil {
			clients = append(clients, client)
		}
	}
	if len(clients) != len(inst.Clients) {
		logger.Warningf("tuic: inbound %d: %d clients excluded: email must contain 1 to 255 UTF-8 bytes", inst.Id, len(inst.Clients)-len(clients))
	}
	return clients
}

func validateTuicClientEmails(protocol model.Protocol, clients []model.Client) error {
	if protocol != model.TUIC {
		return nil
	}
	for _, client := range clients {
		if err := tuic.ValidateRelayEmail(client.Email); err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) desiredTuicInstances(inbounds []*model.Inbound) ([]tuic.Instance, error) {
	db := database.GetDB()
	reserved, err := templateInboundTags()
	if err != nil {
		return nil, err
	}
	instances := make([]tuic.Instance, 0, len(inbounds))
	for _, ib := range inbounds {
		if ib.Protocol != model.TUIC || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if _, ok := tuic.InstanceFromInbound(ib); !ok {
			logger.Warningf("tuic: skipping inbound %d: invalid settings JSON", ib.Id)
			continue
		}
		if _, taken := reserved[ib.Tag]; taken {
			logger.Warningf("tuic: skipping inbound %d: tag conflicts with Xray template", ib.Id)
			continue
		}
		built, err := s.buildInboundForLocalRuntime(db, ib)
		if err != nil {
			return nil, err
		}
		inst, ok := tuic.InstanceFromInbound(built)
		if !ok {
			continue
		}
		inst.Clients = supportedTuicRelayClients(inst)
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
