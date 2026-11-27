package service

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

type skipInboundSettingsCacheInvalidationKey struct{}

const (
	settingsIdxCreateCB = "service:inbound_settings_idx_create"
	settingsIdxUpdateCB = "service:inbound_settings_idx_update"
	settingsIdxDeleteCB = "service:inbound_settings_idx_delete"
	settingsIdxRawCB    = "service:inbound_settings_idx_raw"
)

type inboundSettingsIdxCacheStore struct {
	mu      sync.RWMutex
	db      *gorm.DB
	entries map[int]*fastSettingsIndex
	hookMu  sync.Mutex
}

var inboundSettingsIdxCache = &inboundSettingsIdxCacheStore{
	entries: make(map[int]*fastSettingsIndex),
}

// InvalidateInboundSettingsCache evicts a single inbound's cached settings index.
func InvalidateInboundSettingsCache(id int) {
	inboundSettingsIdxCache.mu.Lock()
	delete(inboundSettingsIdxCache.entries, id)
	inboundSettingsIdxCache.mu.Unlock()
}

// InvalidateAllInboundSettingsCache clears all cached settings indexes.
func InvalidateAllInboundSettingsCache() {
	inboundSettingsIdxCache.mu.Lock()
	clear(inboundSettingsIdxCache.entries)
	inboundSettingsIdxCache.mu.Unlock()
}

func (c *inboundSettingsIdxCacheStore) ensureWriteHooks(db *gorm.DB) {
	if db == nil {
		return
	}
	c.hookMu.Lock()
	defer c.hookMu.Unlock()
	if db.Callback().Create().Get(settingsIdxCreateCB) != nil {
		return
	}
	shouldSkip := func(tx *gorm.DB) bool {
		return tx != nil && tx.Statement != nil && tx.Statement.Context != nil &&
			tx.Statement.Context.Value(skipInboundSettingsCacheInvalidationKey{}) != nil
	}
	onMutate := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Table != "inbounds" || shouldSkip(tx) {
			return
		}
		c.invalidateIfDB(db)
	}
	onUpdate := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Table != "inbounds" || shouldSkip(tx) {
			return
		}
		if m, ok := tx.Statement.Dest.(map[string]any); ok {
			if _, hasSettings := m["settings"]; !hasSettings && !slices.Contains(tx.Statement.Selects, "settings") {
				return
			}
		} else if len(tx.Statement.Selects) > 0 && !slices.Contains(tx.Statement.Selects, "settings") {
			return
		}
		c.invalidateIfDB(db)
	}
	onRaw := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || shouldSkip(tx) {
			return
		}
		sqlStr := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sqlStr, "inbounds") &&
			(strings.Contains(sqlStr, "update ") || strings.Contains(sqlStr, "delete ") ||
				strings.Contains(sqlStr, "insert ") || strings.Contains(sqlStr, "replace ")) {
			c.invalidateIfDB(db)
		}
	}
	_ = db.Callback().Create().After("gorm:create").Register(settingsIdxCreateCB, onMutate)
	_ = db.Callback().Update().After("gorm:update").Register(settingsIdxUpdateCB, onUpdate)
	_ = db.Callback().Delete().After("gorm:delete").Register(settingsIdxDeleteCB, onMutate)
	_ = db.Callback().Raw().After("gorm:raw").Register(settingsIdxRawCB, onRaw)
}

func (c *inboundSettingsIdxCacheStore) invalidateIfDB(db *gorm.DB) {
	c.mu.Lock()
	if c.db == db {
		clear(c.entries)
	}
	c.mu.Unlock()
}

func (c *inboundSettingsIdxCacheStore) getBatch(db *gorm.DB, ids []int) (map[int]*fastSettingsIndex, bool) {
	if db == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.db != db {
		return nil, false
	}
	hits := make(map[int]*fastSettingsIndex, len(ids))
	for _, id := range ids {
		idx, ok := c.entries[id]
		if !ok || idx == nil {
			return nil, false
		}
		hits[id] = idx
	}
	return hits, true
}

func (c *inboundSettingsIdxCacheStore) getOne(db *gorm.DB, id int) *fastSettingsIndex {
	if db == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.db != db {
		return nil
	}
	return c.entries[id]
}

func applySerializedTxSettingsCacheOps(db *gorm.DB, ops map[int]*fastSettingsIndex) {
	if db == nil || len(ops) == 0 {
		return
	}
	inboundSettingsIdxCache.ensureWriteHooks(db)
	inboundSettingsIdxCache.mu.Lock()
	if inboundSettingsIdxCache.db != db {
		clear(inboundSettingsIdxCache.entries)
		inboundSettingsIdxCache.db = db
	}
	for id, idx := range ops {
		if idx == nil {
			delete(inboundSettingsIdxCache.entries, id)
		} else {
			inboundSettingsIdxCache.entries[id] = idx
		}
	}
	inboundSettingsIdxCache.mu.Unlock()
}

func withSkipSettingsCacheInvalidation(tx *gorm.DB) *gorm.DB {
	if tx == nil || tx.Statement == nil {
		return tx
	}
	ctx := tx.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return tx.WithContext(context.WithValue(ctx, skipInboundSettingsCacheInvalidationKey{}, struct{}{}))
}

func fetchInboundsByIDs(tx *gorm.DB, ids []int) (map[int]*model.Inbound, error) {
	out := make(map[int]*model.Inbound, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var rows []*model.Inbound
		if err := tx.Model(&model.Inbound{}).Where("id IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.Id] = r
		}
	}
	return out, nil
}

// loadInboundsForClientWrite returns inbound rows and cached fastSettingsIndex entries.
// On SQLite inside runSerializedTx, cache hits omit the settings column from SELECT.
func loadInboundsForClientWrite(tx *gorm.DB, ids []int) (map[int]*model.Inbound, map[int]*fastSettingsIndex, error) {
	out := make(map[int]*model.Inbound, len(ids))
	idxMap := make(map[int]*fastSettingsIndex, len(ids))
	if len(ids) == 0 {
		return out, idxMap, nil
	}
	if tx == nil {
		tx = database.GetDB()
	}
	unique := uniqueInts(ids)
	root := database.GetDB()
	state := serializedTxOf(tx)
	if root != nil {
		inboundSettingsIdxCache.ensureWriteHooks(root)
	}

	isSQLite := tx != nil && tx.Dialector != nil && tx.Name() == database.DialectSQLite
	if isSQLite && state != nil && root != nil {
		if cached, allHit := inboundSettingsIdxCache.getBatch(root, unique); allHit {
			for _, batch := range chunkInts(unique, sqlInChunk) {
				var rows []*model.Inbound
				if err := tx.Model(&model.Inbound{}).Omit("settings").Where("id IN ?", batch).Find(&rows).Error; err != nil {
					return nil, nil, err
				}
				for _, r := range rows {
					if idx := cached[r.Id]; idx != nil {
						r.Settings = idx.raw
						idxMap[r.Id] = idx
					}
					out[r.Id] = r
				}
			}
			return out, idxMap, nil
		}
	}

	inboundsByID, err := fetchInboundsByIDs(tx, unique)
	if err != nil {
		return nil, nil, err
	}
	for id, ib := range inboundsByID {
		out[id] = ib
		if ib == nil {
			continue
		}
		if root != nil {
			if cached := inboundSettingsIdxCache.getOne(root, id); cached != nil && cached.raw == ib.Settings {
				idxMap[id] = cached
				continue
			}
		}
		if parsed, ok := parseFastSettingsIndex(ib.Settings); ok {
			idxMap[id] = parsed
			if state != nil {
				state.stageSettingsCache(id, parsed)
			}
		}
	}
	return out, idxMap, nil
}

type removedSettingsClient struct {
	email  string
	enable bool
}

// removeClientsFast splices out entries whose exact email is in wanted without
// full JSON unmarshal/marshal; callers fall back to JSON decoding if ok is false.
func removeClientsFast(inboundId int, raw string, wanted map[string]struct{}) (string, []removedSettingsClient, bool) {
	var idx *fastSettingsIndex
	if root := database.GetDB(); root != nil {
		if cached := inboundSettingsIdxCache.getOne(root, inboundId); cached != nil && cached.raw == raw {
			idx = cached
		}
	}
	if idx == nil {
		var parsedOK bool
		idx, parsedOK = parseFastSettingsIndex(raw)
		if !parsedOK {
			return "", nil, false
		}
	}
	if !idx.canSplice() {
		return "", nil, false
	}
	dropPos := make(map[int]struct{}, len(wanted))
	var removed []removedSettingsClient
	for email := range wanted {
		for _, pos := range idx.findExactPositions(email) {
			if _, dup := dropPos[pos]; dup {
				continue
			}
			dropPos[pos] = struct{}{}
			sp := idx.spans[pos]
			removed = append(removed, removedSettingsClient{email: sp.email, enable: sp.enable})
		}
	}
	if len(dropPos) == 0 {
		return raw, nil, true
	}
	newRaw, _ := idx.spliceRemove(dropPos)
	return newRaw, removed, true
}
