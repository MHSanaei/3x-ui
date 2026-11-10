package service

import (
	"slices"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// settingCacheTTL bounds how stale a cached settings row may be. Settings are
// written by this process (invalidated immediately) but the x-ui CLI and the
// panel share one database file, so a short TTL keeps a CLI write visible
// without each process having to poll.
const settingCacheTTL = 2 * time.Second

// settingCacheExcludedKeys stay out of the snapshot: the Xray template blob is
// large and every migration that reads it wants the committed value anyway.
var settingCacheExcludedKeys = []string{"xrayTemplateConfig"}

// settingSnapshot memoizes the settings table so a subscription request reading
// eight keys costs one query instead of eight. It is invalidated by every write
// this process performs and refreshes when the database is reopened.
type settingSnapshotCache struct {
	mu     sync.Mutex
	db     *gorm.DB
	values map[string]string
	loaded time.Time
}

var settingsSnapshot settingSnapshotCache

// get returns the cached value for key. ok is false when the key is excluded
// from the snapshot or the table has no row for it; callers then read the row
// directly so a missing setting keeps its old not-found semantics.
func (c *settingSnapshotCache) get(key string) (string, bool, error) {
	if slices.Contains(settingCacheExcludedKeys, key) {
		return "", false, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	db := database.GetDB()
	if db != nil && (c.values == nil || c.db != db || time.Since(c.loaded) >= settingCacheTTL) {
		rows := make([]model.Setting, 0, 128)
		if err := db.Select("key", "value").
			Not("key IN ?", settingCacheExcludedKeys).
			Find(&rows).Error; err != nil {
			return "", false, err
		}
		values := make(map[string]string, len(rows))
		for _, row := range rows {
			values[row.Key] = row.Value
		}
		c.db = db
		c.values = values
		c.loaded = time.Now()
	}
	value, ok := c.values[key]
	return value, ok, nil
}

// store keeps a value this process just wrote visible without a reload.
func (c *settingSnapshotCache) store(key, value string) {
	if slices.Contains(settingCacheExcludedKeys, key) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil || c.db != database.GetDB() {
		return
	}
	c.values[key] = value
}

func (c *settingSnapshotCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = nil
	c.values = nil
	c.loaded = time.Time{}
}

// InvalidateSettingCache drops the memoized settings table. Tests and callers
// that write settings rows directly through the database have to call it.
func InvalidateSettingCache() {
	settingsSnapshot.invalidate()
}
