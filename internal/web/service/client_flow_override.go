package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// clientWithFlow returns a copy of the client matching email with its Flow set
// verbatim, plus whether a match was found. Pure (no DB) so the override is
// unit-testable; unlike clientWithInboundFlow it does not clamp the value.
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

// SetInboundClientFlow overrides the XTLS flow for one client on a single
// inbound, so a client can keep Vision on some flow-capable inbounds and clear
// it on another within the same subscription (#5689, approach 1).
//
// Clearing (""/"none") is always allowed; a non-empty flow must be a recognized
// value (bulkFlowAllowed) and is only accepted on a flow-capable inbound, so an
// invalid value or a flow on a non-capable transport can't reach the Xray
// config. Persists via UpdateInboundClient, whose SyncInbound recreates the
// client_inbounds.flow_override row from the written settings flow.
func (s *ClientService) SetInboundClientFlow(inboundSvc *InboundService, inboundId int, email, flow string) (bool, error) {
	if _, ok := bulkFlowAllowed[flow]; !ok {
		return false, common.NewError("unsupported flow value:", flow)
	}
	if flow == bulkFlowClear {
		flow = ""
	}
	inbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		return false, err
	}
	if flow != "" && !inboundCanEnableTlsFlow(string(inbound.Protocol), inbound.StreamSettings, inbound.Settings) {
		return false, common.NewError("inbound is not flow-capable:", inboundId)
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
	return s.UpdateInboundClient(inboundSvc, &model.Inbound{
		Id:       inboundId,
		Settings: string(settingsPayload),
	}, email)
}
