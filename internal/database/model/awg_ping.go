package model

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// AWGPingPolicy grants IPv4 Echo independently of Xray TCP/UDP routing.
type AWGPingPolicy struct {
	Enabled bool     `json:"enabled"`
	Allow   []string `json:"allow" example:"[\"192.0.2.0/24\"]"`
	Deny    []string `json:"deny" example:"[\"192.0.2.8/32\"]"`
}

func (p *AWGPingPolicy) Clone() *AWGPingPolicy {
	if p == nil {
		return nil
	}
	return &AWGPingPolicy{Enabled: p.Enabled, Allow: slices.Clone(p.Allow), Deny: slices.Clone(p.Deny)}
}

func (p *AWGPingPolicy) Normalize() error {
	if p == nil {
		return nil
	}
	for _, list := range []*[]string{&p.Allow, &p.Deny} {
		if len(*list) > 256 {
			return fmt.Errorf("AWG ping allows at most 256 entries per list")
		}
		normalized := make([]string, 0, len(*list))
		for _, s := range *list {
			s = strings.TrimSpace(s)
			if len(s) > 32 {
				return fmt.Errorf("AWG ping requires IPv4 addresses or CIDRs")
			}
			prefix, err := netip.ParsePrefix(s)
			if err != nil {
				if addr, e := netip.ParseAddr(s); e == nil && addr.Is4() {
					prefix = netip.PrefixFrom(addr, 32)
					err = nil
				}
			}
			if err != nil || !prefix.Addr().Is4() {
				return fmt.Errorf("AWG ping requires IPv4 addresses or CIDRs")
			}
			normalized = append(normalized, prefix.Masked().String())
		}
		slices.Sort(normalized)
		*list = slices.Compact(normalized)
	}
	return nil
}

func (p *AWGPingPolicy) Allows(addr netip.Addr) bool {
	if p == nil || !p.Enabled || !addr.Is4() || !addr.IsGlobalUnicast() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || (addr.As4()[0] == 0 || addr.As4()[0] >= 224) {
		return false
	}
	// Validate both lists before any match, including disabled rules from stored/imported data.
	policy := p.Clone()
	if policy.Normalize() != nil {
		return false
	}
	for _, s := range policy.Deny {
		if netip.MustParsePrefix(s).Contains(addr) {
			return false
		}
	}
	allowed := false
	for _, s := range policy.Allow {
		prefix := netip.MustParsePrefix(s)
		if !prefix.Contains(addr) {
			continue
		}
		if prefix.Bits() <= 30 {
			bytes := addr.As4()
			value := uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
			mask := ^uint32(0) >> prefix.Bits()
			if value&mask == mask {
				return false
			}
		}
		allowed = true
	}
	return allowed
}
