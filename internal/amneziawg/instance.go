// Package amneziawg holds the AmneziaWG protocol's shared, DB-backed shapes
// (Instance, Peer, Obfuscation20, ServerSettings/InboundSettings) and the
// pure functions that derive an Instance from a stored inbound row. It no
// longer manages any OS-level interface itself: that was the kernel-module
// (DKMS) + awg-quick + TPROXY architecture this fork shipped originally,
// retired in favor of an embedded, pure-Go one (amneziawg-go over a gVisor
// netstack, see internal/amneziawgnet) in a hard cutover. This package's
// remaining code is deliberately protocol-shape-only, with no OS dependency
// at all, so both the (now-removed) kernel-module path and the embedded
// path could read -- and, historically, did read -- it identically.
package amneziawg

import (
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// InstanceFromInbound derives a desired Instance from an AmneziaWG inbound,
// building one peer per active client. Returns false when the inbound is not
// a usable AmneziaWG inbound (wrong protocol, unparseable settings, or no
// server block) or has no enabled peer to serve — mirroring
// mtproto.InstanceFromInbound, which skips the sidecar entirely rather than
// run it with nothing to serve.
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.AmneziaWG {
		return Instance{}, false
	}
	var parsed InboundSettings
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil || parsed.Server == nil {
		return Instance{}, false
	}
	server := parsed.Server

	peers := make([]Peer, 0, len(parsed.Clients))
	for _, c := range parsed.Clients {
		if !c.Enable || c.PublicKey == "" || len(c.AllowedIPs) == 0 {
			continue
		}
		peers = append(peers, Peer{
			Email:          c.Email,
			PublicKey:      c.PublicKey,
			PresharedKey:   c.PreSharedKey,
			AllowedIPs:     c.AllowedIPs,
			ForwardedPorts: c.ForwardedPorts,
		})
	}
	if len(peers) == 0 {
		return Instance{}, false
	}

	addresses := []string{serverAddress(server.SubnetIP, server.SubnetCIDR)}
	if server.IPv6Enabled {
		if v6, ok := serverAddressV6(server.IPv6Subnet); ok {
			addresses = append(addresses, v6)
		}
	}

	return Instance{
		Id:                     ib.Id,
		Tag:                    ib.Tag,
		InterfaceName:          interfaceNameForID(ib.Id),
		ListenPort:             ib.Port,
		PrivateKey:             server.PrivateKey,
		PublicKey:              server.PublicKey,
		Address:                addresses,
		MTU:                    server.MTU,
		Obfuscation:            server.Obfuscation(),
		HeaderProtectionKey:    server.HeaderProtectionKey,
		ContentPaddingAddition: server.ContentPaddingAddition,
		RekeyAfterTime:         server.RekeyAfterTime,
		RekeyTimeout:           server.RekeyTimeout,
		RejectAfterTime:        server.RejectAfterTime,
		KeepaliveTimeout:       server.KeepaliveTimeout,
		MaxHandshakeAttempts:   server.MaxHandshakeAttempts,
		Peers:                  peers,
		ExternalInterface:      server.ExternalInterface,
		IPv6Enabled:            server.IPv6Enabled,
		IPv6ExternalInterface:  server.IPv6ExternalInterface,
		RouteThroughXray:       server.RouteThroughXray,
	}, true
}

// interfaceNameForID derives the OS-level interface name for an inbound, e.g.
// "awg42". Kept even though the embedded path has no real kernel interface
// of its own: internal/amneziawgnet still uses the same name as a purely
// cosmetic/log-friendly label, so an existing peer's identity/history
// doesn't shift across the cutover.
func interfaceNameForID(id int) string {
	return fmt.Sprintf("awg%d", id)
}

// serverAddress returns the server's own tunnel address for a subnet base,
// e.g. "10.8.1.1/24" for base "10.8.1.0" or "10.8.1.5". The server always
// holds the first usable host of the network subnetIP/cidr actually
// describes -- derived via netip rather than assuming subnetIP already ends
// in ".0", so a subnetIP that isn't a bare network address (a typo, or a
// manually edited value) can never collide with peer addresses, which are
// allocated starting from the network's second host upward (see
// allocateWireguardAddress). Falls back to the previous literal behavior
// only if subnetIP/cidr doesn't parse as an IPv4 network at all -- normal
// saves never reach that path since ValidateSubnetIPv4 already rejects it.
func serverAddress(subnetIP string, cidr int) string {
	if cidr <= 0 {
		cidr = 24
	}
	// A /32 has no host bits at all -- "first usable host" is meaningless,
	// and Next() would step outside the block entirely -- so a single-host
	// base is used exactly as given, same as before this fix.
	prefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", subnetIP, cidr))
	if err != nil || !prefix.Addr().Is4() || cidr >= 32 {
		return fmt.Sprintf("%s/%d", subnetIP, cidr)
	}
	host := prefix.Masked().Addr().Next()
	return fmt.Sprintf("%s/%d", host, cidr)
}

// serverAddressV6 returns the server's own IPv6 tunnel address for a subnet
// CIDR (e.g. "fd86:ea04:1115::1/64" for "fd86:ea04:1115::/64"), the first
// usable host in the prefix. ok is false when subnetCIDR is empty or not a
// valid IPv6 prefix.
func serverAddressV6(subnetCIDR string) (addr string, ok bool) {
	prefix, err := netip.ParsePrefix(subnetCIDR)
	if err != nil || !prefix.Addr().Is6() {
		return "", false
	}
	host := prefix.Masked().Addr().Next()
	return fmt.Sprintf("%s/%d", host, prefix.Bits()), true
}

// FirstIPv4 returns the first IPv4 address (mask stripped) among allowedIPs,
// or "" if none — used by internal/web/service/server.go's
// amneziawgEmailIndex to derive a peer's tunnel IPv4 address for the panel's
// access-log viewer.
func FirstIPv4(allowedIPs []string) string {
	for _, a := range allowedIPs {
		if prefix, err := netip.ParsePrefix(a); err == nil {
			if prefix.Addr().Is4() {
				return prefix.Addr().String()
			}
			continue
		}
		if addr, err := netip.ParseAddr(a); err == nil && addr.Is4() {
			return addr.String()
		}
	}
	return ""
}

// FirstIPv6 returns the first IPv6 address (mask stripped) among allowedIPs,
// or "" if none — the IPv6 counterpart of FirstIPv4, used by
// internal/amneziawgnet's IPv6-address-alias mechanism to find which
// address, if any, a peer wants aliased onto the host, and by
// internal/web/service/xray.go's injectAmneziawgV6Egress to build that
// peer's own freedom outbound (sendThrough). Only the first match is
// returned, exactly like FirstIPv4 — more than one IPv6 AllowedIPs entry
// per peer is not a supported configuration for either feature.
func FirstIPv6(allowedIPs []string) string {
	for _, a := range allowedIPs {
		if prefix, err := netip.ParsePrefix(a); err == nil {
			if prefix.Addr().Is6() && !prefix.Addr().Is4In6() {
				return prefix.Addr().String()
			}
			continue
		}
		if addr, err := netip.ParseAddr(a); err == nil && addr.Is6() && !addr.Is4In6() {
			return addr.String()
		}
	}
	return ""
}
