package job

import "testing"

// Addresses in the examples below come from the documentation ranges reserved
// by RFC 5737 and RFC 3849.
func TestIpLimitAllowlistMatchesAddressesAndNetworks(t *testing.T) {
	list := parseIpLimitAllowlist("203.0.113.10, 198.51.100.0/24\n2001:db8::/32 ; not-an-ip")

	for _, ip := range []string{"203.0.113.10", "198.51.100.7", "2001:db8::1"} {
		if !list.contains(ip) {
			t.Fatalf("%s should be allowlisted", ip)
		}
	}
	for _, ip := range []string{"203.0.113.11", "192.0.2.5", "2001:db9::1", ""} {
		if list.contains(ip) {
			t.Fatalf("%s must not be allowlisted", ip)
		}
	}
}

// A typo must not disable the limit for everybody, so an unparsable entry is
// dropped and the rest of the list keeps working.
func TestIpLimitAllowlistIgnoresUnparsableEntries(t *testing.T) {
	list := parseIpLimitAllowlist("nonsense, 203.0.113.0/24")
	if !list.contains("203.0.113.5") {
		t.Fatal("a valid entry stopped working because a neighbouring one was malformed")
	}
	if list.contains("192.0.2.1") {
		t.Fatal("a malformed entry must not widen the allowlist")
	}
	if parseIpLimitAllowlist("nonsense").empty() != true {
		t.Fatal("a list of only malformed entries must be empty, not permissive")
	}
}

// The point of the setting: a shared address is neither banned nor counted, so
// the office NAT it protects does not consume the client's limit either.
func TestIpLimitAllowlistSplitKeepsAllowedOutOfTheCount(t *testing.T) {
	live := []IPWithTimestamp{
		{IP: "203.0.113.10", Timestamp: 1},
		{IP: "192.0.2.1", Timestamp: 2},
		{IP: "192.0.2.2", Timestamp: 3},
	}
	list := parseIpLimitAllowlist("203.0.113.10")

	limited, allowed := list.split(live)
	if len(allowed) != 1 || allowed[0].IP != "203.0.113.10" {
		t.Fatalf("allowed = %v, want the allowlisted address alone", allowed)
	}
	if len(limited) != 2 {
		t.Fatalf("limited = %v, want the two ordinary addresses", limited)
	}

	kept, banned := selectIpsToBan(limited, 2)
	if len(banned) != 0 {
		t.Fatalf("banned = %v, want none: the allowlisted address must not push an ordinary one over the limit", banned)
	}
	if len(kept) != 2 {
		t.Fatalf("kept = %v, want both ordinary addresses", kept)
	}
}

func TestIpLimitAllowlistEmptyListChangesNothing(t *testing.T) {
	live := []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: 1}, {IP: "192.0.2.2", Timestamp: 2}}
	limited, allowed := parseIpLimitAllowlist("").split(live)
	if allowed != nil || len(limited) != 2 {
		t.Fatalf("empty allowlist changed the input: limited=%v allowed=%v", limited, allowed)
	}
}
