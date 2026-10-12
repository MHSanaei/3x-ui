package service

import (
	"math"
	"slices"
	"testing"
)

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

// A size near MaxInt used to overflow the capacity math and panic in make (CodeQL #119).
func TestChunkHugeSizeKeepsOneBatch(t *testing.T) {
	strs := []string{"a", "b", "c"}
	if got := chunkStrings(strs, math.MaxInt); len(got) != 1 || !slices.Equal(got[0], strs) {
		t.Fatalf("chunkStrings(MaxInt) = %v, want [%v]", got, strs)
	}
	ints := []int{1, 2, 3}
	if got := chunkInts(ints, math.MaxInt); len(got) != 1 || !slices.Equal(got[0], ints) {
		t.Fatalf("chunkInts(MaxInt) = %v, want [%v]", got, ints)
	}
}
