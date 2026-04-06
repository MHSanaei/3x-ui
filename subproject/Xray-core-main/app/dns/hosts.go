package dns

import (
	"context"
	"runtime"
	"strconv"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/strmatcher"
	"github.com/xtls/xray-core/features/dns"
)

// StaticHosts represents static domain-ip mapping in DNS server.
type StaticHosts struct {
	ips      [][]net.Address
	matchers strmatcher.IndexMatcher
}

// NewStaticHosts creates a new StaticHosts instance.
func NewStaticHosts(hosts []*Config_HostMapping) (*StaticHosts, error) {
	g := new(strmatcher.MatcherGroup)
	sh := &StaticHosts{
		ips:      make([][]net.Address, len(hosts)+16),
		matchers: g,
	}

	defer runtime.GC()
	for i, mapping := range hosts {
		hosts[i] = nil
		matcher, err := toStrMatcher(mapping.Type, mapping.Domain)
		if err != nil {
			errors.LogErrorInner(context.Background(), err, "failed to create domain matcher, ignore domain rule [type: ", mapping.Type, ", domain: ", mapping.Domain, "]")
			continue
		}
		id := g.Add(matcher)
		ips := make([]net.Address, 0, len(mapping.Ip)+1)
		switch {
		case len(mapping.ProxiedDomain) > 0:
			if mapping.ProxiedDomain[0] == '#' {
				rcode, err := strconv.Atoi(mapping.ProxiedDomain[1:])
				if err != nil {
					return nil, err
				}
				ips = append(ips, dns.RCodeError(rcode))
			} else {
				ips = append(ips, net.DomainAddress(mapping.ProxiedDomain))
			}
		case len(mapping.Ip) > 0:
			for _, ip := range mapping.Ip {
				addr := net.IPAddress(ip)
				if addr == nil {
					errors.LogError(context.Background(), "invalid IP address in static hosts: ", ip, ", ignore this ip for rule [type: ", mapping.Type, ", domain: ", mapping.Domain, "]")
					continue
				}
				ips = append(ips, addr)
			}
			if len(ips) == 0 {
				continue
			}
		}

		sh.ips[id] = ips
	}

	return sh, nil
}

func filterIP(ips []net.Address, option dns.IPOption) []net.Address {
	filtered := make([]net.Address, 0, len(ips))
	for _, ip := range ips {
		if (ip.Family().IsIPv4() && option.IPv4Enable) || (ip.Family().IsIPv6() && option.IPv6Enable) {
			filtered = append(filtered, ip)
		}
	}
	return filtered
}

func (h *StaticHosts) lookupInternal(domain string) ([]net.Address, error) {
	ips := make([]net.Address, 0)
	found := false
	for _, id := range h.matchers.Match(domain) {
		for _, v := range h.ips[id] {
			if err, ok := v.(dns.RCodeError); ok {
				if uint16(err) == 0 {
					return nil, dns.ErrEmptyResponse
				}
				return nil, err
			}
		}
		ips = append(ips, h.ips[id]...)
		found = true
	}
	if !found {
		return nil, nil
	}
	return ips, nil
}

func (h *StaticHosts) lookup(domain string, option dns.IPOption, maxDepth int) ([]net.Address, error) {
	switch addrs, err := h.lookupInternal(domain); {
	case err != nil:
		return nil, err
	case len(addrs) == 0: // Not recorded in static hosts, return nil
		return addrs, nil
	case len(addrs) == 1 && addrs[0].Family().IsDomain(): // Try to unwrap domain
		errors.LogDebug(context.Background(), "found replaced domain: ", domain, " -> ", addrs[0].Domain(), ". Try to unwrap it")
		if maxDepth > 0 {
			unwrapped, err := h.lookup(addrs[0].Domain(), option, maxDepth-1)
			if err != nil {
				return nil, err
			}
			if unwrapped != nil {
				return unwrapped, nil
			}
		}
		return addrs, nil
	default: // IP record found, return a non-nil IP array
		return filterIP(addrs, option), nil
	}
}

// Lookup returns IP addresses or proxied domain for the given domain, if exists in this StaticHosts.
func (h *StaticHosts) Lookup(domain string, option dns.IPOption) ([]net.Address, error) {
	return h.lookup(domain, option, 5)
}
func NewStaticHostsFromCache(matcher strmatcher.IndexMatcher, hostIPs map[string][]string) (*StaticHosts, error) {
	sh := &StaticHosts{
		ips:      make([][]net.Address, matcher.Size()+1),
		matchers: matcher,
	}

	order := hostIPs["_ORDER"]
	var offset uint32

	img, ok := matcher.(*strmatcher.IndexMatcherGroup)
	if !ok {
		// Single matcher (e.g. only manual or only one geosite)
		if len(order) > 0 {
			pattern := order[0]
			ips := parseIPs(hostIPs[pattern])
			for i := uint32(1); i <= matcher.Size(); i++ {
				sh.ips[i] = ips
			}
		}
		return sh, nil
	}

	for i, m := range img.Matchers {
		if i < len(order) {
			pattern := order[i]
			ips := parseIPs(hostIPs[pattern])
			for j := uint32(1); j <= m.Size(); j++ {
				sh.ips[offset+j] = ips
			}
			offset += m.Size()
		}
	}
	return sh, nil
}

func parseIPs(raw []string) []net.Address {
	addrs := make([]net.Address, 0, len(raw))
	for _, s := range raw {
		if len(s) > 1 && s[0] == '#' {
			rcode, _ := strconv.Atoi(s[1:])
			addrs = append(addrs, dns.RCodeError(rcode))
		} else {
			addrs = append(addrs, net.ParseAddress(s))
		}
	}
	return addrs
}
