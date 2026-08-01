package service

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

const defaultWireguardBase = "10.0.0.0/24"

func keepAliveStr(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return strconv.Itoa(seconds)
}

func wireguardHostAddr(s string) netip.Addr {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}
	}
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Addr()
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a
	}
	return netip.Addr{}
}

func wireguardAllocationBase(used []string, fallback string) string {
	for _, u := range used {
		a := wireguardHostAddr(u)
		if !a.IsValid() || !a.Is4() || a.IsUnspecified() {
			continue
		}
		if p, err := a.Prefix(24); err == nil {
			return p.String()
		}
	}
	return fallback
}

const wireguardPoolFloorBits = 16

// allocateWireguardAddress returns the first free single-host address in base
// (suffixed /32 for an IPv4 base, /128 for IPv6) that is not already present
// in used. The server holds the first host (.1 / ::1), so allocation starts
// at the second host (.2 / ::2). When allowWidening is set and base is an
// IPv4 prefix narrower than wireguardPoolFloorBits, falls back to a wider
// /16 scope once the configured base is exhausted, so a small pool doesn't
// hard-fail while a wider address space is genuinely available.
//
// allowWidening must be false for AmneziaWG: unlike plain WireGuard, an
// AmneziaWG peer's address must fall inside the kernel interface's own
// configured Address subnet to be routable at all, so silently handing out
// an address from a wider scope would produce an address the interface can
// never actually reach — better to fail loudly on pool exhaustion than hand
// out a client config that will never connect.
func allocateWireguardAddress(used []string, base string, allowWidening bool) (string, error) {
	if base == "" {
		base = defaultWireguardBase
	}
	prefix, err := netip.ParsePrefix(base)
	if err != nil {
		return "", err
	}
	hostBits := "32"
	if prefix.Addr().Is6() {
		hostBits = "128"
	}
	taken := make(map[netip.Addr]struct{}, len(used))
	for _, u := range used {
		if a := wireguardHostAddr(u); a.IsValid() {
			taken[a] = struct{}{}
		}
	}
	scopes := []netip.Prefix{prefix}
	if allowWidening && prefix.Addr().Is4() && prefix.Bits() > wireguardPoolFloorBits {
		if wider, wErr := prefix.Addr().Prefix(wireguardPoolFloorBits); wErr == nil {
			scopes = append(scopes, wider)
		}
	}
	for _, scope := range scopes {
		addr := scope.Masked().Addr().Next().Next()
		for scope.Contains(addr) {
			if _, ok := taken[addr]; !ok {
				return addr.String() + "/" + hostBits, nil
			}
			addr = addr.Next()
		}
	}
	return "", common.NewError("wireguard: no free address available in", scopes[len(scopes)-1].String())
}

// normalizeWireguardAllowedIPs validates user-supplied allowedIPs entries and
// canonicalizes them: bare addresses become single-host prefixes, duplicates drop.
func normalizeWireguardAllowedIPs(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		p, err := netip.ParsePrefix(v)
		if err != nil {
			a, aErr := netip.ParseAddr(v)
			if aErr != nil {
				return nil, common.NewError("wireguard: invalid allowedIPs entry:", v)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		norm := p.String()
		if _, dup := seen[norm]; dup {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out, nil
}

func wireguardAllowedIPsCollision(entries, used []string) string {
	taken := make(map[string]struct{}, len(used))
	for _, u := range used {
		taken[strings.TrimSpace(u)] = struct{}{}
	}
	for _, e := range entries {
		if _, ok := taken[e]; ok {
			return e
		}
	}
	return ""
}

// defaultWireguardClients fills in blank WireGuard credentials for newly added
// clients: a generated keypair when none was provided, a derived public key when
// only a private key was given, and a unique tunnel address allocated from the
// inbound's subnet. It mutates both the typed clients and the parallel raw client
// maps that get persisted into the inbound settings. Existing values are never
// overwritten, so editing a client never rotates its keys.
func defaultWireguardClients(existing, clients []model.Client, interfaceClients []any) error {
	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	base := wireguardAllocationBase(used, defaultWireguardBase)
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
			addr, err := allocateWireguardAddress(used, base, true)
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
				return common.NewError("wireguard: allowedIPs has no usable entry")
			}
			if hit := wireguardAllowedIPsCollision(normalized, used); hit != "" {
				return common.NewError("wireguard: allowedIPs entry already used by another client:", hit)
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
				if c.KeepAlive > 0 {
					m["keepAlive"] = c.KeepAlive
				}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
