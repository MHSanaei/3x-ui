package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultAmneziaWGSubnetBase resolves the /CIDR base new peer addresses are
// allocated from, out of the inbound's own configured server subnet — unlike
// WireGuard, which always falls back to a fixed 10.0.0.0/24.
func defaultAmneziaWGSubnetBase(settingsJSON string) (string, error) {
	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(settingsJSON), &parsed); err != nil {
		return "", fmt.Errorf("amneziawg: invalid settings: %w", err)
	}
	if parsed.Server == nil {
		return "", fmt.Errorf("amneziawg: settings missing server block")
	}
	cidr := parsed.Server.SubnetCIDR
	if cidr <= 0 {
		cidr = 24
	}
	return fmt.Sprintf("%s/%d", parsed.Server.SubnetIP, cidr), nil
}

// defaultAmneziaWGClients fills in blank AmneziaWG credentials for newly
// added clients: a generated keypair when none was provided, a derived
// public key when only a private key was given, and a unique tunnel address
// allocated from the inbound's own configured subnet. It mutates both the
// typed clients and the parallel raw client maps that get persisted into the
// inbound settings. Existing values are never overwritten, so editing a
// client never rotates its keys. Mirrors defaultWireguardClients, reusing
// its IP allocation and validation helpers — the only real difference is
// where the allocation base comes from.
func defaultAmneziaWGClients(settingsJSON string, existing, clients []model.Client, interfaceClients []any) error {
	base, err := defaultAmneziaWGSubnetBase(settingsJSON)
	if err != nil {
		return err
	}

	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	for i := range clients {
		c := &clients[i]
		if c.PrivateKey == "" && c.PublicKey == "" {
			priv, pub, err := wgutil.GenerateWireguardKeypair()
			if err != nil {
				return err
			}
			c.PrivateKey = priv
			c.PublicKey = pub
		} else if c.PublicKey == "" && c.PrivateKey != "" {
			pub, err := wgutil.PublicKeyFromPrivate(c.PrivateKey)
			if err != nil {
				return err
			}
			c.PublicKey = pub
		}
		if len(c.AllowedIPs) == 0 {
			addr, err := allocateWireguardAddress(used, base)
			if err != nil {
				return err
			}
			c.AllowedIPs = []string{addr}
		} else {
			normalized, err := normalizeWireguardAllowedIPs(c.AllowedIPs)
			if err != nil {
				return err
			}
			if len(normalized) == 0 {
				return common.NewError("amneziawg: allowedIPs has no usable entry")
			}
			if hit := wireguardAllowedIPsCollision(normalized, used); hit != "" {
				return common.NewError("amneziawg: allowedIPs entry already used by another client:", hit)
			}
			c.AllowedIPs = normalized
		}
		used = append(used, c.AllowedIPs...)

		if i < len(interfaceClients) {
			if m, ok := interfaceClients[i].(map[string]any); ok {
				m["privateKey"] = c.PrivateKey
				m["publicKey"] = c.PublicKey
				m["allowedIPs"] = c.AllowedIPs
				if c.PreSharedKey != "" {
					m["preSharedKey"] = c.PreSharedKey
				}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
