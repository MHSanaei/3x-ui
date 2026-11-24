package controller

import (
	"strings"
	"sync"
	"time"
)

const (
	loginLimitMaxFailures = 5
	loginLimitWindow      = 5 * time.Minute
	loginLimitCooldown    = 15 * time.Minute
	// Hard ceiling on tracked (ip, username) records. The key includes the
	// caller-supplied username, so an unauthenticated attacker rotating
	// usernames would otherwise grow the map without bound.
	loginLimitMaxRecords = 10000
	// Failures one address may spread over all usernames, as a multiple of the per-username
	// limit, before the address is blocked as a whole: bounds the records one peer can own.
	loginLimitIPFactor = 5
)

var defaultLoginLimiter = newLoginLimiter(loginLimitMaxFailures, loginLimitWindow, loginLimitCooldown)

type loginLimiter struct {
	mu          sync.Mutex
	now         func() time.Time
	maxFailures int
	window      time.Duration
	cooldown    time.Duration
	attempts    map[string]*loginLimitRecord
	ipAttempts  map[string]*loginLimitRecord
}

// loginLimitRecord counts failures and admitted attempts still being checked: a burst
// sent at once would otherwise all pass before bcrypt reports the first failure.
type loginLimitRecord struct {
	failures     []time.Time
	inFlight     []time.Time
	blockedUntil time.Time
}

func newLoginLimiter(maxFailures int, window, cooldown time.Duration) *loginLimiter {
	return &loginLimiter{
		now:         time.Now,
		maxFailures: maxFailures,
		window:      window,
		cooldown:    cooldown,
		attempts:    make(map[string]*loginLimitRecord),
		ipAttempts:  make(map[string]*loginLimitRecord),
	}
}

// allow admits an attempt and reserves a slot for it in both the (ip, username) and the
// address-wide record; the caller reports the outcome through registerFailure or registerSuccess.
func (l *loginLimiter) allow(ip, username string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	ipKey := strings.TrimSpace(ip)
	if blockedUntil, ok := l.admit(l.ipAttempts, ipKey, l.maxFailures*loginLimitIPFactor, now); !ok {
		return blockedUntil, false
	}
	if blockedUntil, ok := l.admit(l.attempts, loginLimitKey(ip, username), l.maxFailures, now); !ok {
		l.ipAttempts[ipKey].dropReservation()
		return blockedUntil, false
	}
	return time.Time{}, true
}

// admit refuses a blocked key, or one whose failures and pending attempts already fill the
// budget (starting its cooldown), and otherwise reserves a slot. Callers hold l.mu.
func (l *loginLimiter) admit(records map[string]*loginLimitRecord, key string, limit int, now time.Time) (time.Time, bool) {
	record := records[key]
	if record == nil {
		l.evictForRoom(records, now)
		record = &loginLimitRecord{}
		records[key] = record
	}
	if now.Before(record.blockedUntil) {
		return record.blockedUntil, false
	}
	record.blockedUntil = time.Time{}
	record.prune(now.Add(-l.window))
	if len(record.failures)+len(record.inFlight) >= limit {
		record.block(now.Add(l.cooldown))
		return record.blockedUntil, false
	}
	record.inFlight = append(record.inFlight, now)
	return time.Time{}, true
}

func (l *loginLimiter) registerFailure(ip, username string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	ipBlockedUntil, ipBlocked := l.fail(l.ipAttempts, strings.TrimSpace(ip), l.maxFailures*loginLimitIPFactor, now)
	blockedUntil, blocked := l.fail(l.attempts, loginLimitKey(ip, username), l.maxFailures, now)
	if blocked {
		return blockedUntil, true
	}
	return ipBlockedUntil, ipBlocked
}

func (l *loginLimiter) fail(records map[string]*loginLimitRecord, key string, limit int, now time.Time) (time.Time, bool) {
	record := records[key]
	if record == nil {
		l.evictForRoom(records, now)
		record = &loginLimitRecord{}
		records[key] = record
	}
	record.prune(now.Add(-l.window))
	record.dropReservation()
	record.failures = append(record.failures, now)
	if len(record.failures) >= limit {
		record.block(now.Add(l.cooldown))
		return record.blockedUntil, true
	}
	return time.Time{}, false
}

func (l *loginLimiter) registerSuccess(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, loginLimitKey(ip, username))
	if record := l.ipAttempts[strings.TrimSpace(ip)]; record != nil {
		record.dropReservation()
	}
}

func (r *loginLimitRecord) prune(cutoff time.Time) {
	r.failures = pruneLoginFailures(r.failures, cutoff)
	r.inFlight = pruneLoginFailures(r.inFlight, cutoff)
}

func (r *loginLimitRecord) block(until time.Time) {
	r.failures = nil
	r.inFlight = nil
	r.blockedUntil = until
}

func (r *loginLimitRecord) dropReservation() {
	if r != nil && len(r.inFlight) > 0 {
		r.inFlight = r.inFlight[1:]
	}
}

// evictForRoom keeps a records map bounded before inserting a new record.
// It first reclaims records that are no longer blocked and whose failures have
// aged out of the window; if the map is still at the ceiling (a genuine
// broad flood), it drops one record that is neither blocked nor awaiting an
// outcome, then any, so memory can never grow past the cap. Callers hold l.mu.
func (l *loginLimiter) evictForRoom(records map[string]*loginLimitRecord, now time.Time) {
	if len(records) < loginLimitMaxRecords {
		return
	}
	cutoff := now.Add(-l.window)
	for key, record := range records {
		if now.Before(record.blockedUntil) {
			continue
		}
		record.prune(cutoff)
		if len(record.failures) == 0 && len(record.inFlight) == 0 {
			delete(records, key)
		}
	}
	if len(records) < loginLimitMaxRecords {
		return
	}
	for key, record := range records {
		if now.Before(record.blockedUntil) || len(record.inFlight) > 0 {
			continue
		}
		delete(records, key)
		return
	}
	for key := range records {
		delete(records, key)
		return
	}
}

func loginLimitKey(ip, username string) string {
	return strings.TrimSpace(ip) + "\x00" + strings.ToLower(strings.TrimSpace(username))
}

func pruneLoginFailures(failures []time.Time, cutoff time.Time) []time.Time {
	keepFrom := 0
	for keepFrom < len(failures) && failures[keepFrom].Before(cutoff) {
		keepFrom++
	}
	return failures[keepFrom:]
}
