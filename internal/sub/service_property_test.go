package sub

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestProp_JoinHostPort_Bracketing asserts the RFC-3986 authority contract for any
// host/port: SplitHostPort must recover the (un-bracketed) host and the exact port,
// and an IPv6 literal is bracketed exactly once regardless of input brackets.
func TestProp_JoinHostPort_Bracketing(t *testing.T) {
	hosts := []string{
		"1.2.3.4", "example.com", "sub.host.test",
		"2001:db8::1", "[2001:db8::1]", "::1", "[::1]", "fe80::1%eth0",
	}
	rapid.Check(t, func(t *rapid.T) {
		host := rapid.SampledFrom(hosts).Draw(t, "host")
		port := rapid.IntRange(0, 65535).Draw(t, "port")

		out := joinHostPort(host, port)

		gotHost, gotPort, err := net.SplitHostPort(out)
		if err != nil {
			t.Fatalf("SplitHostPort(%q) failed: %v", out, err)
		}
		wantHost := strings.Trim(host, "[]")
		if gotHost != wantHost {
			t.Fatalf("host round-trip: joinHostPort(%q,%d)=%q -> host %q, want %q", host, port, out, gotHost, wantHost)
		}
		if gotPort != strconv.Itoa(port) {
			t.Fatalf("port round-trip: got %q, want %d (out=%q)", gotPort, port, out)
		}
		// An IPv6 literal (contains a colon in the host) must be bracketed once.
		if strings.Contains(wantHost, ":") {
			if strings.Count(out, "[") != 1 || strings.Count(out, "]") != 1 {
				t.Fatalf("IPv6 host not bracketed exactly once: %q", out)
			}
		}
	})
}

// TestProp_EncodeUserinfo_RoundTrip asserts encodeUserinfo produces a userinfo token
// that net/url parses back to the original password for ANY input — the contract that
// trojan/ss links rely on. A field-mapping mutant that mangles escaping breaks this.
func TestProp_EncodeUserinfo_RoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pw := rapid.String().Draw(t, "pw")
		raw := "trojan://" + encodeUserinfo(pw) + "@example.com:443"
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q) failed for pw=%q: %v", raw, pw, err)
		}
		if got := u.User.Username(); got != pw {
			t.Fatalf("userinfo round-trip mismatch: pw=%q got=%q", pw, got)
		}
	})
}

// TestProp_SplitLinkLines_Invariants asserts splitLinkLines never emits empty or
// untrimmed lines, and that re-splitting its own joined output is a fixed point.
func TestProp_SplitLinkLines_Invariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.String().Draw(t, "raw")
		out := splitLinkLines(raw)
		for i, line := range out {
			if line == "" {
				t.Fatalf("splitLinkLines emitted an empty line at %d for %q", i, raw)
			}
			if line != strings.TrimSpace(line) {
				t.Fatalf("splitLinkLines emitted an untrimmed line %q", line)
			}
		}
		rejoined := splitLinkLines(strings.Join(out, "\n"))
		if len(rejoined) != len(out) {
			t.Fatalf("not a fixed point: %d -> %d lines", len(out), len(rejoined))
		}
		for i := range out {
			if rejoined[i] != out[i] {
				t.Fatalf("fixed-point mismatch at %d: %q vs %q", i, out[i], rejoined[i])
			}
		}
	})
}
