package sub

import (
	"net"
	"strings"
	"testing"
)

func TestIsCIDR(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "192.168.1.0/24", want: true},
		{input: "10.0.0.0/8", want: true},
		{input: "[192.168.1.0/24]", want: true},
		{input: "2a01:4f8:1c1b:729::/64", want: true},
		{input: "[2a01:4f8:1c1b:729::/64]", want: true},
		{input: "fe80::/10", want: true},
		{input: "example.com", want: false},
		{input: "192.168.1.1", want: false},
		{input: "2001:db8::1", want: false},
		{input: "", want: false},
		{input: "invalid/33", want: false},
		{input: "999.999.999.999/24", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := IsCIDR(tc.input); got != tc.want {
				t.Fatalf("IsCIDR(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestRandomIPsFromCIDR_IPv4(t *testing.T) {
	t.Run("single host /32", func(t *testing.T) {
		ips, err := RandomIPsFromCIDR("192.0.2.1/32", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 1 || ips[0] != "192.0.2.1" {
			t.Fatalf("RandomIPsFromCIDR(/32) = %v, want [192.0.2.1]", ips)
		}
	})

	t.Run("small subnet /30 caps count", func(t *testing.T) {
		ips, err := RandomIPsFromCIDR("198.51.100.0/30", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 2 {
			t.Fatalf("len(ips) = %d, want 2", len(ips))
		}
		for _, ipStr := range ips {
			if ipStr == "198.51.100.0" || ipStr == "198.51.100.3" {
				t.Fatalf("got network or broadcast address: %s", ipStr)
			}
		}
	})

	t.Run("subnet /24 generates unique IPs within range", func(t *testing.T) {
		cidr := "203.0.113.0/24"
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("parse cidr: %v", err)
		}

		ips, err := RandomIPsFromCIDR(cidr, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 10 {
			t.Fatalf("len(ips) = %d, want 10", len(ips))
		}

		seen := make(map[string]struct{}, len(ips))
		for _, ipStr := range ips {
			if _, dup := seen[ipStr]; dup {
				t.Fatalf("duplicate IP emitted: %s", ipStr)
			}
			seen[ipStr] = struct{}{}

			parsed := net.ParseIP(ipStr)
			if parsed == nil || !ipNet.Contains(parsed) {
				t.Fatalf("IP %s outside expected subnet %s", ipStr, cidr)
			}
			if strings.HasSuffix(ipStr, ".0") || strings.HasSuffix(ipStr, ".255") {
				t.Fatalf("unusable host IP picked: %s", ipStr)
			}
		}
	})
}

func TestRandomIPsFromCIDR_IPv6(t *testing.T) {
	t.Run("single host /128", func(t *testing.T) {
		ips, err := RandomIPsFromCIDR("2001:db8::1/128", 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 1 || ips[0] != "2001:db8::1" {
			t.Fatalf("RandomIPsFromCIDR(/128) = %v, want [2001:db8::1]", ips)
		}
	})

	t.Run("standard /64 datacenter block", func(t *testing.T) {
		cidr := "2a01:4f8:1c1b:729::/64"
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("parse cidr: %v", err)
		}

		ips, err := RandomIPsFromCIDR(cidr, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 10 {
			t.Fatalf("len(ips) = %d, want 10", len(ips))
		}

		seen := make(map[string]struct{}, len(ips))
		for _, ipStr := range ips {
			if _, dup := seen[ipStr]; dup {
				t.Fatalf("duplicate IP emitted: %s", ipStr)
			}
			seen[ipStr] = struct{}{}

			parsed := net.ParseIP(ipStr)
			if parsed == nil || !ipNet.Contains(parsed) {
				t.Fatalf("IP %s outside expected subnet %s", ipStr, cidr)
			}
			if ipStr == "2a01:4f8:1c1b:729::" {
				t.Fatalf("subnet-router anycast address was emitted")
			}
		}
	})

	t.Run("bracketed cidr notation", func(t *testing.T) {
		ips, err := RandomIPsFromCIDR("[2a01:4f8:1c1b:729::/64]", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 3 {
			t.Fatalf("len(ips) = %d, want 3", len(ips))
		}
	})
}

func TestRandomIPsFromCIDR_Invalid(t *testing.T) {
	if _, err := RandomIPsFromCIDR("example.com", 1); err == nil {
		t.Fatalf("RandomIPsFromCIDR(domain) expected error, got nil")
	}
}
