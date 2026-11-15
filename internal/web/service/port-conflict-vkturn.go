package service

import (
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/vkturn"
)

func checkVKTurnPortConflict(db *gorm.DB, inbound *model.Inbound, ignoreID int, transports transportBits) (*portConflictDetail, error) {
	if transports&transportUDP != 0 {
		var candidates []*model.Inbound
		q := db.Model(model.Inbound{}).Where("protocol = ?", model.WireGuard)
		if ignoreID > 0 {
			q = q.Where("id != ?", ignoreID)
		}
		if err := q.Find(&candidates).Error; err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			settings, err := vkturn.SettingsFromInbound(candidate)
			if err != nil || !settings.Enabled || settings.Port != inbound.Port || !sameNode(candidate.NodeID, inbound.NodeID) {
				continue
			}
			return &portConflictDetail{
				InboundID: candidate.Id, Remark: candidate.Remark, Tag: candidate.Tag,
				Listen: "0.0.0.0", Port: settings.Port, Transports: transportUDP, VKTurn: true,
			}, nil
		}
	}
	settings, err := vkturn.SettingsFromInbound(inbound)
	if err != nil || !settings.Enabled {
		return nil, nil
	}
	listener := &model.Inbound{Protocol: model.WireGuard, Port: settings.Port, Listen: "0.0.0.0", NodeID: inbound.NodeID, Settings: `{}`}
	return checkPortConflictTx(db, listener, ignoreID)
}
