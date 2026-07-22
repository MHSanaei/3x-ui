package service

import "testing"

func TestNormalizeTrafficResetDay(t *testing.T) {
	tests := map[int]int{
		0:  1,
		1:  1,
		15: 15,
		31: 31,
		32: 31,
	}
	for input, want := range tests {
		if got := normalizeTrafficResetDay(input); got != want {
			t.Errorf("normalizeTrafficResetDay(%d) = %d, want %d", input, got, want)
		}
	}
}
