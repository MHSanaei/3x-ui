package utils

import (
	"sync"
)

// TypedSyncMap is a wrapper of sync.Map that provides type-safe for keys and values.
// No need to use type assertions every time, so you can have more time to enjoy other things like GochiUsa
// If sync.Map methods returned nil, it will return the zero value of the type V.
type TypedSyncMap[K, V any] struct {
	syncMap *sync.Map
}

// NewTypedSyncMap creates a new TypedSyncMap
// K is key type, V is value type
// It is recommended to use pointer types for V because sync.Map might return nil
// If sync.Map methods really returned nil, it will return the zero value of the type V
func NewTypedSyncMap[K any, V any]() *TypedSyncMap[K, V] {
	return &TypedSyncMap[K, V]{
		syncMap: &sync.Map{},
	}
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *TypedSyncMap[K, V]) Clear() {
	m.syncMap.Clear()
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// The old value must be of a comparable type.
//
// If there is no current value for key in the map, CompareAndDelete
// returns false (even if the old value is the nil interface value).
func (m *TypedSyncMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.syncMap.CompareAndDelete(key, old)
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
// The old value must be of a comparable type.
func (m *TypedSyncMap[K, V]) CompareAndSwap(key K, old V, new V) (swapped bool) {
	return m.syncMap.CompareAndSwap(key, old, new)
}

// Delete deletes the value for a key.
func (m *TypedSyncMap[K, V]) Delete(key K) {
	m.syncMap.Delete(key)
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *TypedSyncMap[K, V]) Load(key K) (value V, ok bool) {
	anyValue, ok := m.syncMap.Load(key)
	// anyValue might be nil
	if anyValue != nil {
		value = anyValue.(V)
	}
	return value, ok
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *TypedSyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	anyValue, loaded := m.syncMap.LoadAndDelete(key)
	if anyValue != nil {
		value = anyValue.(V)
	}
	return value, loaded
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *TypedSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	anyActual, loaded := m.syncMap.LoadOrStore(key, value)
	if anyActual != nil {
		actual = anyActual.(V)
	}
	return actual, loaded
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *TypedSyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.syncMap.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

// Store sets the value for a key.
func (m *TypedSyncMap[K, V]) Store(key K, value V) {
	m.syncMap.Store(key, value)
}

// Swap swaps the value for a key and returns the previous value if any. The loaded result reports whether the key was present.
func (m *TypedSyncMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	anyPrevious, loaded := m.syncMap.Swap(key, value)
	if anyPrevious != nil {
		previous = anyPrevious.(V)
	}
	return previous, loaded
}