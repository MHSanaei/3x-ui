package service

import "sync"

var (
	subDataInvalidatorsMu  sync.Mutex
	subDataInvalidators    = map[int]func(){}
	nextSubDataInvalidator int
)

// RegisterSubDataInvalidator registers a callback run after any mutation that can change what a
// subscription serves; release unregisters it, so a stopped subscription server's cache goes.
func RegisterSubDataInvalidator(f func()) (release func()) {
	subDataInvalidatorsMu.Lock()
	defer subDataInvalidatorsMu.Unlock()
	id := nextSubDataInvalidator
	nextSubDataInvalidator++
	subDataInvalidators[id] = f
	return func() {
		subDataInvalidatorsMu.Lock()
		defer subDataInvalidatorsMu.Unlock()
		delete(subDataInvalidators, id)
	}
}

// InvalidateSubData drops every registered subscription snapshot. Purging is
// clearing a small map, so callers never need to compute affected subIds.
func InvalidateSubData() {
	subDataInvalidatorsMu.Lock()
	defer subDataInvalidatorsMu.Unlock()
	for _, invalidate := range subDataInvalidators {
		invalidate()
	}
}
