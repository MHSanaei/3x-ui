package tgbot

import (
	"testing"
	"time"
)

// Mirrors reset_exp_c's arithmetic: a negative expiry marks "N days from first use",
// so a lapsed client restarts rather than being extended from a date already past.
func TestExtendedExpiry(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	const dayMs = int64(24 * 60 * 60000)

	tests := []struct {
		name    string
		current int64
		days    int64
		want    int64
	}{
		{
			name:    "future expiry is pushed out",
			current: now.AddDate(0, 0, 2).UnixMilli(),
			days:    30,
			want:    now.AddDate(0, 0, 2).UnixMilli() + 30*dayMs,
		},
		{
			name:    "lapsed client restarts from first use",
			current: now.AddDate(0, 0, -5).UnixMilli(),
			days:    30,
			want:    -30 * dayMs,
		},
		{
			name:    "unlimited client is given a window from first use",
			current: 0,
			days:    30,
			want:    -30 * dayMs,
		},
		{
			name:    "an unstarted window grows",
			current: -7 * dayMs,
			days:    30,
			want:    -37 * dayMs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extendedExpiry(tc.current, tc.days, now); got != tc.want {
				t.Fatalf("extendedExpiry(%d, %d) = %d, want %d", tc.current, tc.days, got, tc.want)
			}
		})
	}
}

// A bulk action must name exactly who it will touch before it touches them,
// and must never quietly half-apply to a roster larger than the cap.
func TestBulkTargets(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clients := rosterFixture(now)

	t.Run("extend takes everyone on the ladder", func(t *testing.T) {
		targets, over := bulkTargets(clients, bulkExtend, now)
		if over {
			t.Fatal("six clients tripped the cap")
		}
		assertEmails(t, targets, "soon@x", "overdue@x")
	})

	t.Run("disable takes only the overdue that are still on", func(t *testing.T) {
		targets, _ := bulkTargets(clients, bulkDisable, now)
		assertEmails(t, targets, "overdue@x")
	})

	t.Run("an unknown action selects nobody", func(t *testing.T) {
		targets, _ := bulkTargets(clients, "wipe_everything", now)
		if len(targets) != 0 {
			t.Fatalf("unknown action selected %v", emailsOf(targets))
		}
	})

	t.Run("too many matches refuses rather than half-applying", func(t *testing.T) {
		crowd := make([]bulkClient, 0, bulkLimit+1)
		for range bulkLimit + 1 {
			crowd = append(crowd, clients[1])
		}
		targets, over := bulkTargets(crowd, bulkExtend, now)
		if !over {
			t.Fatalf("cap of %d not tripped by %d matches", bulkLimit, len(crowd))
		}
		if len(targets) != 0 {
			t.Fatal("a refused bulk action still returned targets")
		}
	})
}
