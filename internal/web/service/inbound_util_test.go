package service

import "testing"

func TestNormalizeSubSortIndex(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 1},
		{1, 1},
		{7, 7},
		{-1, -1},
		{-10, -10},
	}
	for _, tc := range cases {
		if got := normalizeSubSortIndex(tc.in); got != tc.want {
			t.Fatalf("normalizeSubSortIndex(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
