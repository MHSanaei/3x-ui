package job

import (
	"net/netip"
	"slices"
	"strings"
)

// An address that matches is neither counted towards a client's IP limit nor
// banned: counting it would still cut the shared network it protects (#5378).
type ipLimitAllowlist struct {
	prefixes []netip.Prefix
	addrs    []netip.Addr
}

// Comma-separated, each entry a CIDR or a bare address. Unparseable entries are
// skipped: the validator uses these same rules, so only a hand-edited DB differs.
func parseIpLimitAllowlist(raw string) ipLimitAllowlist {
	var list ipLimitAllowlist
	for field := range strings.SplitSeq(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(field); err == nil {
			// Unmapped: contains() unmaps the queried address, and Prefix.Contains
			// is false whenever the bit lengths disagree.
			if addr := prefix.Addr(); addr.Is4In6() {
				if p4, perr := addr.Unmap().Prefix(prefix.Bits() - 96); perr == nil {
					prefix = p4
				}
			}
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
	if slices.Contains(l.addrs, addr) {
		return true
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
