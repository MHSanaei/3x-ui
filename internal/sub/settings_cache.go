package sub

import (
	"strconv"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/sync/singleflight"
)

// Subscription apps poll on multi-minute intervals, so inbound settings at most this old read the
// same as fresh ones; panel edits purge them at once.
const settingsCacheTTL = 10 * time.Second

// inboundSettingsCache keeps each inbound's settings without its clients array for subscription
// bodies, so every subscriber of a large inbound shares one small copy instead of the whole blob.
type inboundSettingsCache struct {
	ttl   time.Duration
	now   func() time.Time
	group singleflight.Group

	mu         sync.Mutex
	generation uint64
	entries    map[int]settingsEntry
}

type settingsEntry struct {
	settings  string
	expiresAt time.Time
}

func newInboundSettingsCache(ttl time.Duration) *inboundSettingsCache {
	return &inboundSettingsCache{ttl: ttl, now: time.Now, entries: map[int]settingsEntry{}}
}

// settingsFor returns the inbound's client-less settings, loading them at most once per TTL window
// even when concurrent requests miss together.
func (c *inboundSettingsCache) settingsFor(inboundId int, load func() (string, error)) (string, error) {
	if settings, ok := c.lookup(inboundId); ok {
		return settings, nil
	}
	v, err, _ := c.group.Do(strconv.Itoa(inboundId), func() (any, error) {
		if settings, ok := c.lookup(inboundId); ok {
			return settings, nil
		}
		c.mu.Lock()
		generation := c.generation
		c.mu.Unlock()
		raw, err := load()
		if err != nil {
			return "", err
		}
		settings := withoutClients(raw)
		c.mu.Lock()
		defer c.mu.Unlock()
		// A purge during the load means the row may have changed under it; serve it, never keep it.
		if generation == c.generation {
			c.entries[inboundId] = settingsEntry{settings: settings, expiresAt: c.now().Add(c.ttl)}
		}
		return settings, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *inboundSettingsCache) lookup(inboundId int) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[inboundId]
	if !ok || !c.now().Before(entry.expiresAt) {
		return "", false
	}
	return entry.settings, true
}

// purgeAll drops every entry after a panel mutation, so an edit reaches the next fetch.
func (c *inboundSettingsCache) purgeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	clear(c.entries)
}

// withoutClients drops the clients array, nearly all of a large inbound's settings, which
// subscription bodies resolve from the clients table instead; malformed settings stay as they are.
func withoutClients(settings string) string {
	var shallow map[string]json.RawMessage
	if err := json.Unmarshal([]byte(settings), &shallow); err != nil {
		return settings
	}
	if _, ok := shallow["clients"]; !ok {
		return settings
	}
	delete(shallow, "clients")
	out, err := json.Marshal(shallow)
	if err != nil {
		return settings
	}
	return string(out)
}
