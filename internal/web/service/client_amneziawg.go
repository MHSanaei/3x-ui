package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultAmneziaWGSubnetBases resolves the /CIDR bases new peer addresses are
// allocated from, out of the inbound's own configured server subnet(s) —
// unlike WireGuard, which always falls back to a fixed 10.0.0.0/24. v6Base is
// "" when the server doesn't have IPv6 enabled.
func defaultAmneziaWGSubnetBases(settingsJSON string) (v4Base, v6Base string, err error) {
	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(settingsJSON), &parsed); err != nil {
		return "", "", fmt.Errorf("amneziawg: invalid settings: %w", err)
	}
	if parsed.Server == nil {
		return "", "", fmt.Errorf("amneziawg: settings missing server block")
	}
	cidr := parsed.Server.SubnetCIDR
	if cidr <= 0 {
		cidr = 24
	}
	v4Base = fmt.Sprintf("%s/%d", parsed.Server.SubnetIP, cidr)
	if parsed.Server.IPv6Enabled && parsed.Server.IPv6Subnet != "" {
		v6Base = parsed.Server.IPv6Subnet
	}
	return v4Base, v6Base, nil
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
//
// crossInboundUsed maps AllowedIPs already claimed by clients on every OTHER
// WireGuard/AmneziaWG inbound on this panel to a human-readable description
// of which inbound holds it (see otherTunnelAllowedIPs) — it only narrows
// which addresses are free to hand out or accept, and lets a manual-entry
// collision name the other inbound instead of just the address.
func defaultAmneziaWGClients(settingsJSON string, existing, clients []model.Client, interfaceClients []any, crossInboundUsed map[string]string) error {
	v4Base, v6Base, err := defaultAmneziaWGSubnetBases(settingsJSON)
	if err != nil {
		return err
	}

	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	for addr := range crossInboundUsed {
		used = append(used, addr)
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
			// allowWidening=false: unlike WireGuard's Xray-native inbound,
			// AmneziaWG's kernel interface Address is exactly the configured
			// subnet, so an address allocated outside it would be silently
			// unroutable. Exhaustion here must fail loudly instead.
			addr, err := allocateWireguardAddress(used, v4Base, false)
			if err != nil {
				return err
			}
			allowed := []string{addr}
			if v6Base != "" {
				addr6, err := allocateWireguardAddress(used, v6Base, false)
				if err != nil {
					return err
				}
				allowed = append(allowed, addr6)
			}
			c.AllowedIPs = allowed
		} else {
			normalized, err := normalizeWireguardAllowedIPs(c.AllowedIPs)
			if err != nil {
				return err
			}
			if len(normalized) == 0 {
				return common.NewError("amneziawg: allowedIPs has no usable entry")
			}
			if hit := wireguardAllowedIPsCollision(normalized, used); hit != "" {
				if where := crossInboundUsed[hit]; where != "" {
					return common.NewError("amneziawg: allowedIPs entry", hit, "is already used by a client on", where)
				}
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
