package frontproxy

import (
	"net/http"
	"testing"
	"time"
)

func TestLoginAttemptsLocksAtThreshold(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newLoginAttempts()
	a.now = func() time.Time { return clock }

	for i := 0; i < 4; i++ {
		locked, _ := a.fail("1.2.3.4", 5, time.Minute)
		if locked {
			t.Fatalf("fail %d: locked out before threshold", i+1)
		}
	}
	locked, until := a.fail("1.2.3.4", 5, time.Minute)
	if !locked {
		t.Fatal("5th failure did not lock the key")
	}
	if want := clock.Add(time.Minute); !until.Equal(want) {
		t.Fatalf("until = %v, want %v", until, want)
	}
}

func TestLoginAttemptsCheckReflectsLockWithoutCountingAnAttempt(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newLoginAttempts()
	a.now = func() time.Time { return clock }

	for i := 0; i < 5; i++ {
		a.fail("1.2.3.4", 5, time.Minute)
	}
	if locked, _ := a.check("1.2.3.4"); !locked {
		t.Fatal("check did not report the key as locked")
	}
	// Advance past the ban: check must clear it without a fail() call.
	clock = clock.Add(time.Minute + time.Second)
	if locked, _ := a.check("1.2.3.4"); locked {
		t.Fatal("check still reports locked after the ban window elapsed")
	}
	// A single subsequent failure must not immediately re-lock: the count
	// was reset, not preserved across the expired lockout.
	if locked, _ := a.fail("1.2.3.4", 5, time.Minute); locked {
		t.Fatal("first failure after expiry locked out immediately")
	}
}

func TestLoginAttemptsLockClearsAfterExpiryOnFail(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newLoginAttempts()
	a.now = func() time.Time { return clock }

	for i := 0; i < 5; i++ {
		a.fail("5.6.7.8", 5, time.Minute)
	}
	clock = clock.Add(2 * time.Minute)
	// fail() itself must notice the expired lockout and start over, not
	// keep extending a ban that should already be lifted.
	for i := 0; i < 4; i++ {
		locked, _ := a.fail("5.6.7.8", 5, time.Minute)
		if locked {
			t.Fatalf("fail %d after expiry: locked before the new threshold", i+1)
		}
	}
}

func TestLoginAttemptsKeysAreIndependent(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newLoginAttempts()
	a.now = func() time.Time { return clock }

	for i := 0; i < 5; i++ {
		a.fail("1.1.1.1", 5, time.Minute)
	}
	if locked, _ := a.check("1.1.1.1"); !locked {
		t.Fatal("1.1.1.1 should be locked")
	}
	if locked, _ := a.check("2.2.2.2"); locked {
		t.Fatal("2.2.2.2 should be unaffected by 1.1.1.1's lockout")
	}
}

func TestClientKeyStripsPort(t *testing.T) {
	r := &http.Request{RemoteAddr: "203.0.113.7:54321"}
	if got := clientKey(r); got != "203.0.113.7" {
		t.Fatalf("clientKey = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientKeyFallsBackToRawOnUnparseableAddr(t *testing.T) {
	r := &http.Request{RemoteAddr: "not-a-valid-addr"}
	if got := clientKey(r); got != "not-a-valid-addr" {
		t.Fatalf("clientKey = %q, want the raw value unchanged", got)
	}
}
