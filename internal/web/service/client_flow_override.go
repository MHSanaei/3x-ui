package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// clientWithFlow returns a copy of the client matching email from clients, with
// its Flow set verbatim, plus whether a match was found. Pure (no DB) so the
// per-inbound flow override is unit-testable. Unlike clientWithInboundFlow it
// does NOT clamp the value against the inbound's capability — applying an empty
// flow to a flow-capable inbound is exactly the point of a per-(client, inbound)
// override (#5689, approach 1).
func clientWithFlow(clients []model.Client, email, flow string) (model.Client, bool) {
	for i := range clients {
		if clients[i].Email == email {
			c := clients[i]
			c.Flow = flow
			return c, true
		}
	}
	return model.Client{}, false
}

// SetInboundClientFlow overrides the XTLS flow for one client on ONE inbound,
// bypassing the capability clamp. It lets a client carry Vision on some
// flow-capable inbounds and an empty flow on others within the same
// subscription (#5689, approach 1) — e.g. Vision on a Reality inbound but not on
// a tunneled XHTTP+vlessenc inbound the same client is delivered on.
//
// It writes the inbound settings.clients[].flow (read by subscription
// generation) via UpdateInboundClient, then mirrors the value onto the
// per-membership client_inbounds.flow_override (read by EffectiveFlow and the
// clients UI).
//
// NOTE for reviewers: an explicit empty flow can be re-populated by
// restoreVisionFlowForEligibleInbound (#4792) if the inbound is later edited
// into a newly flow-eligible state, since that path treats an empty settings
// flow as "restore the intended Vision from a sibling". Making an explicit clear
// durable across such edits needs a small marker (e.g. a sentinel flow_override
// value or a per-membership lock) — a design decision deferred to maintainers;
// see #5689.
func (s *ClientService) SetInboundClientFlow(inboundSvc *InboundService, inboundId int, email, flow string) (bool, error) {
	inbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		return false, err
	}
	clients, err := inboundSvc.GetClients(inbound)
	if err != nil {
		return false, err
	}
	target, found := clientWithFlow(clients, email, flow)
	if !found {
		return false, common.NewError("client not found on inbound:", email)
	}
	settingsPayload, err := json.Marshal(map[string][]model.Client{"clients": {target}})
	if err != nil {
		return false, err
	}
	needRestart, err := s.UpdateInboundClient(inboundSvc, &model.Inbound{
		Id:       inboundId,
		Settings: string(settingsPayload),
	}, email)
	if err != nil {
		return needRestart, err
	}
	// Mirror onto the relational per-membership override so EffectiveFlow and the
	// clients UI agree with what the subscription now emits.
	if rec, rErr := s.GetRecordByEmail(nil, email); rErr == nil {
		if uErr := database.GetDB().Model(&model.ClientInbound{}).
			Where("client_id = ? AND inbound_id = ?", rec.Id, inboundId).
			Update("flow_override", flow).Error; uErr != nil {
			logger.Warning("SetInboundClientFlow: flow_override update failed:", uErr)
		}
	}
	return needRestart, nil
}
