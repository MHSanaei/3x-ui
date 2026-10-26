package sub

import (
	"container/list"
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	// subRateLimitDefaultPerMinute is the fallback per-client limit when the
	// stored setting is unreadable or zero.
	subRateLimitDefaultPerMinute = 60
	// subRateLimitWindow is the fixed window one subRateLimiter counts over.
	subRateLimitWindow = time.Minute
	// subRateLimitMaxKeys bounds tracked client buckets. Each bucket holds at
	// most `limit` timestamps, so the worst-case map stays proportional to the
	// configured limit.
	subRateLimitMaxKeys = 4096
	// subRateLimitIdleTTL drops buckets for clients that stopped requesting,
	// so scanners rotating source addresses cannot pin the map at its ceiling
	// forever.
	subRateLimitIdleTTL = 20 * time.Minute
)

// subRateLimiter is a fixed-capacity sliding-window rate limiter keyed by
// client identity. Every subscription route passes through one instance: the
// subscription body is cheap to serve but its reads are not free, and a
// subId probe loop must not turn into a database load generator.
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

// allow records one request for key and reports whether it is under the
// window limit. When it is not, retryAfter is the time until the oldest hit
// in the window ages out.
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

// purgeIdleLocked drops buckets whose client has not been seen for idleTTL.
// The LRU front is the least recently used, so the scan stops at the first
// fresh entry. Callers hold l.mu.
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

// evictOldestLocked drops one least-recently-used bucket to make room. It
// only runs when the map is at capacity, i.e. under a genuine flood of
// distinct identities. Callers hold l.mu.
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

// clientIPResolver derives the identity the rate limiter keys on. It keeps
// the panel's login-limiter semantics (internal/web/controller/util.go
// getRemoteIp): the peer address, unless the peer is a trusted proxy, in
// which case X-Forwarded-For is walked from the proxy end and a
// client-supplied X-Real-IP is only believed when it agrees with the
// forwarded chain. A spoofed header behind a trusted proxy therefore falls
// back to the proxy's own address instead of poisoning the bucket key.
type clientIPResolver struct {
	trusted []netip.Prefix
}

// newClientIPResolver parses the panel's trustedProxyCIDRs setting once per
// subscription server start. An unset, empty, or invalid value falls back to
// loopback only — a wildcard or malformed entry must never widen trust.
func newClientIPResolver(settingService service.SettingService) *clientIPResolver {
	trusted := service.DefaultTrustedProxyCIDRs
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			trusted = trimmed
		}
	}
	prefixes, err := parseTrustedCIDRs(trusted)
	if err != nil {
		logger.Warning("sub: invalid trusted proxy CIDRs, falling back to loopback:", err)
		prefixes = loopbackTrustedCIDRs()
	}
	return &clientIPResolver{trusted: prefixes}
}

func newLoopbackIPResolver() *clientIPResolver {
	return &clientIPResolver{trusted: loopbackTrustedCIDRs()}
}

func loopbackTrustedCIDRs() []netip.Prefix {
	prefixes, err := parseTrustedCIDRs(service.DefaultTrustedProxyCIDRs)
	if err != nil {
		// The default is a compile-time constant list; this cannot happen.
		return nil
	}
	return prefixes
}

func parseTrustedCIDRs(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "*") {
			return nil, errors.New("trusted proxy CIDRs must not trust every address")
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, err
		}
		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}

func (r *clientIPResolver) isTrusted(addr netip.Addr) bool {
	for _, prefix := range r.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (r *clientIPResolver) resolve(remoteAddr, xff, xRealIP string) string {
	remoteIP, ok := extractIP(remoteAddr)
	if !ok {
		return "unknown"
	}
	if !r.isTrusted(remoteIP) {
		return remoteIP.String()
	}

	forwardedIP, hasForwarded := r.forwardedForClient(xff)
	realIP, hasRealIP := extractIP(xRealIP)
	switch {
	case hasRealIP && hasForwarded && realIP != forwardedIP:
		// A proxy rewrites at most one of the two, so the other is the
		// client's own claim; neither may pick the bucket key.
		return remoteIP.String()
	case hasForwarded:
		return forwardedIP.String()
	case hasRealIP:
		return realIP.String()
	}
	return remoteIP.String()
}

// forwardedForClient walks X-Forwarded-For from the proxy end: entries a
// client sent come first, so the first hop that is not a trusted proxy is the
// address one saw. Mirrors the panel's forwardedForClient.
func (r *clientIPResolver) forwardedForClient(xff string) (netip.Addr, bool) {
	parts := strings.Split(xff, ",")
	var client netip.Addr
	found := false
	for i := len(parts) - 1; i >= 0; i-- {
		ip, ok := extractIP(parts[i])
		if !ok {
			break
		}
		client = ip
		found = true
		if !r.isTrusted(ip) {
			break
		}
	}
	return client, found
}

func extractIP(value string) (netip.Addr, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return netip.Addr{}, false
	}
	if ip, err := netip.ParseAddr(candidate); err == nil {
		return ip.Unmap(), true
	}
	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip, err := netip.ParseAddr(host); err == nil {
			return ip.Unmap(), true
		}
	}
	if strings.Count(candidate, ":") == 1 {
		if host, _, err := net.SplitHostPort("[" + candidate + "]"); err == nil {
			if ip, err := netip.ParseAddr(host); err == nil {
				return ip.Unmap(), true
			}
		}
	}
	return netip.Addr{}, false
}

// subRateLimitGuard rejects requests over the per-client subscription limit
// with 429 and a Retry-After hint. A nil limiter disables the guard: direct
// NewSUBController users (tests, embedders) keep the historical behavior.
func subRateLimitGuard(limiter *subRateLimiter, resolver *clientIPResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		resolverForGuard := resolver
		if resolverForGuard == nil {
			resolverForGuard = newLoopbackIPResolver()
		}
		ip := resolverForGuard.resolve(c.Request.RemoteAddr, c.GetHeader("X-Forwarded-For"), c.GetHeader("X-Real-IP"))
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
