package global

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"sync"
	"time"
)

type HashEntry struct {
	Hash      string
	Value     string
	Timestamp time.Time
}

type HashStorage struct {
	sync.RWMutex
	Data       map[string]HashEntry
	Expiration time.Duration
}

func NewHashStorage(expiration time.Duration) *HashStorage {
	return &HashStorage{
		Data:       make(map[string]HashEntry),
		Expiration: expiration,
	}
}

func (h *HashStorage) SaveHash(query string) string {
	h.Lock()
	defer h.Unlock()

	md5Hash := md5.Sum([]byte(query))
	md5HashString := hex.EncodeToString(md5Hash[:])

	entry := HashEntry{
		Hash:      md5HashString,
		Value:     query,
		Timestamp: time.Now(),
	}

	h.Data[md5HashString] = entry

	return md5HashString
}

func (h *HashStorage) GetValue(hash string) (string, bool) {
	h.RLock()
	defer h.RUnlock()

	entry, exists := h.Data[hash]

	return entry.Value, exists
}

func (h *HashStorage) IsMD5(hash string) bool {
	match, _ := regexp.MatchString("^[a-f0-9]{32}$", hash)
	return match
}

func (h *HashStorage) RemoveExpiredHashes() {
	h.Lock()
	defer h.Unlock()

	now := time.Now()

	for hash, entry := range h.Data {
		if now.Sub(entry.Timestamp) > h.Expiration {
			delete(h.Data, hash)
		}
	}
}

func (h *HashStorage) Reset() {
	h.Lock()
	defer h.Unlock()

	h.Data = make(map[string]HashEntry)
}
