package service

import (
	"fmt"
	"strconv"
	"strings"

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

func validateClientKeepAliveAgainstExisting(protocol model.Protocol, client model.Client, existing []model.Client, identityEmail ...string) error {
	if err := validateClientKeepAlive(protocol, client); err == nil || client.KeepAlive == nil {
		return err
	}
	if protocol != model.AmneziaWG && protocol != model.WireGuard {
		return nil
	}
	value := client.KeepAlive.String()
	identity := client.Email
	if len(identityEmail) > 0 && identityEmail[0] != "" {
		identity = identityEmail[0]
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || strconv.FormatInt(parsed, 10) != value {
		return validateClientKeepAlive(protocol, client)
	}
	for _, old := range existing {
		if strings.EqualFold(old.Email, identity) && old.KeepAlive != nil && old.KeepAlive.String() == value {
			return nil
		}
	}
	return validateClientKeepAlive(protocol, client)
}

func validateClientsKeepAliveAgainstExisting(protocol model.Protocol, clients, existing []model.Client) error {
	for _, client := range clients {
		if err := validateClientKeepAliveAgainstExisting(protocol, client, existing); err != nil {
			return fmt.Errorf("client %q: %w", client.Email, err)
		}
	}
	return nil
}
