package service

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
)

func (s *InboundService) resolveInboundAndClient(clientEmail string) (*model.Inbound, string, bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, "", false, err
	}
	if inbound == nil {
		return nil, "", false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, "", false, err
	}

	clientID := ""
	clientEnabled := false
	for _, oldClient := range clients {
		if oldClient.Email != clientEmail {
			continue
		}
		switch inbound.Protocol {
		case "trojan":
			clientID = oldClient.Password
		case "shadowsocks":
			clientID = oldClient.Email
		default:
			clientID = oldClient.ID
		}
		clientEnabled = oldClient.Enable
		break
	}

	if clientID == "" {
		return nil, "", false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	return inbound, clientID, clientEnabled, nil
}

func (s *InboundService) applySingleClientUpdate(inbound *model.Inbound, clientEmail string, mutate func(client map[string]any)) error {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	clients, ok := settings["clients"].([]any)
	if !ok {
		return common.NewError("invalid clients format in inbound settings")
	}

	newClients := make([]any, 0, 1)
	for idx := range clients {
		c, ok := clients[idx].(map[string]any)
		if !ok {
			continue
		}
		if c["email"] != clientEmail {
			continue
		}
		mutate(c)
		c["updated_at"] = time.Now().Unix() * 1000
		newClients = append(newClients, c)
		break
	}

	if len(newClients) == 0 {
		return common.NewError("Client Not Found For Email:", clientEmail)
	}

	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	inbound.Settings = string(modifiedSettings)
	return nil
}

func (s *InboundService) SetClientTelegramUserID(trafficId int, tgId int64) (bool, error) {
	traffic, inbound, err := s.GetClientInboundByTrafficID(trafficId)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Traffic ID:", trafficId)
	}
	clientEmail := traffic.Email

	_, clientID, _, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, err
	}

	if err := s.applySingleClientUpdate(inbound, clientEmail, func(client map[string]any) {
		client["tgId"] = tgId
	}); err != nil {
		return false, err
	}

	needRestart, err := s.UpdateInboundClient(inbound, clientID)
	return needRestart, err
}

func (s *InboundService) checkIsEnabledByEmail(clientEmail string) (bool, error) {
	_, _, enabled, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, err
	}
	return enabled, nil
}

func (s *InboundService) ToggleClientEnableByEmail(clientEmail string) (bool, bool, error) {
	inbound, clientID, oldEnabled, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, false, err
	}

	if err := s.applySingleClientUpdate(inbound, clientEmail, func(client map[string]any) {
		client["enable"] = !oldEnabled
	}); err != nil {
		return false, false, err
	}

	needRestart, err := s.UpdateInboundClient(inbound, clientID)
	if err != nil {
		return false, needRestart, err
	}

	return !oldEnabled, needRestart, nil
}

func (s *InboundService) ResetClientIpLimitByEmail(clientEmail string, count int) (bool, error) {
	inbound, clientID, _, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, err
	}

	if err := s.applySingleClientUpdate(inbound, clientEmail, func(client map[string]any) {
		client["limitIp"] = count
	}); err != nil {
		return false, err
	}

	needRestart, err := s.UpdateInboundClient(inbound, clientID)
	return needRestart, err
}

func (s *InboundService) ResetClientExpiryTimeByEmail(clientEmail string, expiryTime int64) (bool, error) {
	inbound, clientID, _, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, err
	}

	if err := s.applySingleClientUpdate(inbound, clientEmail, func(client map[string]any) {
		client["expiryTime"] = expiryTime
	}); err != nil {
		return false, err
	}

	needRestart, err := s.UpdateInboundClient(inbound, clientID)
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficLimitByEmail(clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}

	inbound, clientID, _, err := s.resolveInboundAndClient(clientEmail)
	if err != nil {
		return false, err
	}

	if err := s.applySingleClientUpdate(inbound, clientEmail, func(client map[string]any) {
		client["totalGB"] = totalGB * 1024 * 1024 * 1024
	}); err != nil {
		return false, err
	}

	needRestart, err := s.UpdateInboundClient(inbound, clientID)
	return needRestart, err
}
