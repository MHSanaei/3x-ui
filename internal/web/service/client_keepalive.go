package service

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func validateClientKeepAlive(protocol model.Protocol, client model.Client) error {
	if client.KeepAlive == nil {
		return nil
	}
	switch protocol {
	case model.AmneziaWG:
		return client.KeepAlive.Validate(true)
	case model.WireGuard:
		return client.KeepAlive.Validate(false)
	default:
		return nil
	}
}

func validateClientsKeepAlive(protocol model.Protocol, clients []model.Client) error {
	for _, client := range clients {
		if err := validateClientKeepAlive(protocol, client); err != nil {
			return fmt.Errorf("client %q: %w", client.Email, err)
		}
	}
	return nil
}
