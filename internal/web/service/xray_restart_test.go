package service

import (
	"testing"
	"time"
)

func TestRestartXrayRespectsManualStop(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() { isManuallyStopped.Store(false) })

	isManuallyStopped.Store(true)
	_ = (&XrayService{}).RestartXray(false)

	if !isManuallyStopped.Load() {
		t.Fatal("a non-forced restart cleared a deliberate manual stop and would revive xray")
	}
}

func TestApplyPendingRestartReArmsFlagOnFailure(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() {
		isManuallyStopped.Store(false)
		isNeedXrayRestart.Store(false)
	})
	isManuallyStopped.Store(false)

	svc := &XrayService{}
	svc.SetToNeedRestart()
	svc.ApplyPendingRestart()

	if !isNeedXrayRestart.Load() {
		t.Fatal("a failed restart must re-arm the need-restart flag so the pending config change is retried")
	}
}

// resetAmneziawgRelayResyncState clears the shared debounce/rate-limit state
// so each test below starts clean regardless of what ran before it.
func resetAmneziawgRelayResyncState() {
	amneziawgRelayResyncMu.Lock()
	defer amneziawgRelayResyncMu.Unlock()
	if amneziawgRelayResyncTimer != nil {
		amneziawgRelayResyncTimer.Stop()
		amneziawgRelayResyncTimer = nil
	}
	amneziawgRelayResyncLastFire = time.Time{}
}

func TestScheduleAmneziaWGRelayResyncSetsNeedRestartFlagImmediately(t *testing.T) {
	resetAmneziawgRelayResyncState()
	t.Cleanup(func() {
		isNeedXrayRestart.Store(false)
		resetAmneziawgRelayResyncState()
	})
	isNeedXrayRestart.Store(false)

	(&XrayService{}).ScheduleAmneziaWGRelayResync()

	// Must be set synchronously -- a caller checking right after this call
	// must already see the pending change, not only once the timer fires.
	if !isNeedXrayRestart.Load() {
		t.Fatal("expected the need-restart flag to be set immediately, not only once the debounce timer fires")
	}
}

// Two pending timers would mean two restarts scheduled for one burst of
// edits, so a second call must cancel the first, not leave both live.
func TestScheduleAmneziaWGRelayResyncCoalescesRapidCalls(t *testing.T) {
	resetAmneziawgRelayResyncState()
	t.Cleanup(func() {
		isNeedXrayRestart.Store(false)
		resetAmneziawgRelayResyncState()
	})

	svc := &XrayService{}
	svc.ScheduleAmneziaWGRelayResync()

	amneziawgRelayResyncMu.Lock()
	first := amneziawgRelayResyncTimer
	amneziawgRelayResyncMu.Unlock()
	if first == nil {
		t.Fatal("expected a debounce timer to be armed")
	}

	svc.ScheduleAmneziaWGRelayResync()

	amneziawgRelayResyncMu.Lock()
	second := amneziawgRelayResyncTimer
	amneziawgRelayResyncMu.Unlock()
	if second == nil {
		t.Fatal("expected a debounce timer to still be armed after the second call")
	}
	if second == first {
		t.Fatal("a second call must arm a fresh timer, not reuse the first")
	}
	// (*time.Timer).Stop returns false once a timer has already fired or
	// been stopped -- the observable proof the first was really cancelled.
	if first.Stop() {
		t.Fatal("the first call's timer should already have been stopped by the second call, not left pending")
	}
}

// Proves the timer really fires, not just gets armed -- substitutes
// amneziawgRelayResyncFire for a channel signal instead of the ambiguous flag.
func TestScheduleAmneziaWGRelayResyncFiresAfterDelay(t *testing.T) {
	resetAmneziawgRelayResyncState()
	origDelay, origFire := amneziawgRelayResyncDelay, amneziawgRelayResyncFire
	t.Cleanup(func() {
		amneziawgRelayResyncDelay, amneziawgRelayResyncFire = origDelay, origFire
		isNeedXrayRestart.Store(false)
		resetAmneziawgRelayResyncState()
	})
	amneziawgRelayResyncDelay = 10 * time.Millisecond
	fired := make(chan *XrayService, 1)
	amneziawgRelayResyncFire = func(s *XrayService) { fired <- s }

	svc := &XrayService{}
	svc.ScheduleAmneziaWGRelayResync()

	select {
	case got := <-fired:
		if got != svc {
			t.Fatal("the timer fired with a different *XrayService than the one that scheduled it")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("debounce timer never fired within the timeout")
	}
}

// A recent fire must not arm another timer -- repeated edits should fall
// back to the pre-existing 30s cron cadence, not restart once per edit.
func TestScheduleAmneziaWGRelayResyncRateLimitsRepeatedCalls(t *testing.T) {
	resetAmneziawgRelayResyncState()
	t.Cleanup(func() {
		isNeedXrayRestart.Store(false)
		resetAmneziawgRelayResyncState()
	})
	amneziawgRelayResyncMu.Lock()
	amneziawgRelayResyncLastFire = time.Now()
	amneziawgRelayResyncMu.Unlock()

	(&XrayService{}).ScheduleAmneziaWGRelayResync()

	amneziawgRelayResyncMu.Lock()
	armed := amneziawgRelayResyncTimer
	amneziawgRelayResyncMu.Unlock()
	if armed != nil {
		t.Fatal("a call right after a recent fire must not arm another timer")
	}
}
