package netsafe

import (
	"context"
	"strings"
	"testing"
)

// TestSSRFGuardedDialContext_LiteralIPSkipsResolver pins the netsafe.go:37
// decision (`if ip := net.ParseIP(host); ip != nil`). The string "fe80::1%eth0"
// is rejected by net.ParseIP (returns nil) but accepted by the resolver, which
// yields the link-local address fe80::1. With the branch intact, ParseIP returns
// nil so the host falls through to LookupIPAddr, resolves to fe80::1, and is
// blocked by IsBlockedIP -> the error mentions the resolved blocked address.
// If the condition is flipped to `ip == nil`, the nil-IP literal path is taken
// instead: ips = [{IP: nil}], IsBlockedIP(nil) is false, the guard never fires
// and the error would never say "blocked private/internal address fe80::1".
func TestSSRFGuardedDialContext_LiteralIPSkipsResolver(t *testing.T) {
	_, err := SSRFGuardedDialContext(context.Background(), "tcp", "[fe80::1%eth0]:80")
	if err == nil {
		t.Fatal("expected error for link-local host with zone suffix")
	}
	if !strings.Contains(err.Error(), "blocked private/internal address fe80::1") {
		t.Fatalf("expected guard to block resolved link-local fe80::1, got: %v", err)
	}
}

// TestSSRFGuardedDialContext_LiteralPrivateIPv6Blocked complements the above by
// confirming that a valid IP literal (parsed by the line 37 branch) is still run
// through IsBlockedIP and rejected with the literal in the message.
func TestSSRFGuardedDialContext_LiteralPrivateIPv6Blocked(t *testing.T) {
	_, err := SSRFGuardedDialContext(context.Background(), "tcp", "[::1]:80")
	if err == nil {
		t.Fatal("expected dial to ::1 to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked private/internal address ::1") {
		t.Fatalf("expected '::1' literal in blocked error, got: %v", err)
	}
}

// TestNormalizeHost_LengthBoundary pins the netsafe.go:76 length check
// (`len(addr) > 253`). A valid-pattern hostname of exactly 253 chars must be
// accepted (kills `>` -> `>=` / off-by-one mutations of the bound), while the
// same hostname at 254 chars must be rejected.
func TestNormalizeHost_LengthBoundary(t *testing.T) {
	label := strings.Repeat("a", 61)
	base := label + "." + label + "." + label + "." // 186 chars, valid pattern
	h253 := base + strings.Repeat("a", 253-len(base))
	if len(h253) != 253 {
		t.Fatalf("test setup: expected 253-char host, got %d", len(h253))
	}
	h254 := h253 + "a"

	got, err := NormalizeHost(h253)
	if err != nil {
		t.Fatalf("NormalizeHost(253-char valid host) returned error: %v", err)
	}
	if got != h253 {
		t.Fatalf("NormalizeHost(253-char host) = %q, want unchanged input", got)
	}

	if _, err := NormalizeHost(h254); err == nil {
		t.Fatal("NormalizeHost(254-char host) expected error, got nil")
	}
}

// TestNormalizeHost_PatternClauseIndependentOfLength pins the OR in line 76:
// a short hostname (well under the 253 limit) that violates the pattern must
// still be rejected. If `||` were mutated to `&&`, this short-but-invalid host
// would slip through because the length clause is false.
func TestNormalizeHost_PatternClauseIndependentOfLength(t *testing.T) {
	cases := []string{
		"under_score.example.com",
		"bad host",
		"exa$mple.com",
		"-leadingdash.com",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if len(in) > 253 {
				t.Fatalf("test setup: %q should be short to isolate the pattern clause", in)
			}
			if _, err := NormalizeHost(in); err == nil {
				t.Fatalf("NormalizeHost(%q) expected error for invalid pattern, got nil", in)
			}
		})
	}
}

// TestNormalizeHost_ValidShortHostAccepted ensures a short valid-pattern host is
// accepted, so a mutation dropping the `!` on the pattern match (rejecting valid
// hosts) is caught alongside the rejection cases above.
func TestNormalizeHost_ValidShortHostAccepted(t *testing.T) {
	const in = "node-1.example.com"
	got, err := NormalizeHost(in)
	if err != nil {
		t.Fatalf("NormalizeHost(%q) returned error: %v", in, err)
	}
	if got != in {
		t.Fatalf("NormalizeHost(%q) = %q, want %q", in, got, in)
	}
}
