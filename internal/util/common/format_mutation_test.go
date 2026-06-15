package common

import "testing"

// TestFormatTraffic_UnitBoundaries pins the exact switch point in the loop
// condition `size >= 1024`: a unit must roll over at exactly 1024 (not 1023,
// not 1025), and a value one byte short must stay in the lower unit. This kills
// CONDITIONALS_BOUNDARY (>= -> >) and ARITHMETIC_BASE on the 1024 comparison.
func TestFormatTraffic_UnitBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		// Just below the first boundary: must NOT roll over to KB.
		{"one_below_kb", 1023, "1023.00B"},
		// Exactly at the boundary: must roll over to KB.
		{"exactly_kb", 1024, "1.00KB"},
		// Just above: stays in KB.
		{"one_above_kb", 1025, "1.00KB"},
		// Just below the MB boundary: stays in KB (proves division divisor 1024).
		{"one_below_mb", 1024*1024 - 1, "1024.00KB"},
		// Exactly at the MB boundary: rolls over to MB.
		{"exactly_mb", 1024 * 1024, "1.00MB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatTraffic(c.bytes); got != c.want {
				t.Fatalf("FormatTraffic(%d) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}

// TestFormatTraffic_ClampsAtPB pins the upper bound guard
// `unitIndex < len(units)-1`: huge values must clamp at PB instead of indexing
// past the units slice. A mutated bound (< -> <= via CONDITIONALS_BOUNDARY, or
// len(units)-1 -> len(units)+1 via INVERT_NEGATIVES/ARITHMETIC_BASE) would run
// one extra iteration and panic with index-out-of-range, so the assertion that
// these return a normal "PB" string kills those mutants.
func TestFormatTraffic_ClampsAtPB(t *testing.T) {
	const pb = int64(1024 * 1024 * 1024 * 1024 * 1024)
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		// Stays at PB even though size is still >= 1024 at the PB level.
		{"1024_pb", 1024 * pb, "1024.00PB"},
		// Max int64 must not overflow the units slice.
		{"max_int64", 9223372036854775807, "8192.00PB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatTraffic(c.bytes); got != c.want {
				t.Fatalf("FormatTraffic(%d) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}
