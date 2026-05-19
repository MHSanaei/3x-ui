package netsafe

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.0.1", true},
		{"0.0.0.0", true},
		{"::", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2606:4700:4700::1111", false},
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			ip := net.ParseIP(c.ip)
			if ip == nil {
				t.Fatalf("could not parse %q", c.ip)
			}
			if got := IsBlockedIP(ip); got != c.want {
				t.Fatalf("IsBlockedIP(%s) = %v, want %v", c.ip, got, c.want)
			}
		})
	}
}

func TestAllowPrivateFromContext_Default(t *testing.T) {
	if AllowPrivateFromContext(context.Background()) {
		t.Fatal("default context should report AllowPrivate=false")
	}
}

func TestAllowPrivateFromContext_RoundTrip(t *testing.T) {
	ctx := ContextWithAllowPrivate(context.Background(), true)
	if !AllowPrivateFromContext(ctx) {
		t.Fatal("expected AllowPrivate=true after ContextWithAllowPrivate(true)")
	}
	ctx = ContextWithAllowPrivate(ctx, false)
	if AllowPrivateFromContext(ctx) {
		t.Fatal("expected AllowPrivate=false after overriding with false")
	}
}

func TestNormalizeHost_Valid(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"example.com", "example.com"},
		{"  example.com  ", "example.com"},
		{"a.b.c.example.com", "a.b.c.example.com"},
		{"10.0.0.1", "10.0.0.1"},
		{"[2606:4700:4700::1111]", "2606:4700:4700::1111"},
		{"2606:4700:4700::1111", "2606:4700:4700::1111"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := NormalizeHost(c.in)
			if err != nil {
				t.Fatalf("NormalizeHost(%q) returned error: %v", c.in, err)
			}
			if !strings.EqualFold(got, c.want) {
				t.Fatalf("NormalizeHost(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeHost_Invalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"-leading-dash.com",
		"trailing-dash-.com",
		"bad host with spaces",
		"under_score.example.com",
		"exa$mple.com",
		strings.Repeat("a", 254),
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := NormalizeHost(in); err == nil {
				t.Fatalf("NormalizeHost(%q) expected error, got nil", in)
			}
		})
	}
}

func TestSSRFGuardedDialContext_BlocksLiteralPrivateIP(t *testing.T) {
	_, err := SSRFGuardedDialContext(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected dial to 127.0.0.1 to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected 'blocked' in error, got: %v", err)
	}
}

func TestSSRFGuardedDialContext_AllowPrivateBypassesGuard(t *testing.T) {
	ctx := ContextWithAllowPrivate(context.Background(), true)
	_, err := SSRFGuardedDialContext(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("dial to a closed loopback port should still fail at the connect step")
	}
	if strings.Contains(err.Error(), "blocked private/internal address") {
		t.Fatalf("expected guard to be bypassed when AllowPrivate=true, got: %v", err)
	}
}

func TestSSRFGuardedDialContext_BadAddress(t *testing.T) {
	if _, err := SSRFGuardedDialContext(context.Background(), "tcp", "no-port"); err == nil {
		t.Fatal("expected error for address without port")
	}
}
