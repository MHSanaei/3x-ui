package job

import (
	"testing"
	"time"
)

func TestMonthlyResetDue(t *testing.T) {
	cases := []struct {
		name     string
		resetDay int
		now      time.Time
		want     bool
	}{
		{"legacy default on first", 0, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true},
		{"configured day", 15, time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), true},
		{"before configured day", 15, time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC), false},
		{"month end", 31, time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC), true},
		{"short month fallback", 31, time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), true},
		{"leap year fallback", 31, time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC), true},
		{"not before short month end", 31, time.Date(2028, time.February, 28, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := monthlyResetDue(tc.resetDay, tc.now); got != tc.want {
				t.Fatalf("monthlyResetDue(%d, %s) = %v, want %v", tc.resetDay, tc.now.Format(time.DateOnly), got, tc.want)
			}
		})
	}
}
