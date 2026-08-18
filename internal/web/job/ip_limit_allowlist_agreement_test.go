package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

// Save-time validation and scan-time parsing must accept exactly the same set:
// anything the validator lets through and the parser drops is silently unprotected.
func TestAllowlistValidatorAndParserAgree(t *testing.T) {
	for _, entry := range []string{
		"198.51.100.7",
		"198.51.100.0/24",
		"2001:db8::1",
		"2001:db8::/32",
		"198.51.100.0/024",
		"not-an-address",
		"198.51.100.0/33",
	} {
		accepted := entity.CheckNetipAddrOrPrefixList(entry, "invalid:") == nil
		parsed := len(parseIpLimitAllowlist(entry).prefixes)+len(parseIpLimitAllowlist(entry).addrs) > 0
		if accepted != parsed {
			t.Errorf("%q: validator=%v parser=%v — a disagreement leaves the entry silently unprotected", entry, accepted, parsed)
		}
	}
}

// An IPv4-mapped prefix used to parse but never match, because contains() unmaps
// the queried address and Prefix.Contains is false across bit lengths.
func TestAllowlistMatchesIPv4MappedPrefix(t *testing.T) {
	list := parseIpLimitAllowlist("::ffff:198.51.100.0/120")
	if !list.contains("198.51.100.5") {
		t.Fatal("an IPv4-mapped entry matched nothing: it protects no one")
	}
}
