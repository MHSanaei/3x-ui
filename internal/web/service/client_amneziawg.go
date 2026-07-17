package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// defaultAmneziaWGClients fills in blank AmneziaWG credentials for newly added
// clients: a generated keypair, a preshared key, and a unique tunnel address
// allocated from the inbound's subnet. It mutates both the typed clients and
// the parallel raw client maps that get persisted into the inbound settings.
func defaultAmneziaWGClients(settingsJSON string, clients []model.Client, interfaceClients []any) error {
	var inbound amneziawg.SettingsInbound
	if err := json.Unmarshal([]byte(settingsJSON), &inbound); err != nil {
		return fmt.Errorf("failed to parse AWG settings: %w", err)
	}
	if inbound.Server == nil {
		return fmt.Errorf("AWG settings missing server config")
	}

	assignedIPs := make([]string, 0, len(inbound.Clients))
	for _, c := range inbound.Clients {
		if c.AssignedIP != "" {
			assignedIPs = append(assignedIPs, c.AssignedIP)
		}
	}

	for i := range clients {
		c := &clients[i]
		if c.PrivateKey == "" && c.PublicKey == "" {
			priv, pub, psk, err := amneziawg.GenerateWireGuardKeyPair()
			if err != nil {
				return fmt.Errorf("failed to generate AWG keypair: %w", err)
			}
			c.PrivateKey = priv
			c.PublicKey = pub
			c.PreSharedKey = psk
		}
		ip, err := amneziawg.NextClientIP(inbound.Server.SubnetIP, assignedIPs)
		if err != nil {
			return fmt.Errorf("failed to allocate IP for AWG client: %w", err)
		}
		assignedIPs = append(assignedIPs, ip)
		c.AllowedIPs = []string{ip + "/32"}

		if i < len(interfaceClients) {
			if m, ok := interfaceClients[i].(map[string]any); ok {
				m["privateKey"] = c.PrivateKey
				m["publicKey"] = c.PublicKey
				m["presharedKey"] = c.PreSharedKey
				m["assignedIp"] = ip
				m["allowedIPs"] = []string{ip + "/32"}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
