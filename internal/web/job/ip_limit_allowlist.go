package job

import (
	"net/netip"
	"strings"
)

// ipLimitAllowlist holds the operator's trusted addresses and networks. An IP
// that matches is neither counted towards a client's IP limit nor banned:
// counting it would still cut the office or campus NAT the entry exists to
// protect, which is the whole point of the setting (#5378).
type ipLimitAllowlist struct {
	prefixes []netip.Prefix
	addrs    []netip.Addr
}

// parseIpLimitAllowlist accepts the same shape as trustedProxyCIDRs: entries
// separated by commas, semicolons, whitespace or newlines, each either a CIDR
// or a bare address. Unparsable entries are skipped rather than failing the
// scan — a typo must not disable the limit for everybody.
func parseIpLimitAllowlist(raw string) ipLimitAllowlist {
	var list ipLimitAllowlist
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	}) {
		if prefix, err := netip.ParsePrefix(field); err == nil {
			list.prefixes = append(list.prefixes, prefix.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(field); err == nil {
			list.addrs = append(list.addrs, addr.Unmap())
		}
	}
	return list
}

func (l ipLimitAllowlist) empty() bool {
	return len(l.prefixes) == 0 && len(l.addrs) == 0
}

func (l ipLimitAllowlist) contains(ip string) bool {
	if l.empty() {
		return false
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, allowed := range l.addrs {
		if allowed == addr {
			return true
		}
	}
	for _, prefix := range l.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// split separates the entries an allowlist protects from the ones the limit
// still applies to, preserving the caller's ordering in both.
func (l ipLimitAllowlist) split(entries []IPWithTimestamp) (limited, allowed []IPWithTimestamp) {
	if l.empty() {
		return entries, nil
	}
	limited = make([]IPWithTimestamp, 0, len(entries))
	for _, entry := range entries {
		if l.contains(entry.IP) {
			allowed = append(allowed, entry)
			continue
		}
		limited = append(limited, entry)
	}
	return limited, allowed
}
