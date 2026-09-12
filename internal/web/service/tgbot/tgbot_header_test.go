package tgbot

import "testing"

// The three cases the roster already distinguishes: a config that never
// lapses, one whose window has not opened, and one with a real deadline.
func TestExpiryStateOf(t *testing.T) {
	tests := []struct {
		name       string
		expiryTime int64
		want       expiryState
	}{
		{name: "unlimited", expiryTime: 0, want: expiryUnlimited},
		{name: "not started", expiryTime: -1, want: expiryNotStarted},
		{name: "not started, long window", expiryTime: -2592000000, want: expiryNotStarted},
		{name: "real deadline", expiryTime: 1788000000000, want: expiryDated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expiryStateOf(tc.expiryTime); got != tc.want {
				t.Fatalf("expiryStateOf(%d) = %d, want %d", tc.expiryTime, got, tc.want)
			}
		})
	}
}

// A customer reading "0 B left" when they have no quota at all would think
// they were cut off, so unlimited has to be reported as unlimited.
func TestRemainingBytes(t *testing.T) {
	const gb = int64(1073741824)

	tests := []struct {
		name    string
		used    int64
		total   int64
		want    int64
		limited bool
	}{
		{name: "unlimited", used: 5 * gb, total: 0, want: 0, limited: false},
		{name: "negative total is unlimited", used: 5 * gb, total: -1, want: 0, limited: false},
		{name: "half spent", used: 50 * gb, total: 100 * gb, want: 50 * gb, limited: true},
		{name: "nothing spent", used: 0, total: 100 * gb, want: 100 * gb, limited: true},
		{name: "exactly spent", used: 100 * gb, total: 100 * gb, want: 0, limited: true},
		{name: "overspent never goes negative", used: 150 * gb, total: 100 * gb, want: 0, limited: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, limited := remainingBytes(tc.used, tc.total)
			if limited != tc.limited {
				t.Fatalf("remainingBytes(%d, %d) limited = %v, want %v", tc.used, tc.total, limited, tc.limited)
			}
			if got != tc.want {
				t.Fatalf("remainingBytes(%d, %d) = %d, want %d", tc.used, tc.total, got, tc.want)
			}
		})
	}
}
