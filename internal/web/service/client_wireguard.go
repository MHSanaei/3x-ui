package service

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

const defaultWireguardBase = "10.0.0.0/24"

// wireguardSubnetSettings is the subset of a WireGuard inbound's top-level
// settings JSON this package cares about for subnet resolution. Unlike
// AmneziaWG (whose whole settings shape is a typed struct in
// internal/amneziawg), plain WireGuard has no dedicated Go struct on this
// fork's side at all -- everything else is handled as untyped
// map[string]any -- so this stays a narrow, local decode rather than
// introducing a full struct just for two fields.
type wireguardSubnetSettings struct {
	SubnetIP   string `json:"subnetIp"`
	SubnetCIDR int    `json:"subnetCidr"`
}

// explicitWireguardSubnetBase resolves an admin-configured subnet base out
// of settingsJSON's own subnetIp/subnetCidr fields, mirroring AmneziaWG's
// defaultAmneziaWGSubnetBases. Returns "" when either field is unset/empty
// or doesn't parse as a valid prefix -- callers fall back to
// wireguardAllocationBase's existing infer-from-clients behavior in that
// case, so an inbound saved before this field existed (or one that simply
// never set it) keeps behaving exactly as it always has.
func explicitWireguardSubnetBase(settingsJSON string) string {
	var parsed wireguardSubnetSettings
	if err := json.Unmarshal([]byte(settingsJSON), &parsed); err != nil {
		return ""
	}
	ip := strings.TrimSpace(parsed.SubnetIP)
	if ip == "" || parsed.SubnetCIDR <= 0 {
		return ""
	}
	base := fmt.Sprintf("%s/%d", ip, parsed.SubnetCIDR)
	if _, err := netip.ParsePrefix(base); err != nil {
		return ""
	}
	return base
}

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
// not already in used, starting at the second host (the server holds the first).
//
// allowWidening retries in the containing /16 once base's pool is exhausted.
// True for Xray-native WireGuard, whose AllowedIPs aren't tied to a kernel
// interface subnet; AmneziaWG must pass false and fail loudly instead, since an
// address outside its interface's own Address would be silently unroutable.
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
	var wide []netip.Prefix
	for _, u := range used {
		p, ok := wireguardClaimedPrefix(u)
		if !ok {
			continue
		}
		if p.IsSingleIP() {
			taken[p.Addr()] = struct{}{}
		} else {
			wide = append(wide, p)
		}
	}
	isTaken := func(a netip.Addr) bool {
		if _, ok := taken[a]; ok {
			return true
		}
		for _, p := range wide {
			if p.Contains(a) {
				return true
			}
		}
		return false
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
			if !isTaken(addr) {
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

// wireguardClaimedPrefix is the masked range an allowedIPs entry claims, as xray
// reads it. A /0 default route claims no tunnel address, as legacy peers carry it.
func wireguardClaimedPrefix(s string) (netip.Prefix, bool) {
	s = strings.TrimSpace(s)
	p, err := netip.ParsePrefix(s)
	if err != nil {
		a, aErr := netip.ParseAddr(s)
		if aErr != nil {
			return netip.Prefix{}, false
		}
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if p.Bits() == 0 {
		return netip.Prefix{}, false
	}
	return p.Masked(), true
}

// wireguardAllowedIPsOverlap returns the first entry whose range overlaps a used
// one, and that used entry; xray routes and attributes by containment, not equality.
func wireguardAllowedIPsOverlap(entries, used []string) (entry, taken string) {
	usedPrefixes := make([]netip.Prefix, len(used))
	usedOK := make([]bool, len(used))
	for i, u := range used {
		usedPrefixes[i], usedOK[i] = wireguardClaimedPrefix(u)
	}
	for _, e := range entries {
		ep, ok := wireguardClaimedPrefix(e)
		if !ok {
			continue
		}
		for i, up := range usedPrefixes {
			if usedOK[i] && ep.Overlaps(up) {
				return e, used[i]
			}
		}
	}
	return "", ""
}

// defaultWireguardClients fills in blank WireGuard credentials for newly added
// clients: a generated keypair when none was provided, a derived public key when
// only a private key was given, and a unique tunnel address allocated from the
// inbound's subnet. It mutates both the typed clients and the parallel raw client
// maps that get persisted into the inbound settings. Existing values are never
// overwritten, so editing a client never rotates its keys.
//
// crossInboundUsed maps AllowedIPs already claimed by clients on every OTHER
// WireGuard/AmneziaWG inbound on this panel to a human-readable description
// of which inbound holds it (see otherTunnelAllowedIPs). It is folded into
// used only AFTER the base subnet is resolved, so an unrelated inbound's
// subnet can never skew this inbound's own base-subnet resolution — it only
// ever narrows which addresses are free to hand out or accept, and lets a
// manual-entry collision name the other inbound instead of just the address.
//
// settingsJSON is checked first for an admin-configured subnetIp/subnetCidr
// (see explicitWireguardSubnetBase) — set explicitly, that always wins.
// Only when it's unset does base fall back to inferring from existing
// clients' own addresses, and finally to defaultWireguardBase, exactly as
// before this field existed.
func defaultWireguardClients(settingsJSON string, existing, clients []model.Client, interfaceClients []any, crossInboundUsed map[string]string) error {
	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	base := explicitWireguardSubnetBase(settingsJSON)
	if base == "" {
		base = wireguardAllocationBase(used, defaultWireguardBase)
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
			if entry, taken := wireguardAllowedIPsOverlap(normalized, used); taken != "" {
				if where := crossInboundUsed[taken]; where != "" {
					return common.NewError("wireguard: allowedIPs entry", entry, "overlaps", taken, "used by a client on", where)
				}
				return common.NewError("wireguard: allowedIPs entry", entry, "overlaps", taken, "used by another client")
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
				if ka := c.KeepAliveSeconds(); ka > 0 {
					m["keepAlive"] = ka
				}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
