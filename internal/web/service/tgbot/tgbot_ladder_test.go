package tgbot

import (
	"strings"
	"testing"
	"time"
)

func at(t *testing.T, layout string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", layout, time.Local)
	if err != nil {
		t.Fatalf("parse %q: %v", layout, err)
	}
	return parsed
}

// Rungs are exact calendar-day matches, not a rolling window: each one must fire
// on its own day and stay silent on every other, or a customer is nagged daily.
func TestExpiryRung(t *testing.T) {
	now := at(t, "2026-08-21 09:00")

	tests := []struct {
		name   string
		expiry time.Time
		want   renewalRung
	}{
		{"three days out", at(t, "2026-08-24 09:00"), rungThreeDays},
		{"three days out, late in the day", at(t, "2026-08-24 23:59"), rungThreeDays},
		{"two days out", at(t, "2026-08-23 09:00"), rungTwoDays},
		{"tomorrow", at(t, "2026-08-22 00:01"), rungTomorrow},
		{"later today", at(t, "2026-08-21 23:00"), rungToday},
		{"earlier today is still today", at(t, "2026-08-21 01:00"), rungToday},
		{"yesterday", at(t, "2026-08-20 23:00"), rungOverdue},
		{"long overdue", at(t, "2026-07-01 09:00"), rungOverdue},
		{"four days out", at(t, "2026-08-25 09:00"), rungNone},
		{"far future", at(t, "2027-01-01 09:00"), rungNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expiryRung(tc.expiry.UnixMilli(), now); got != tc.want {
				t.Fatalf("expiryRung(%s) = %v, want %v", tc.expiry.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// A zero expiry never lapses and a negative one has not started counting down,
// so neither belongs on a ladder about renewal dates.
func TestExpiryRungIgnoresSentinelExpiries(t *testing.T) {
	now := at(t, "2026-08-21 09:00")
	for _, expiry := range []int64{0, -2592000000} {
		if got := expiryRung(expiry, now); got != rungNone {
			t.Fatalf("expiryRung(%d) = %v, want rungNone", expiry, got)
		}
	}
}

// "Tomorrow" must mean the next date, not the next 24 hours: an expiry late
// tonight is due today, and one just after midnight is due tomorrow.
func TestExpiryRungUsesCalendarDaysNotElapsedHours(t *testing.T) {
	now := at(t, "2026-08-21 23:30")
	if got := expiryRung(at(t, "2026-08-22 00:30").UnixMilli(), now); got != rungTomorrow {
		t.Fatalf("an expiry one hour away but on the next date = %v, want rungTomorrow", got)
	}
	if got := expiryRung(at(t, "2026-08-21 23:59").UnixMilli(), now); got != rungToday {
		t.Fatalf("an expiry later tonight = %v, want rungToday", got)
	}
}

// The 2-day rung widens what the admin summary covers without adding a fourth
// reminder: it must carry a summary key and no customer message key.
func TestTwoDayRungIsSummaryOnly(t *testing.T) {
	if _, notifies := rungMessageKey[rungTwoDays]; notifies {
		t.Fatal("rungTwoDays has a customer message key, so clients would be notified 2 days out")
	}
	if _, summarised := rungSummaryKey[rungTwoDays]; !summarised {
		t.Fatal("rungTwoDays has no summary key, so admins would never see it")
	}

	// Every other rung on the ladder still reaches the customer.
	for _, rung := range []renewalRung{rungThreeDays, rungTomorrow, rungToday, rungOverdue} {
		if _, notifies := rungMessageKey[rung]; !notifies {
			t.Fatalf("rung %v lost its customer message key", rung)
		}
	}
}

// The summary reads as a countdown, so 2 days must sit between tomorrow and 3
// days rather than being appended wherever the map happened to order it.
func TestRenewalSummaryOrdersTwoDaysAfterTomorrow(t *testing.T) {
	tg := &Tgbot{}
	summary := tg.renewalSummary(map[renewalRung][]renewalNotice{
		rungOverdue:   {{email: "overdue@x"}},
		rungToday:     {{email: "today@x"}},
		rungTomorrow:  {{email: "tomorrow@x"}},
		rungTwoDays:   {{email: "twodays@x"}},
		rungThreeDays: {{email: "threedays@x"}},
	})

	want := []string{
		"tgbot.messages.renewSummaryOverdue",
		"tgbot.messages.renewSummaryToday",
		"tgbot.messages.renewSummaryTomorrow",
		"tgbot.messages.renewSummaryTwoDays",
		"tgbot.messages.renewSummaryThreeDays",
	}
	prev := -1
	for _, key := range want {
		next := strings.Index(summary, key)
		if next < 0 {
			t.Fatalf("summary is missing %q: %q", key, summary)
		}
		if next < prev {
			t.Fatalf("%q appears out of countdown order in %q", key, summary)
		}
		prev = next
	}
}

// An admin reading the summary needs to know who actually got their reminder,
// so a delivered, a refused and an unlinked customer must not look alike.
func TestSummaryClientListMarksDelivery(t *testing.T) {
	tests := []struct {
		name   string
		notice renewalNotice
		want   string
	}{
		{name: "delivered", notice: renewalNotice{email: "reached@x", outcome: deliveryDelivered}, want: "✅ reached@x"},
		{name: "refused", notice: renewalNotice{email: "blocked@x", outcome: deliveryFailed}, want: "❌ blocked@x"},
		{name: "no telegram account", notice: renewalNotice{email: "unlinked@x", outcome: deliveryUnlinked}, want: "➖ unlinked@x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := summaryClientList([]renewalNotice{tc.notice}); got != tc.want {
				t.Fatalf("summaryClientList = %q, want %q", got, tc.want)
			}
		})
	}
}

// Nobody is messaged 2 days out, so a tick or a cross there would report on a
// reminder that was never sent.
func TestSummaryClientListLeavesUnsentRungUnmarked(t *testing.T) {
	got := summaryClientList([]renewalNotice{{email: "twodays@x", outcome: deliveryNone}})
	if got != "twodays@x" {
		t.Fatalf("summaryClientList = %q, want the bare email", got)
	}
}

// Marks are per client, so the list must stay sorted by email rather than by
// whichever send happened to finish first.
func TestSummaryClientListSortsByEmail(t *testing.T) {
	got := summaryClientList([]renewalNotice{
		{email: "carol@x", outcome: deliveryDelivered},
		{email: "alice@x", outcome: deliveryFailed},
		{email: "bob@x", outcome: deliveryUnlinked},
	})
	want := "❌ alice@x, ➖ bob@x, ✅ carol@x"
	if got != want {
		t.Fatalf("summaryClientList = %q, want %q", got, want)
	}
}
