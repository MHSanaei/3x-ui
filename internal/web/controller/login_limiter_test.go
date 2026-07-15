package controller

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoginLimiterBoundsMemoryUnderUsernameFlood(t *testing.T) {
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	for i := 0; i < loginLimitMaxRecords+100; i++ {
		limiter.registerFailure("1.2.3.4", "user-"+strconv.Itoa(i))
	}

	limiter.mu.Lock()
	n := len(limiter.attempts)
	limiter.mu.Unlock()

	if n > loginLimitMaxRecords {
		t.Fatalf("attempts map grew to %d, exceeding the %d ceiling under a username flood", n, loginLimitMaxRecords)
	}
}

func TestLoginLimiterEvictionSparesActiveBlocks(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.mu.Lock()
	for i := 0; i < loginLimitMaxRecords-1; i++ {
		limiter.attempts["victim-"+strconv.Itoa(i)] = &loginLimitRecord{blockedUntil: now.Add(10 * time.Minute)}
	}
	limiter.attempts["filler"] = &loginLimitRecord{failures: []time.Time{now}}
	limiter.mu.Unlock()

	if _, blocked := limiter.registerFailure("9.9.9.9", "newcomer"); blocked {
		t.Fatal("the eviction-triggering failure itself should not be blocked yet")
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	survivors := 0
	for key, record := range limiter.attempts {
		if strings.HasPrefix(key, "victim-") && now.Before(record.blockedUntil) {
			survivors++
		}
	}
	if survivors != loginLimitMaxRecords-1 {
		t.Fatalf("eviction under a full map dropped an actively-blocked record: %d/%d victims survived", survivors, loginLimitMaxRecords-1)
	}
}

func TestLoginLimiterBlocksAfterConfiguredFailures(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	limiter.now = func() time.Time { return now }

	for i := range 4 {
		if _, blocked := limiter.registerFailure("192.0.2.10", "Admin"); blocked {
			t.Fatalf("failure %d should not block yet", i+1)
		}
		if _, ok := limiter.allow("192.0.2.10", "admin"); !ok {
			t.Fatalf("failure %d should still allow login attempts", i+1)
		}
	}

	blockedUntil, blocked := limiter.registerFailure("192.0.2.10", "ADMIN")
	if !blocked {
		t.Fatal("fifth failure should start cooldown")
	}
	if want := now.Add(15 * time.Minute); !blockedUntil.Equal(want) {
		t.Fatalf("blocked until %s, want %s", blockedUntil, want)
	}
	if _, ok := limiter.allow("192.0.2.10", "admin"); ok {
		t.Fatal("login should be blocked during cooldown")
	}

	now = blockedUntil
	if _, ok := limiter.allow("192.0.2.10", "admin"); !ok {
		t.Fatal("login should be allowed after cooldown")
	}
}

func TestLoginLimiterPrunesOldFailuresAndResetsOnSuccess(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	limiter.now = func() time.Time { return now }

	for range 4 {
		limiter.registerFailure("192.0.2.10", "admin")
	}
	now = now.Add(6 * time.Minute)
	if _, blocked := limiter.registerFailure("192.0.2.10", "admin"); blocked {
		t.Fatal("old failures should be pruned outside the rolling window")
	}

	limiter.registerSuccess("192.0.2.10", "admin")
	for i := range 4 {
		if _, blocked := limiter.registerFailure("192.0.2.10", "admin"); blocked {
			t.Fatalf("success should reset previous failures; failure %d blocked", i+1)
		}
	}
}

func TestLoginLimiterSeparatesIPAndUsername(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	limiter.now = func() time.Time { return now }

	for range 5 {
		limiter.registerFailure("192.0.2.10", "admin")
	}
	if _, ok := limiter.allow("192.0.2.11", "admin"); !ok {
		t.Fatal("different IP should not be blocked")
	}
	if _, ok := limiter.allow("192.0.2.10", "other-admin"); !ok {
		t.Fatal("different username should not be blocked")
	}
}
