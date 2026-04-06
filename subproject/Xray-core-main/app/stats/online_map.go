package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	localhostIPv4 = "127.0.0.1"
	localhostIPv6 = "[::1]"
)

type ipEntry struct {
	refCount int
	lastSeen time.Time
}

// OnlineMap is a refcount-based implementation of stats.OnlineMap.
// IPs are tracked by reference counting: AddIP increments, RemoveIP decrements.
// An IP is removed from the map when its reference count reaches zero.
type OnlineMap struct {
	entries map[string]*ipEntry
	access  sync.Mutex
	count   atomic.Int64
}

// NewOnlineMap creates a new OnlineMap instance.
func NewOnlineMap() *OnlineMap {
	return &OnlineMap{
		entries: make(map[string]*ipEntry),
	}
}

// AddIP implements stats.OnlineMap.
func (om *OnlineMap) AddIP(ip string) {
	if ip == localhostIPv4 || ip == localhostIPv6 {
		return
	}

	om.access.Lock()
	defer om.access.Unlock()

	if e, ok := om.entries[ip]; ok {
		e.refCount++
		e.lastSeen = time.Now()
	} else {
		om.entries[ip] = &ipEntry{
			refCount: 1,
			lastSeen: time.Now(),
		}
		om.count.Add(1)
	}
}

// RemoveIP implements stats.OnlineMap.
func (om *OnlineMap) RemoveIP(ip string) {
	om.access.Lock()
	defer om.access.Unlock()

	e, ok := om.entries[ip]
	if !ok {
		return
	}
	e.refCount--
	if e.refCount <= 0 {
		delete(om.entries, ip)
		om.count.Add(-1)
	}
}

// Count implements stats.OnlineMap.
func (om *OnlineMap) Count() int {
	return int(om.count.Load())
}

// List implements stats.OnlineMap.
func (om *OnlineMap) List() []string {
	om.access.Lock()
	defer om.access.Unlock()

	keys := make([]string, 0, len(om.entries))
	for ip := range om.entries {
		keys = append(keys, ip)
	}
	return keys
}

// IPTimeMap implements stats.OnlineMap.
func (om *OnlineMap) IPTimeMap() map[string]time.Time {
	om.access.Lock()
	defer om.access.Unlock()

	result := make(map[string]time.Time, len(om.entries))
	for ip, e := range om.entries {
		result[ip] = e.lastSeen
	}
	return result
}
