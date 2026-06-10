package common

import "testing"

func TestFormatTraffic(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0.00B"},
		{"under_one_kb", 512, "512.00B"},
		{"exactly_one_kb", 1024, "1.00KB"},
		{"one_and_a_half_kb", 1536, "1.50KB"},
		{"one_mb", 1024 * 1024, "1.00MB"},
		{"one_gb", 1024 * 1024 * 1024, "1.00GB"},
		{"one_tb", 1024 * 1024 * 1024 * 1024, "1.00TB"},
		{"one_pb", 1024 * 1024 * 1024 * 1024 * 1024, "1.00PB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FormatTraffic(c.bytes)
			if got != c.want {
				t.Fatalf("FormatTraffic(%d) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}
