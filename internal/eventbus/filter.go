package eventbus

import (
	"sync"
	"time"
)

// RateLimiter prevents notification spam from flapping events.
type RateLimiter struct {
	mu        sync.Mutex
	lastSent  map[string]time.Time
	cooldown  time.Duration
	lastPrune time.Time
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
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	if now.Sub(r.lastSent[key]) < r.cooldown {
		return false
	}
	r.lastSent[key] = now
	return true
}

// pruneLocked drops keys whose cooldown has elapsed. Such an entry no longer
// affects Allow's result, so removing it is safe and keeps the map from
// retaining one entry per (eventType, source) ever seen. Throttled to once per
// cooldown so a busy bus doesn't sweep the whole map on every event.
func (r *RateLimiter) pruneLocked(now time.Time) {
	if now.Sub(r.lastPrune) < r.cooldown {
		return
	}
	r.lastPrune = now
	for k, v := range r.lastSent {
		if now.Sub(v) >= r.cooldown {
			delete(r.lastSent, k)
		}
	}
}
