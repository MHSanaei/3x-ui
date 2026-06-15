package eventbus

import (
	"sync"
	"time"
)

// RateLimiter prevents notification spam from flapping events.
type RateLimiter struct {
	mu       sync.Mutex
	lastSent map[string]time.Time
	cooldown time.Duration
}

// NewRateLimiter creates a rate limiter with the given cooldown period.
func NewRateLimiter(cooldown time.Duration) *RateLimiter {
	return &RateLimiter{
		lastSent: make(map[string]time.Time),
		cooldown: cooldown,
	}
}

// Allow returns true if the event should be sent (cooldown has elapsed).
func (r *RateLimiter) Allow(eventType EventType, source string) bool {
	key := string(eventType) + ":" + source
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.lastSent[key]) < r.cooldown {
		return false
	}
	r.lastSent[key] = time.Now()
	return true
}
