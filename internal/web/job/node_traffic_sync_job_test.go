package job

import (
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAtomicBool_DefaultIsFalse(t *testing.T) {
	var a atomicBool
	if a.takeAndReset() {
		t.Fatal("default atomicBool should report false")
	}
}

func TestAtomicBool_SetThenTakeReturnsTrueOnce(t *testing.T) {
	var a atomicBool
	a.set()
	if !a.takeAndReset() {
		t.Fatal("takeAndReset after set should return true")
	}
	if a.takeAndReset() {
		t.Fatal("second takeAndReset should return false (state was reset)")
	}
}

func TestAtomicBool_SetIsIdempotent(t *testing.T) {
	var a atomicBool
	a.set()
	a.set()
	a.set()
	if !a.takeAndReset() {
		t.Fatal("repeated set should still leave the flag true")
	}
	if a.takeAndReset() {
		t.Fatal("flag should be cleared after the first take")
	}
}

func TestAtomicBool_ConcurrentSettersExactlyOneTakeWins(t *testing.T) {
	var a atomicBool
	const setters = 100
	const readers = 20

	var wg sync.WaitGroup
	for range setters {
		wg.Go(func() {
			a.set()
		})
	}
	wg.Wait()

	trueCount := 0
	var rwg sync.WaitGroup
	var mu sync.Mutex
	for range readers {
		rwg.Go(func() {
			if a.takeAndReset() {
				mu.Lock()
				trueCount++
				mu.Unlock()
			}
		})
	}
	rwg.Wait()

	if trueCount != 1 {
		t.Fatalf("expected exactly one reader to observe true, got %d", trueCount)
	}
}

// Regression (#6283): a node onboarded in selected mode with an empty tag
// list empties its snapshot via FilterNodeSnapshot before the merge sees it,
// so that sync adopts nothing and must not stamp InboundsAdoptedAt.
func TestSyncCanAdoptInbounds(t *testing.T) {
	cases := []struct {
		name     string
		node     *model.Node
		aliases  []string
		expected bool
	}{
		{"all mode always adopts", &model.Node{InboundSyncMode: "all"}, nil, true},
		{
			"selected with tags adopts",
			&model.Node{InboundSyncMode: "selected", InboundTags: []string{"in-443-tcp"}},
			nil,
			true,
		},
		{
			"selected empty with adopted alias adopts",
			&model.Node{InboundSyncMode: "selected"},
			[]string{"in-443-tcp"},
			true,
		},
		{
			// The reported bug: registering in selected mode and choosing
			// tags afterwards stamped adoption while adopting nothing.
			"selected empty with no aliases adopts nothing",
			&model.Node{InboundSyncMode: "selected"},
			nil,
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := syncCanAdoptInbounds(c.node, c.aliases); got != c.expected {
				t.Fatalf("syncCanAdoptInbounds(%+v, %v) = %v, want %v", c.node, c.aliases, got, c.expected)
			}
		})
	}
}
