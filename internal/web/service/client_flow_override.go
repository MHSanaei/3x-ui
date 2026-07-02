package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

func clientWithFlow(clients []model.Client, email, flow string) (model.Client, bool) {
	for i := range clients {
		if clients[i].Email == email {
			c := clients[i]
			c.Flow = flow
			c.FlowLock = true
			return c, true
		}
	}
	return model.Client{}, false
}

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
