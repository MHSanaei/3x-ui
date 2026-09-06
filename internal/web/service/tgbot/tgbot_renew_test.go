package tgbot

import (
	"testing"
	"time"
)

// A mute is stored against the expiry it was made for. An admin extending the
// client must void it, or a customer who muted once is never chased again.
func TestMuteCoversExpiry(t *testing.T) {
	tests := []struct {
		name   string
		stored int64
		found  bool
		expiry int64
		want   bool
	}{
		{name: "never muted", stored: 0, found: false, expiry: 1700, want: false},
		{name: "muted for this expiry", stored: 1700, found: true, expiry: 1700, want: true},
		{name: "client was extended", stored: 1700, found: true, expiry: 1900, want: false},
		{name: "client was shortened", stored: 1900, found: true, expiry: 1700, want: false},
		{name: "stored zero but absent", stored: 0, found: false, expiry: 0, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := muteCovers(tc.stored, tc.found, tc.expiry); got != tc.want {
				t.Fatalf("muteCovers(%d, %v, %d) = %v, want %v", tc.stored, tc.found, tc.expiry, got, tc.want)
			}
		})
	}
}

// A renewal request pings every admin, so one customer tapping repeatedly must
// not become a way to flood them.
func TestWithinCooldown(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	tests := []struct {
		name string
		last int64
		want bool
	}{
		{name: "never asked", last: 0, want: false},
		{name: "asked a minute ago", last: now.Add(-time.Minute).Unix(), want: true},
		{name: "asked 23h ago", last: now.Add(-23 * time.Hour).Unix(), want: true},
		{name: "asked 25h ago", last: now.Add(-25 * time.Hour).Unix(), want: false},
		{name: "clock went backwards", last: now.Add(time.Hour).Unix(), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinCooldown(tc.last, now, day); got != tc.want {
				t.Fatalf("withinCooldown(%d, %v, %v) = %v, want %v", tc.last, now, day, got, tc.want)
			}
		})
	}
}

// The mark vocabulary is what an admin reads the daily summary by, so a muted
// client must be distinguishable from one Telegram refused.
func TestDeliveryMutedMark(t *testing.T) {
	marks := map[deliveryOutcome]string{
		deliveryNone:      "",
		deliveryUnlinked:  "➖ ",
		deliveryFailed:    "❌ ",
		deliveryDelivered: "✅ ",
		deliveryMuted:     "🔕 ",
	}
	seen := map[string]deliveryOutcome{}
	for outcome, want := range marks {
		if got := outcome.mark(); got != want {
			t.Fatalf("outcome %d mark = %q, want %q", outcome, got, want)
		}
		if other, dup := seen[want]; dup && want != "" {
			t.Fatalf("outcomes %d and %d share the mark %q", other, outcome, want)
		}
		seen[want] = outcome
	}
}

// The reminder keyboard is the customer's only way to act, and both buttons
// address one client, so both must carry the email the ownership check reads.
func TestRenewKeyboardTargetsTheClient(t *testing.T) {
	tg := &Tgbot{}
	data := callbackData(tg.renewKeyboard("amy@example.com"))

	for _, want := range []string{"renew_req amy@example.com", "renew_mute amy@example.com"} {
		if !contains(data, want) {
			t.Fatalf("renew keyboard is missing %q: %v", want, data)
		}
	}
}
