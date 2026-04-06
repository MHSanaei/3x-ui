package antireplay

import (
	"sync"
	"time"
)

// ReplayFilter checks for replay attacks.
type ReplayFilter[T comparable] struct {
	lock      sync.Mutex
	poolA     map[T]struct{}
	poolB     map[T]struct{}
	interval  time.Duration
	lastClean time.Time
}

// NewMapFilter create a new filter with specifying the expiration time interval in seconds.
func NewMapFilter[T comparable](interval int64) *ReplayFilter[T] {
	filter := &ReplayFilter[T]{
		poolA:     make(map[T]struct{}),
		poolB:     make(map[T]struct{}),
		interval:  time.Duration(interval) * time.Second,
		lastClean: time.Now(),
	}
	return filter
}

// Check determines if there are duplicate records.
func (filter *ReplayFilter[T]) Check(sum T) bool {
	filter.lock.Lock()
	defer filter.lock.Unlock()

	now := time.Now()
	if now.Sub(filter.lastClean) >= filter.interval {
		filter.poolB = filter.poolA
		filter.poolA = make(map[T]struct{})
		filter.lastClean = now
	}

	_, existsA := filter.poolA[sum]
	_, existsB := filter.poolB[sum]
	if !existsA && !existsB {
		filter.poolA[sum] = struct{}{}
	}
	return !(existsA || existsB)
}
