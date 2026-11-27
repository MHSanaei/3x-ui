package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const settingCacheTTL = 5 * time.Second

type settingCacheEntry struct {
	value     string
	found     bool
	expiresAt time.Time
}

type settingCacheStore struct {
	mu         sync.RWMutex
	db         *gorm.DB
	generation uint64
	entries    map[string]settingCacheEntry
	group      singleflight.Group
	hookMu     sync.Mutex
}

var settingsCache = &settingCacheStore{
	entries: make(map[string]settingCacheEntry),
}

// InvalidateSettingsCache clears all cached settings rows.
func InvalidateSettingsCache() {
	settingsCache.invalidateAll()
}

// InvalidateSettingKey evicts a single setting key from the cache.
func InvalidateSettingKey(key string) {
	settingsCache.invalidateKey(key)
}

func (c *settingCacheStore) invalidateAll() {
	c.mu.Lock()
	c.generation++
	clear(c.entries)
	c.mu.Unlock()
}

func (c *settingCacheStore) invalidateKey(key string) {
	c.mu.Lock()
	c.generation++
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *settingCacheStore) ensureWriteHooks(db *gorm.DB) {
	if db == nil {
		return
	}
	const cbCreate = "service:setting_cache_create"
	c.hookMu.Lock()
	defer c.hookMu.Unlock()
	if db.Callback().Create().Get(cbCreate) != nil {
		return
	}
	cb := func(tx *gorm.DB) {
		if tx != nil && tx.Statement != nil && tx.Statement.Table == "settings" {
			c.invalidateAll()
		}
	}
	onRaw := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		sqlStr := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sqlStr, "settings") &&
			(strings.Contains(sqlStr, "update ") || strings.Contains(sqlStr, "delete ") ||
				strings.Contains(sqlStr, "insert ") || strings.Contains(sqlStr, "replace ")) {
			c.invalidateAll()
		}
	}
	_ = db.Callback().Create().After("gorm:create").Register(cbCreate, cb)
	_ = db.Callback().Update().After("gorm:update").Register("service:setting_cache_update", cb)
	_ = db.Callback().Delete().After("gorm:delete").Register("service:setting_cache_delete", cb)
	_ = db.Callback().Raw().After("gorm:raw").Register("service:setting_cache_raw", onRaw)
}

func (c *settingCacheStore) lookup(db *gorm.DB, key string, now time.Time) (settingCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.db != db {
		return settingCacheEntry{}, false
	}
	entry, ok := c.entries[key]
	if !ok || !now.Before(entry.expiresAt) {
		return settingCacheEntry{}, false
	}
	return entry, true
}

func (c *settingCacheStore) get(db *gorm.DB, key string) (string, bool, error) {
	if db == nil {
		return "", false, gorm.ErrInvalidDB
	}
	c.ensureWriteHooks(db)
	if entry, ok := c.lookup(db, key, time.Now()); ok {
		return entry.value, entry.found, nil
	}
	sfKey := fmt.Sprintf("%p:%s", db, key)
	v, err, _ := c.group.Do(sfKey, func() (any, error) {
		if entry, ok := c.lookup(db, key, time.Now()); ok {
			return entry, nil
		}
		c.mu.RLock()
		gen := c.generation
		prevDB := c.db
		c.mu.RUnlock()

		var row model.Setting
		err := db.Model(model.Setting{}).Where("key = ?", key).First(&row).Error
		var entry settingCacheEntry
		if err == nil {
			entry = settingCacheEntry{
				value:     row.Value,
				found:     true,
				expiresAt: time.Now().Add(settingCacheTTL),
			}
		} else if database.IsNotFound(err) {
			entry = settingCacheEntry{
				found:     false,
				expiresAt: time.Now().Add(settingCacheTTL),
			}
		} else {
			return settingCacheEntry{}, err
		}

		c.mu.Lock()
		if c.db != db {
			c.db = db
			c.generation++
			clear(c.entries)
			c.entries[key] = entry
		} else if gen == c.generation && prevDB == db {
			c.entries[key] = entry
		}
		c.mu.Unlock()
		return entry, nil
	})
	if err != nil {
		return "", false, err
	}
	entry := v.(settingCacheEntry)
	return entry.value, entry.found, nil
}
