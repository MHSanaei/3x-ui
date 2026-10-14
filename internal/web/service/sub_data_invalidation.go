package service

import "sync"

var (
	subDataInvalidatorsMu sync.Mutex
	subDataInvalidators   []func()
)

// RegisterSubDataInvalidator registers a callback invoked after any mutation
// that can change which inbounds a subscription resolves to or how they are
// served. The subscription service registers its snapshot cache here.
func RegisterSubDataInvalidator(f func()) {
	subDataInvalidatorsMu.Lock()
	defer subDataInvalidatorsMu.Unlock()
	subDataInvalidators = append(subDataInvalidators, f)
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
