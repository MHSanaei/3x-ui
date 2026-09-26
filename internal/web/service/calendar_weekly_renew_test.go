package service

import (
	"testing"
	"time"
)

func TestNextWeeklyRenewal_RepeatedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ expiry, want string }{
		{"2026-10-25T00:00:00-04:00", "2026-11-01T00:00:00-04:00"},
		{"2026-11-01T00:00:00-04:00", "2026-11-08T00:00:00-05:00"},
		{"2026-11-01T00:00:00-05:00", "2026-11-08T00:00:00-05:00"},
	} {
		from, err := time.Parse(time.RFC3339, tt.expiry)
		if err != nil {
			t.Fatal(err)
		}
		got := nextWeeklyRenewal(from, 7, loc).Format(time.RFC3339)
		if got != tt.want {
			t.Fatalf("next Sunday after %s = %s, want %s", tt.expiry, got, tt.want)
		}
	}
}

func TestNextWeeklyRenewal_SkippedDates(t *testing.T) {
	for _, tt := range []struct {
		zone, from, want string
		weekday          int
	}{
		{"America/Havana", "2026-03-01T00:00:00-05:00", "2026-03-08T01:00:00-04:00", 7},
		{"Pacific/Apia", "2011-12-23T00:00:00-10:00", "2012-01-06T00:00:00+14:00", 5},
	} {
		t.Run(tt.zone, func(t *testing.T) {
			loc, err := time.LoadLocation(tt.zone)
			if err != nil {
				t.Fatal(err)
			}
			from, err := time.Parse(time.RFC3339, tt.from)
			if err != nil {
				t.Fatal(err)
			}
			got := nextWeeklyRenewal(from, tt.weekday, loc).Format(time.RFC3339)
			if got != tt.want {
				t.Fatalf("next weekly cutoff = %s, want %s", got, tt.want)
			}
		})
	}
}
