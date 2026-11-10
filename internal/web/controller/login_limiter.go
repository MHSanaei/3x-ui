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
)

var defaultLoginLimiter = newLoginLimiter(loginLimitMaxFailures, loginLimitWindow, loginLimitCooldown)

type loginLimiter struct {
	mu          sync.Mutex
	now         func() time.Time
	maxFailures int
	window      time.Duration
	cooldown    time.Duration
	attempts    map[string]*loginLimitRecord
}

type loginLimitRecord struct {
	failures []time.Time
	// inFlight are admitted attempts whose outcome is not known yet.
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
	}
}

// admit reserves a slot for one login attempt, counting unverified attempts as
// failures so a parallel burst cannot outrun the limit. Denied until the returned time.
func (l *loginLimiter) admit(ip, username string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	key := loginLimitKey(ip, username)
	record := l.attempts[key]
	if record == nil {
		l.evictForRoom(now)
		record = &loginLimitRecord{}
		l.attempts[key] = record
	}
	if now.Before(record.blockedUntil) {
		return record.blockedUntil, false
	}
	record.blockedUntil = time.Time{}
	cutoff := now.Add(-l.window)
	record.failures = pruneLoginFailures(record.failures, cutoff)
	record.inFlight = pruneLoginFailures(record.inFlight, cutoff)
	if len(record.failures)+len(record.inFlight) >= l.maxFailures {
		return oldestLoginAttempt(record, now).Add(l.window), false
	}
	record.inFlight = append(record.inFlight, now)
	return time.Time{}, true
}

func (l *loginLimiter) registerFailure(ip, username string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	key := loginLimitKey(ip, username)
	record := l.attempts[key]
	if record == nil {
		l.evictForRoom(now)
		record = &loginLimitRecord{}
		l.attempts[key] = record
	}
	if len(record.inFlight) > 0 {
		record.inFlight = record.inFlight[1:]
	}
	record.failures = pruneLoginFailures(record.failures, now.Add(-l.window))
	record.failures = append(record.failures, now)
	if len(record.failures) >= l.maxFailures {
		record.failures = nil
		record.inFlight = nil
		record.blockedUntil = now.Add(l.cooldown)
		return record.blockedUntil, true
	}
	return time.Time{}, false
}

func (l *loginLimiter) registerSuccess(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, loginLimitKey(ip, username))
}

// evictForRoom keeps the attempts map bounded before inserting a new record.
// It first reclaims records that are no longer blocked and whose failures have
// aged out of the window; if the map is still at the ceiling (a genuine
// broad flood), it drops one arbitrary record so memory can never grow past the
// cap. Callers hold l.mu.
func (l *loginLimiter) evictForRoom(now time.Time) {
	if len(l.attempts) < loginLimitMaxRecords {
		return
	}
	cutoff := now.Add(-l.window)
	for key, record := range l.attempts {
		if now.Before(record.blockedUntil) {
			continue
		}
		record.failures = pruneLoginFailures(record.failures, cutoff)
		record.inFlight = pruneLoginFailures(record.inFlight, cutoff)
		if len(record.failures)+len(record.inFlight) == 0 {
			delete(l.attempts, key)
		}
	}
	if len(l.attempts) < loginLimitMaxRecords {
		return
	}
	for key, record := range l.attempts {
		if now.Before(record.blockedUntil) {
			continue
		}
		delete(l.attempts, key)
		return
	}
	for key := range l.attempts {
		delete(l.attempts, key)
		return
	}
}

func oldestLoginAttempt(record *loginLimitRecord, fallback time.Time) time.Time {
	oldest := fallback
	for _, attempts := range [][]time.Time{record.failures, record.inFlight} {
		if len(attempts) > 0 && attempts[0].Before(oldest) {
			oldest = attempts[0]
		}
	}
	return oldest
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
