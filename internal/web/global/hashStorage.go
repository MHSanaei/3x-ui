package global

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"sync"
	"time"
)

// HashEntry represents a stored hash entry with its value and timestamp.
type HashEntry struct {
	Hash      string    // MD5 hash string
	Value     string    // Original value
	Timestamp time.Time // Time when the hash was created
}

// HashStorage provides thread-safe storage for hash-value pairs with expiration.
type HashStorage struct {
	sync.RWMutex
	Data       map[string]HashEntry // Map of hash to entry
	Expiration time.Duration        // Expiration duration for entries
}

// NewHashStorage creates a new HashStorage instance with the specified expiration duration.
func NewHashStorage(expiration time.Duration) *HashStorage {
	return &HashStorage{
		Data:       make(map[string]HashEntry),
		Expiration: expiration,
	}
}

// SaveHash generates an MD5 hash for the given query string and stores it with a timestamp.
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

// GetValue retrieves the original value for the given hash, returning true if found.
func (h *HashStorage) GetValue(hash string) (string, bool) {
	h.RLock()
	defer h.RUnlock()

	entry, exists := h.Data[hash]

	return entry.Value, exists
}

// IsMD5 checks if the given string is a valid 32-character MD5 hash.
func (h *HashStorage) IsMD5(hash string) bool {
	match, _ := regexp.MatchString("^[a-f0-9]{32}$", hash)
	return match
}

// RemoveExpiredHashes removes all hash entries that have exceeded the expiration duration.
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

// Reset clears all stored hash entries.
func (h *HashStorage) Reset() {
	h.Lock()
	defer h.Unlock()

	h.Data = make(map[string]HashEntry)
}
