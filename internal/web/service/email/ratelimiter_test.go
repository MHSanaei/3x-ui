package email

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := eventbus.NewRateLimiter(time.Minute)

	if !rl.Allow(eventbus.EventOutboundDown, "proxy-1") {
		t.Error("first call should be allowed")
	}
}

func TestRateLimiterCooldown(t *testing.T) {
	rl := eventbus.NewRateLimiter(100 * time.Millisecond)

	rl.Allow(eventbus.EventOutboundDown, "proxy-1")

	if rl.Allow(eventbus.EventOutboundDown, "proxy-1") {
		t.Error("should be blocked during cooldown")
	}

	time.Sleep(110 * time.Millisecond)

	if !rl.Allow(eventbus.EventOutboundDown, "proxy-1") {
		t.Error("should be allowed after cooldown")
	}
}

func TestRateLimiterPerType(t *testing.T) {
	rl := eventbus.NewRateLimiter(time.Minute)

	rl.Allow(eventbus.EventOutboundDown, "proxy-1")

	if !rl.Allow(eventbus.EventOutboundUp, "proxy-1") {
		t.Error("different event types should be independent")
	}
}

func TestRateLimiterPerSource(t *testing.T) {
	rl := eventbus.NewRateLimiter(time.Minute)

	rl.Allow(eventbus.EventOutboundDown, "proxy-1")

	if !rl.Allow(eventbus.EventOutboundDown, "proxy-2") {
		t.Error("different sources should be independent")
	}
}
