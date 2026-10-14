package controller

import (
	"testing"
	"time"
)

func TestTelegramAuthRateLimitIPGroupsIPv6ByPrefix(t *testing.T) {
	first := telegramAuthRateLimitIP("2001:db8:1:2::1")
	second := telegramAuthRateLimitIP("2001:db8:1:2::ffff")
	if first != second {
		t.Fatalf("same IPv6 /64 has distinct rate keys: %q, %q", first, second)
	}
	if first == telegramAuthRateLimitIP("2001:db8:1:3::1") {
		t.Fatal("different IPv6 /64 shares a rate key")
	}
	if got := telegramAuthRateLimitIP("192.0.2.1"); got != "192.0.2.1" {
		t.Fatalf("IPv4 rate key = %q", got)
	}
}

func TestTelegramAuthCredentialLimiterSharesIPv6Prefix(t *testing.T) {
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	username := "admin"
	for _, ip := range []string{
		"2001:db8:1:2::1",
		"2001:db8:1:2::2",
		"2001:db8:1:2::3",
		"2001:db8:1:2::4",
		"2001:db8:1:2::5",
	} {
		limiter.registerFailure(telegramAuthRateLimitIP(ip), username)
	}
	if _, allowed := limiter.allow(telegramAuthRateLimitIP("2001:db8:1:2::6"), username); allowed {
		t.Fatal("credential attempts from the same IPv6 /64 bypassed the limit")
	}
	if _, allowed := limiter.allow(telegramAuthRateLimitIP("2001:db8:1:3::1"), username); !allowed {
		t.Fatal("credential attempts from a different IPv6 /64 were blocked")
	}
}
