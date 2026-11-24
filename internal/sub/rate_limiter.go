package sub

import (
	"container/list"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientip"
)

const (
	// subRateLimitDefaultPerMinute is the fallback per-client limit when the stored setting is unreadable or zero.
	subRateLimitDefaultPerMinute = 60
	// subRateLimitMaxPerMinute clamps the read path: a hand-edited DB row must not scale one bucket to unbounded memory.
	subRateLimitMaxPerMinute = 1000
	// subRateLimitWindow is the fixed window one subRateLimiter counts over.
	subRateLimitWindow = time.Minute
	// subRateLimitMaxKeys bounds tracked client buckets; each holds at most `limit` timestamps.
	subRateLimitMaxKeys = 4096
	// subRateLimitIdleTTL drops buckets for clients that stopped requesting.
	subRateLimitIdleTTL = 20 * time.Minute
)

// subRateLimiter is a fixed-capacity sliding-window rate limiter keyed by client identity: a subId
// probe loop must not turn into a database load generator.
type subRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	capacity int
	idleTTL  time.Duration
	now      func() time.Time

	buckets map[string]*subRateBucket
	lru     *list.List // front = least recently used key
}

type subRateBucket struct {
	hits     []time.Time
	lastSeen time.Time
	element  *list.Element
}

func newSubRateLimiter(limit int, window time.Duration) *subRateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = subRateLimitWindow
	}
	return &subRateLimiter{
		limit:    limit,
		window:   window,
		capacity: subRateLimitMaxKeys,
		idleTTL:  subRateLimitIdleTTL,
		now:      time.Now,
		buckets:  make(map[string]*subRateBucket),
		lru:      list.New(),
	}
}

// allow records one request for key and reports whether it is under the window limit. When it is
// not, retryAfter is the time until the oldest hit in the window ages out.
func (l *subRateLimiter) allow(key string) (retryAfter time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.purgeIdleLocked(now)

	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= l.capacity {
			l.evictOldestLocked()
		}
		b = &subRateBucket{}
		b.element = l.lru.PushBack(key)
		l.buckets[key] = b
	} else {
		l.lru.MoveToBack(b.element)
	}

	cutoff := now.Add(-l.window)
	keep := 0
	for keep < len(b.hits) && b.hits[keep].Before(cutoff) {
		keep++
	}
	b.hits = b.hits[keep:]
	b.lastSeen = now

	if len(b.hits) >= l.limit {
		return b.hits[0].Add(l.window).Sub(now), false
	}
	b.hits = append(b.hits, now)
	return 0, true
}

// purgeIdleLocked drops buckets idle for longer than idleTTL; the LRU front is the least recently
// used, so the scan stops at the first fresh entry. Callers hold l.mu.
func (l *subRateLimiter) purgeIdleLocked(now time.Time) {
	cutoff := now.Add(-l.idleTTL)
	for l.lru.Len() > 0 {
		front := l.lru.Front()
		key := front.Value.(string)
		if l.buckets[key].lastSeen.After(cutoff) {
			return
		}
		l.removeLocked(front, key)
	}
}

// evictOldestLocked drops one least-recently-used bucket to make room; it only runs at capacity,
// i.e. under a flood of distinct identities. Callers hold l.mu.
func (l *subRateLimiter) evictOldestLocked() {
	front := l.lru.Front()
	if front == nil {
		return
	}
	l.removeLocked(front, front.Value.(string))
}

func (l *subRateLimiter) removeLocked(element *list.Element, key string) {
	l.lru.Remove(element)
	delete(l.buckets, key)
}

// subRateLimitGuard rejects over-limit requests with 429 + Retry-After, keyed on the panel login
// limiter's client identity. A nil limiter is a no-op for direct NewSUBController users.
func subRateLimitGuard(limiter *subRateLimiter, trustedProxyCIDRs string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		ip := clientip.Resolve(c.Request.RemoteAddr, c.GetHeader("X-Real-IP"), c.GetHeader("X-Forwarded-For"), trustedProxyCIDRs)
		if retryAfter, ok := limiter.allow(ip); !ok {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			logger.Debugf("sub: rate limit hit for %s, retry after %ds", ip, seconds)
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}
