package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const inboundMetaCacheTTL = 10 * time.Second

// InboundMeta stores inbound metadata and settings with "clients" stripped so
// link generation and tunnel checks skip reading multi-megabyte client arrays.
type InboundMeta struct {
	Id                    int
	UserId                int
	Up                    int64
	Down                  int64
	Total                 int64
	Remark                string
	SubSortIndex          int
	ExcludeFromSub        bool
	Enable                bool
	ExpiryTime            int64
	TrafficReset          string
	TrafficResetDay       int
	LastTrafficResetTime  int64
	PreserveClientTraffic *bool
	Listen                string
	Port                  int
	Protocol              model.Protocol
	StreamSettings        string
	Tag                   string
	Sniffing              string
	NodeID                *int
	ShareAddrStrategy     string
	ShareAddr             string
	DisableFlow           bool
	OriginNodeGuid        string
	SettingsNoClients     string
}

// ToInboundShell returns a fresh *model.Inbound with client-stripped Settings.
func (m *InboundMeta) ToInboundShell() *model.Inbound {
	if m == nil {
		return nil
	}
	var nodeID *int
	if m.NodeID != nil {
		v := *m.NodeID
		nodeID = &v
	}
	var preserve *bool
	if m.PreserveClientTraffic != nil {
		v := *m.PreserveClientTraffic
		preserve = &v
	}
	return &model.Inbound{
		Id:                    m.Id,
		UserId:                m.UserId,
		Up:                    m.Up,
		Down:                  m.Down,
		Total:                 m.Total,
		Remark:                m.Remark,
		SubSortIndex:          m.SubSortIndex,
		ExcludeFromSub:        m.ExcludeFromSub,
		Enable:                m.Enable,
		ExpiryTime:            m.ExpiryTime,
		TrafficReset:          m.TrafficReset,
		TrafficResetDay:       m.TrafficResetDay,
		LastTrafficResetTime:  m.LastTrafficResetTime,
		PreserveClientTraffic: preserve,
		Listen:                m.Listen,
		Port:                  m.Port,
		Protocol:              m.Protocol,
		Settings:              m.SettingsNoClients,
		StreamSettings:        m.StreamSettings,
		Tag:                   m.Tag,
		Sniffing:              m.Sniffing,
		NodeID:                nodeID,
		ShareAddrStrategy:     m.ShareAddrStrategy,
		ShareAddr:             m.ShareAddr,
		DisableFlow:           m.DisableFlow,
		OriginNodeGuid:        m.OriginNodeGuid,
	}
}

type inboundMetaCacheEntry struct {
	meta      *InboundMeta
	expiresAt time.Time
}

type inboundMetaCacheStore struct {
	mu         sync.RWMutex
	db         *gorm.DB
	generation uint64
	entries    map[int]inboundMetaCacheEntry
	group      singleflight.Group
	hookMu     sync.Mutex
}

var inboundMetaCache = &inboundMetaCacheStore{
	entries: make(map[int]inboundMetaCacheEntry),
}

// InvalidateInboundMeta evicts a single inbound from the metadata cache.
func InvalidateInboundMeta(id int) {
	inboundMetaCache.mu.Lock()
	inboundMetaCache.generation++
	delete(inboundMetaCache.entries, id)
	inboundMetaCache.mu.Unlock()
}

// InvalidateAllInboundMeta clears all cached inbound metadata entries.
func InvalidateAllInboundMeta() {
	inboundMetaCache.mu.Lock()
	inboundMetaCache.generation++
	clear(inboundMetaCache.entries)
	inboundMetaCache.mu.Unlock()
}

func (c *inboundMetaCacheStore) invalidateIfDB(db *gorm.DB) {
	c.mu.Lock()
	if c.db == db {
		c.generation++
		clear(c.entries)
	}
	c.mu.Unlock()
}

func isInboundMetaIgnorableCol(col string) bool {
	switch col {
	case "settings", "last_traffic_reset_time", "up", "down", "all_time":
		return true
	default:
		return false
	}
}

func (c *inboundMetaCacheStore) ensureWriteHooks(db *gorm.DB) {
	if db == nil {
		return
	}
	const cbCreate = "service:inbound_meta_create"
	c.hookMu.Lock()
	defer c.hookMu.Unlock()
	if db.Callback().Create().Get(cbCreate) != nil {
		return
	}
	onMutate := func(tx *gorm.DB) {
		if tx != nil && tx.Statement != nil && tx.Statement.Table == "inbounds" {
			c.invalidateIfDB(db)
		}
	}
	onUpdate := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Table != "inbounds" {
			return
		}
		if m, ok := tx.Statement.Dest.(map[string]any); ok {
			ignorable := true
			for k := range m {
				if !isInboundMetaIgnorableCol(k) {
					ignorable = false
					break
				}
			}
			for _, sel := range tx.Statement.Selects {
				if !isInboundMetaIgnorableCol(sel) {
					ignorable = false
					break
				}
			}
			if ignorable {
				return
			}
		}
		c.invalidateIfDB(db)
	}
	onRaw := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		sqlStr := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sqlStr, "inbounds") &&
			(strings.Contains(sqlStr, "update ") || strings.Contains(sqlStr, "delete ") ||
				strings.Contains(sqlStr, "insert ") || strings.Contains(sqlStr, "replace ")) {
			c.invalidateIfDB(db)
		}
	}
	_ = db.Callback().Create().After("gorm:create").Register(cbCreate, onMutate)
	_ = db.Callback().Update().After("gorm:update").Register("service:inbound_meta_update", onUpdate)
	_ = db.Callback().Delete().After("gorm:delete").Register("service:inbound_meta_delete", onMutate)
	_ = db.Callback().Raw().After("gorm:raw").Register("service:inbound_meta_raw", onRaw)
}

func stripClientsFromSettings(settings string) string {
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

func inboundToMeta(ib *model.Inbound) *InboundMeta {
	if ib == nil {
		return nil
	}
	var nodeID *int
	if ib.NodeID != nil {
		v := *ib.NodeID
		nodeID = &v
	}
	var preserve *bool
	if ib.PreserveClientTraffic != nil {
		v := *ib.PreserveClientTraffic
		preserve = &v
	}
	return &InboundMeta{
		Id:                    ib.Id,
		UserId:                ib.UserId,
		Up:                    ib.Up,
		Down:                  ib.Down,
		Total:                 ib.Total,
		Remark:                ib.Remark,
		SubSortIndex:          ib.SubSortIndex,
		ExcludeFromSub:        ib.ExcludeFromSub,
		Enable:                ib.Enable,
		ExpiryTime:            ib.ExpiryTime,
		TrafficReset:          ib.TrafficReset,
		TrafficResetDay:       ib.TrafficResetDay,
		LastTrafficResetTime:  ib.LastTrafficResetTime,
		PreserveClientTraffic: preserve,
		Listen:                ib.Listen,
		Port:                  ib.Port,
		Protocol:              ib.Protocol,
		StreamSettings:        ib.StreamSettings,
		Tag:                   ib.Tag,
		Sniffing:              ib.Sniffing,
		NodeID:                nodeID,
		ShareAddrStrategy:     ib.ShareAddrStrategy,
		ShareAddr:             ib.ShareAddr,
		DisableFlow:           ib.DisableFlow,
		OriginNodeGuid:        ib.OriginNodeGuid,
		SettingsNoClients:     stripClientsFromSettings(ib.Settings),
	}
}

func (c *inboundMetaCacheStore) lookupBatch(db *gorm.DB, ids []int, now time.Time) (map[int]*InboundMeta, []int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	hits := make(map[int]*InboundMeta, len(ids))
	var misses []int
	if c.db != db {
		misses = append([]int(nil), ids...)
		return hits, misses
	}
	for _, id := range ids {
		entry, ok := c.entries[id]
		if ok && now.Before(entry.expiresAt) {
			hits[id] = entry.meta
		} else {
			misses = append(misses, id)
		}
	}
	return hits, misses
}

// GetInboundMetaBatch returns cached InboundMeta for all requested inbound IDs,
// loading misses in one batched query coalesced via singleflight.
func (s *InboundService) GetInboundMetaBatch(ids []int) (map[int]*InboundMeta, map[int]error) {
	out := make(map[int]*InboundMeta, len(ids))
	errs := make(map[int]error)
	if len(ids) == 0 {
		return out, errs
	}
	db := database.GetDB()
	if db == nil {
		for _, id := range ids {
			errs[id] = gorm.ErrInvalidDB
		}
		return out, errs
	}
	inboundMetaCache.ensureWriteHooks(db)

	unique := uniqueInts(ids)
	hits, misses := inboundMetaCache.lookupBatch(db, unique, time.Now())
	for id, m := range hits {
		out[id] = m
	}
	if len(misses) == 0 {
		return out, errs
	}

	sortedMisses := append([]int(nil), misses...)
	sort.Ints(sortedMisses)
	var keyParts strings.Builder
	fmt.Fprintf(&keyParts, "%p:", db)
	for i, id := range sortedMisses {
		if i > 0 {
			keyParts.WriteByte(',')
		}
		keyParts.WriteString(strconv.Itoa(id))
	}

	v, err, _ := inboundMetaCache.group.Do(keyParts.String(), func() (any, error) {
		inboundMetaCache.mu.RLock()
		gen := inboundMetaCache.generation
		prevDB := inboundMetaCache.db
		inboundMetaCache.mu.RUnlock()

		loaded := make(map[int]*InboundMeta, len(sortedMisses))
		for _, batch := range chunkInts(sortedMisses, sqlInChunk) {
			var rows []model.Inbound
			if err := db.Model(&model.Inbound{}).Where("id IN ?", batch).Find(&rows).Error; err != nil {
				return nil, err
			}
			for i := range rows {
				loaded[rows[i].Id] = inboundToMeta(&rows[i])
			}
		}

		expires := time.Now().Add(inboundMetaCacheTTL)
		inboundMetaCache.mu.Lock()
		if inboundMetaCache.db != db {
			inboundMetaCache.db = db
			inboundMetaCache.generation++
			clear(inboundMetaCache.entries)
			for id, m := range loaded {
				inboundMetaCache.entries[id] = inboundMetaCacheEntry{meta: m, expiresAt: expires}
			}
		} else if gen == inboundMetaCache.generation && prevDB == db {
			for id, m := range loaded {
				inboundMetaCache.entries[id] = inboundMetaCacheEntry{meta: m, expiresAt: expires}
			}
		}
		inboundMetaCache.mu.Unlock()
		return loaded, nil
	})
	if err != nil {
		for _, id := range misses {
			errs[id] = err
		}
		return out, errs
	}
	loaded := v.(map[int]*InboundMeta)
	for _, id := range misses {
		if m, ok := loaded[id]; ok {
			out[id] = m
		} else {
			errs[id] = fmt.Errorf("%w: inbound %d", gorm.ErrRecordNotFound, id)
		}
	}
	return out, errs
}
