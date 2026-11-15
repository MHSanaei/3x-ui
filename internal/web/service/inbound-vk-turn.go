package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/vkturn"
)

func (s *InboundService) DesiredVKTurnInstances() ([]vkturn.Instance, error) {
	var inbounds []model.Inbound
	if err := database.GetDB().Where("protocol = ? AND enable = ? AND node_id IS NULL", model.WireGuard, true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	desired := make([]vkturn.Instance, 0, len(inbounds))
	for _, inbound := range inbounds {
		if instance, ok := vkturn.InstanceFromInbound(&inbound); ok {
			desired = append(desired, instance)
		}
	}
	return desired, nil
}

func inboundVKTurn(protocol, settings string, port int) *vkturn.Settings {
	if model.Protocol(protocol) != model.WireGuard {
		return nil
	}
	cfg, err := vkturn.SettingsFromInbound(&model.Inbound{Protocol: model.WireGuard, Port: port, Settings: settings})
	if err != nil || !cfg.Enabled {
		return nil
	}
	return &cfg
}
