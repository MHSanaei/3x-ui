package sub

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

var errWireguardDeviceUnavailable = errors.New("wireguard device unavailable")

func selectWireguardDevice(client model.Client, id string) (model.Client, error) {
	if id == "" || id == "primary" {
		return client, nil
	}
	for _, device := range client.WireguardDevices {
		if device.Id != id {
			continue
		}
		if !device.Enable {
			return model.Client{}, errWireguardDeviceUnavailable
		}
		client.PrivateKey = device.PrivateKey
		client.PublicKey = device.PublicKey
		client.PreSharedKey = device.PreSharedKey
		client.AllowedIPs = append([]string(nil), device.AllowedIPs...)
		return client, nil
	}
	return model.Client{}, errWireguardDeviceUnavailable
}

func (s *SubService) matchingWireguardDeviceClients(inbound *model.Inbound, subID string) []model.Client {
	clients := s.matchingClients(inbound, subID)
	if inbound.Protocol != model.WireGuard || s.wireguardDevice == "" || s.wireguardDevice == "primary" {
		return clients
	}
	selectedClients := make([]model.Client, 0, len(clients))
	for _, client := range clients {
		selected, err := selectWireguardDevice(client, s.wireguardDevice)
		if err != nil {
			// A shared subscription may also contain unrelated clients.
			// Request-level validation rejects selectors absent everywhere.
			continue
		}
		s.wireguardDeviceSelected = true
		selectedClients = append(selectedClients, selected)
		if inbound.Id > 0 {
			s.clientsByInbound[inbound.Id][client.Email] = selected
		}
	}
	return selectedClients
}

func (s *SubService) validateWireguardDevice() error {
	if s.wireguardDevice != "" && s.wireguardDevice != "primary" && !s.wireguardDeviceSelected {
		return errWireguardDeviceUnavailable
	}
	return nil
}
