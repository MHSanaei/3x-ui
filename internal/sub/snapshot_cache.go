package sub

import (
	"container/list"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/sync/singleflight"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	// Subscription apps poll on multi-minute intervals, so a snapshot at most
	// this old is indistinguishable from a fresh read in practice.
	snapshotCacheTTL = 10 * time.Second
	// Bounds memory: one entry holds one subscriber's inbound rows.
	snapshotCacheCapacity = 256
)

// snapshotEntry holds one subscriber's inbound rows pre-marshaled to JSON, so
// hits decode fresh structs: callers mutate inbounds in place (fallback-master
// projection, external-proxy injection), and sharing live structs across
// requests would race.
type snapshotEntry struct {
	inboundsJSON []byte
	stats        map[string]xray.ClientTraffic
	element      *list.Element
	expiresAt    time.Time
}

// subSnapshotStore caches per-subId subscription reads with single-flight
// builds. Client polls re-run identical queries; the store collapses that to
// one read per subId per TTL window.
type subSnapshotStore struct {
	ttl      time.Duration
	capacity int
	now      func() time.Time
	group    singleflight.Group

	mu      sync.Mutex
	entries map[string]*snapshotEntry
	lru     *list.List
}

func newSubSnapshotStore(ttl time.Duration, capacity int) *subSnapshotStore {
	return &subSnapshotStore{
		ttl:      ttl,
		capacity: capacity,
		now:      time.Now,
		entries:  make(map[string]*snapshotEntry),
		lru:      list.New(),
	}
}

// inboundsFor returns freshly decoded inbound structs and a stats-map copy
// for subId, calling build on a miss. Concurrent callers for one subId share
// a single build; every caller receives its own structs and map.
func (c *subSnapshotStore) inboundsFor(subId string, build func() ([]*model.Inbound, map[string]xray.ClientTraffic, error)) ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
	entry, err := c.entryFor(subId, build)
	if err != nil {
		return nil, nil, err
	}
	var inbounds []*model.Inbound
	if err := json.Unmarshal(entry.inboundsJSON, &inbounds); err != nil {
		return nil, nil, err
	}
	return inbounds, cloneStats(entry.stats), nil
}

func (c *subSnapshotStore) entryFor(subId string, build func() ([]*model.Inbound, map[string]xray.ClientTraffic, error)) (*snapshotEntry, error) {
	if entry, ok := c.lookup(subId); ok {
		return entry, nil
	}
	v, err, _ := c.group.Do(subId, func() (any, error) {
		if entry, ok := c.lookup(subId); ok {
			return entry, nil
		}
		inbounds, stats, err := build()
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(inbounds)
		if err != nil {
			return nil, err
		}
		return c.store(subId, raw, stats), nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*snapshotEntry), nil
}

func (c *subSnapshotStore) lookup(key string) (*snapshotEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(entry.expiresAt) {
		c.removeLocked(key)
		return nil, false
	}
	c.lru.MoveToFront(entry.element)
	return entry, true
}

func (c *subSnapshotStore) store(key string, inboundsJSON []byte, stats map[string]xray.ClientTraffic) *snapshotEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
	entry := &snapshotEntry{
		inboundsJSON: inboundsJSON,
		stats:        cloneStats(stats),
		expiresAt:    c.now().Add(c.ttl),
	}
	entry.element = c.lru.PushFront(key)
	c.entries[key] = entry
	if len(c.entries) > c.capacity {
		c.evictLocked()
	}
	return entry
}

func (c *subSnapshotStore) removeLocked(key string) {
	if entry, ok := c.entries[key]; ok {
		c.lru.Remove(entry.element)
		delete(c.entries, key)
	}
}

// evictLocked drops expired entries first, then least-recently-used ones down
// to half capacity, so a subscriber burst does not evict the whole cache.
func (c *subSnapshotStore) evictLocked() {
	now := c.now()
	for key := c.lru.Back(); key != nil; {
		prev := key.Prev()
		id := key.Value.(string)
		if entry, ok := c.entries[id]; ok && !now.Before(entry.expiresAt) {
			c.removeLocked(id)
		}
		key = prev
	}
	for len(c.entries) > c.capacity/2 && c.lru.Len() > 0 {
		c.removeLocked(c.lru.Back().Value.(string))
	}
}

// purgeAll drops every cached snapshot; the invalidator registry calls it
// after panel mutations so edits surface on the next fetch, not after the
// TTL. A build racing a purge may store a pre-mutation entry, bounded by TTL.
func (c *subSnapshotStore) purgeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*snapshotEntry)
	c.lru.Init()
}

func cloneStats(stats map[string]xray.ClientTraffic) map[string]xray.ClientTraffic {
	out := make(map[string]xray.ClientTraffic, len(stats))
	for email, row := range stats {
		out[email] = row
	}
	return out
}
