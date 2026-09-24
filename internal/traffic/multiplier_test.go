package traffic

import "testing"

const mb = int64(1024 * 1024)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want int64
	}{
		{0, DefaultMultiplier},  // legacy NULL / zero row heals to 1x
		{-1, DefaultMultiplier}, // negative corrupt heals to 1x
		{-10000, DefaultMultiplier},
		{10001, DefaultMultiplier}, // beyond max heals to 1x
		{1, DefaultMultiplier},     // sub-1x heals to 1x
		{99, DefaultMultiplier},    // just below min heals to 1x
		{100, 100},                 // min is kept
		{150, 150},
		{10000, 10000}, // max is kept
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		in   int64
		want bool
	}{
		{0, false}, // 0 is rejected
		{-1, false},
		{1, false},  // sub-1x is rejected
		{99, false}, // just below min is rejected
		{100, true},
		{10000, true},
		{10001, false},
	}
	for _, c := range cases {
		if got := Validate(c.in); got != c.want {
			t.Errorf("Validate(%d) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestApply(t *testing.T) {
	cases := []struct {
		name     string
		delta, m int64
		want     int64
	}{
		{"zero delta bills zero", 0, 150, 0},
		{"negative delta (counter reset) bills zero", -50 * mb, 200, 0},
		{"1x is identity", 100 * mb, 100, 100 * mb},
		{"sub-1x 50 normalizes to 1x", 100 * mb, 50, 100 * mb},
		{"1.5x: 100MB bills as 150MB", 100 * mb, 150, 150 * mb},
		{"2x: 100MB bills as 200MB", 100 * mb, 200, 200 * mb},
		{"floor truncation, no fractional bytes", 101, 150, 151}, // 151.5 -> 151
		{"1 byte at 1.5x floors, no fractional bytes", 1, 150, 1},
		{"invalid multiplier 0 normalizes to 1x", 100, 0, 100},
		{"invalid multiplier 99 normalizes to 1x", 100, 99, 100},
		{"invalid multiplier 10001 normalizes to 1x", 100, 10001, 100},
		{"max multiplier 100x", 100, 10000, 10000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Apply(c.delta, c.m); got != c.want {
				t.Errorf("Apply(%d, %d) = %d, want %d", c.delta, c.m, got, c.want)
			}
		})
	}
}
