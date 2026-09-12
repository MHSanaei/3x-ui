package tgbot

import (
	"testing"
	"time"
)

// The reset knocks every one of a client's devices offline, so an unreadable setting
// must fail closed — unlike the notification toggles, where a spurious notice is harmless.
func TestSelfResetFailsClosed(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	if tg.selfResetEnabled() {
		t.Fatal("self-reset is enabled on a stock panel")
	}
	if err := tg.settingService.SetTgBotAllowSelfReset(true); err != nil {
		t.Fatalf("SetTgBotAllowSelfReset: %v", err)
	}
	if !tg.selfResetEnabled() {
		t.Fatal("self-reset stayed closed after an admin enabled it")
	}
}

// Three independent gates. Each is re-checked in the handler, so a stale button
// or a hand-crafted callback runs into the same wall the keyboard does.
func TestSelfResetGate(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "amy@x", "subamy00000000001", 777)
	seedClient(t, "bob@x", "subbob00000000002", 999)
	tg := new(Tgbot)

	t.Run("refused while the panel has it off", func(t *testing.T) {
		if allowed, _ := tg.selfResetGate(777, "amy@x"); allowed {
			t.Fatal("gate opened while tgBotAllowSelfReset is false")
		}
	})

	if err := tg.settingService.SetTgBotAllowSelfReset(true); err != nil {
		t.Fatalf("SetTgBotAllowSelfReset: %v", err)
	}

	t.Run("owner is allowed", func(t *testing.T) {
		if allowed, key := tg.selfResetGate(777, "amy@x"); !allowed {
			t.Fatalf("owner refused with %q", key)
		}
	})

	// Both refusals must be silent and identical, or the reply becomes an
	// oracle for whether an email names a real client.
	t.Run("another customer's client is refused", func(t *testing.T) {
		allowed, key := tg.selfResetGate(777, "bob@x")
		if allowed {
			t.Fatal("a customer reached another customer's config")
		}
		if key != "" {
			t.Fatalf("refusal explained itself with %q", key)
		}
	})

	t.Run("unknown client is refused the same way", func(t *testing.T) {
		allowed, key := tg.selfResetGate(777, "nobody@x")
		if allowed {
			t.Fatal("a customer reached a client that does not exist")
		}
		if key != "" {
			t.Fatalf("an unknown client is distinguishable from another customer's: %q", key)
		}
	})

	t.Run("no sender is refused", func(t *testing.T) {
		if allowed, _ := tg.selfResetGate(0, "amy@x"); allowed {
			t.Fatal("a callback with no sender passed the gate")
		}
	})

	t.Run("refused inside the cooldown", func(t *testing.T) {
		if err := selfResetAt.put(tg, "amy@x", time.Now().Unix()); err != nil {
			t.Fatalf("put: %v", err)
		}
		if allowed, _ := tg.selfResetGate(777, "amy@x"); allowed {
			t.Fatal("gate opened again inside the cooldown")
		}
	})

	t.Run("allowed once the cooldown lapses", func(t *testing.T) {
		stale := time.Now().Add(-selfResetCooldown - time.Minute).Unix()
		if err := selfResetAt.put(tg, "amy@x", stale); err != nil {
			t.Fatalf("put: %v", err)
		}
		if allowed, key := tg.selfResetGate(777, "amy@x"); !allowed {
			t.Fatalf("gate stayed shut after the cooldown lapsed: %q", key)
		}
	})
}
