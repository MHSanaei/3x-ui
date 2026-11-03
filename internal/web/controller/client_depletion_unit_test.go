package controller

import (
	"math"
	"testing"
)

func TestDeriveClientDepletion(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		up, down, total, expiry, lastOnline, now, at int64
		reason                                       string
	}{
		{"neither", 20, 30, 100, 1100, 800, 1000, 0, ""},
		{"traffic boundary", 60, 40, 100, 1100, 800, 1000, 800, "traffic"},
		{"expiry", 20, 30, 100, 900, 800, 1000, 900, "expiry"},
		{"both last online earlier", 60, 40, 100, 900, 800, 1000, 800, "both"},
		{"both expiry earlier", 60, 40, 100, 700, 800, 1000, 700, "both"},
		{"both equal", 60, 40, 100, 800, 800, 1000, 800, "both"},
		{"traffic unknown time", 60, 40, 100, 1100, 0, 1000, 0, "traffic"},
		{"both unknown time", 60, 40, 100, 900, 0, 1000, 900, "both"},
		{"both negative legacy last online", 60, 40, 100, 900, -1, 1000, 900, "both"},
		{"unlimited quota", 100, 200, 0, 0, 800, 1000, 0, ""},
		{"negative quota", 100, 200, -1, 0, 800, 1000, 0, ""},
		{"no absolute expiry", 20, 30, 100, -100, 800, 1000, 0, ""},
		{"expiry exactly now", 20, 30, 100, 1000, 800, 1000, 1000, "expiry"},
		{"expiry one millisecond ago", 20, 30, 100, 999, 800, 1000, 999, "expiry"},
		{"expiry one millisecond ahead", 20, 30, 100, 1001, 800, 1000, 0, ""},
		{"overflowing usage", math.MaxInt64, 1, math.MaxInt64, 0, 800, 1000, 800, "traffic"},
		{"both counters near maximum", math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64, 0, 800, 1000, 800, "traffic"},
		{"legacy zeroes", 0, 0, 0, 0, 0, 1000, 0, ""},
		{"signed legacy usage", math.MinInt64, -1, 100, 0, 800, 1000, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 {
				at, reason := deriveClientDepletion(tc.up, tc.down, tc.total, tc.expiry, tc.lastOnline, tc.now)
				if at != tc.at || reason != tc.reason {
					t.Fatalf("got (%d,%q), want (%d,%q)", at, reason, tc.at, tc.reason)
				}
			}
		})
	}
}
