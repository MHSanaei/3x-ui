package service

import (
	"testing"
	"time"
)

func TestNextCalendarRenewal_ClampsToShortMonths(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name string
		from time.Time
		day  int
		want time.Time
	}{
		{
			name: "31st in a 28-day February",
			from: time.Date(2026, time.January, 31, 0, 0, 0, 0, utc),
			day:  31,
			want: time.Date(2026, time.February, 28, 0, 0, 0, 0, utc),
		},
		{
			name: "31st returns to the 31st after the short month",
			from: time.Date(2026, time.February, 28, 0, 0, 0, 0, utc),
			day:  31,
			want: time.Date(2026, time.March, 31, 0, 0, 0, 0, utc),
		},
		{
			name: "29th in a leap February",
			from: time.Date(2028, time.January, 29, 0, 0, 0, 0, utc),
			day:  29,
			want: time.Date(2028, time.February, 29, 0, 0, 0, 0, utc),
		},
		{
			name: "31st in a 30-day month",
			from: time.Date(2026, time.March, 31, 0, 0, 0, 0, utc),
			day:  31,
			want: time.Date(2026, time.April, 30, 0, 0, 0, 0, utc),
		},
		{
			name: "December rolls into January",
			from: time.Date(2026, time.December, 5, 0, 0, 0, 0, utc),
			day:  5,
			want: time.Date(2027, time.January, 5, 0, 0, 0, 0, utc),
		},
		{
			name: "later in the same month renews this month",
			from: time.Date(2026, time.June, 3, 12, 0, 0, 0, utc),
			day:  20,
			want: time.Date(2026, time.June, 20, 0, 0, 0, 0, utc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextCalendarRenewal(tc.from, tc.day, utc)
			if !got.Equal(tc.want) {
				t.Fatalf("next renewal = %s, want %s", got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

// The renewal instant is midnight in the panel's zone, not in UTC: an operator
// billing on the 1st expects the period to turn over at their local midnight.
func TestNextCalendarRenewal_UsesPanelZone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Skip("zone database unavailable")
	}
	from := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	got := nextCalendarRenewal(from, 1, loc)

	if got.Location() != loc {
		t.Fatalf("renewal computed in %s, want the panel zone", got.Location())
	}
	y, m, d := got.Date()
	if y != 2026 || m != time.July || d != 1 {
		t.Fatalf("renewal date = %04d-%02d-%02d, want 2026-07-01", y, m, d)
	}
	if h, mi, s := got.Clock(); h != 0 || mi != 0 || s != 0 {
		t.Fatalf("renewal at %02d:%02d:%02d, want local midnight", h, mi, s)
	}
}

// Crossing a DST boundary must still land on local midnight rather than
// drifting an hour, which a fixed 24h*N step cannot promise.
func TestNextCalendarRenewal_SurvivesDaylightSaving(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("zone database unavailable")
	}
	// Berlin moves to summer time on 29 March 2026.
	from := time.Date(2026, time.March, 10, 0, 0, 0, 0, loc)
	got := nextCalendarRenewal(from, 10, loc)

	if h, mi, _ := got.Clock(); h != 0 || mi != 0 {
		t.Fatalf("renewal at %02d:%02d local, want midnight across the DST change", h, mi)
	}
	if got.Day() != 10 || got.Month() != time.April {
		t.Fatalf("renewal = %s, want 10 April", got.Format(time.RFC3339))
	}
}

func TestNextCalendarRenewal_AlwaysMovesForward(t *testing.T) {
	utc := time.UTC
	from := time.Date(2026, time.May, 20, 0, 0, 0, 0, utc)
	// Same day: the boundary has already passed today, so the next one is a
	// month away rather than the instant we started from.
	got := nextCalendarRenewal(from, 20, utc)
	if !got.After(from) {
		t.Fatalf("next renewal %s is not after %s", got, from)
	}
	if got.Month() != time.June {
		t.Fatalf("next renewal = %s, want June", got.Format(time.RFC3339))
	}
}
