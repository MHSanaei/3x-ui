package job

import (
	"sync"
	"testing"
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
