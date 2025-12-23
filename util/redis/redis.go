package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

var (
	client     interface{} // Will be *redis.Client when package is available
	ctx        = context.Background()
	enabled    = false
	mu         sync.RWMutex
	fallbackMu sync.RWMutex
)

// In-memory fallback storage
var (
	fallbackStore = make(map[string]fallbackEntry)
	fallbackSets  = make(map[string]map[string]bool)
	fallbackHash  = make(map[string]map[string]string)
)

type fallbackEntry struct {
	value      interface{}
	expiration time.Time
}

// Init initializes Redis client with graceful fallback
func Init(addr, password string, db int) error {
	// Try to initialize Redis if package is available
	// For now, use in-memory fallback
	enabled = false
	logger.Info("Using in-memory fallback for Redis (Redis package not available)")
	return nil
}

// IsEnabled returns whether Redis is enabled
func IsEnabled() bool {
	return enabled
}

// Set stores a key-value pair with expiration (in-memory fallback)
func Set(key string, value interface{}, expiration time.Duration) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	entry := fallbackEntry{
		value:      value,
		expiration: time.Now().Add(expiration),
	}
	fallbackStore[key] = entry

	// Auto-cleanup expired entries
	if expiration > 0 {
		go func(k string, exp time.Duration) {
			time.Sleep(exp)
			fallbackMu.Lock()
			defer fallbackMu.Unlock()
			if entry, ok := fallbackStore[k]; ok && time.Now().After(entry.expiration) {
				delete(fallbackStore, k)
			}
		}(key, expiration)
	}

	return nil
}

// Get retrieves a value by key (in-memory fallback)
func Get(key string) (string, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	entry, ok := fallbackStore[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}

	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		return "", fmt.Errorf("key expired")
	}

	return fmt.Sprintf("%v", entry.value), nil
}

// Del deletes a key (in-memory fallback)
func Del(key string) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	delete(fallbackStore, key)
	return nil
}

// Exists checks if key exists (in-memory fallback)
func Exists(key string) (bool, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	entry, ok := fallbackStore[key]
	if !ok {
		return false, nil
	}

	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		return false, nil
	}

	return true, nil
}

// Incr increments a key (in-memory fallback)
func Incr(key string) (int64, error) {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	entry, ok := fallbackStore[key]
	var count int64 = 0
	if ok {
		if val, ok := entry.value.(int64); ok {
			count = val
		} else if val, ok := entry.value.(int); ok {
			count = int64(val)
		} else if val, ok := entry.value.(string); ok {
			fmt.Sscanf(val, "%d", &count)
		}
	}

	count++
	fallbackStore[key] = fallbackEntry{
		value:      count,
		expiration: entry.expiration,
	}

	return count, nil
}

// Expire sets expiration on a key (in-memory fallback)
func Expire(key string, expiration time.Duration) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	entry, ok := fallbackStore[key]
	if !ok {
		return fmt.Errorf("key not found")
	}

	entry.expiration = time.Now().Add(expiration)
	fallbackStore[key] = entry
	return nil
}

// HSet sets a field in a hash (in-memory fallback)
func HSet(key, field string, value interface{}) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	if fallbackHash[key] == nil {
		fallbackHash[key] = make(map[string]string)
	}
	fallbackHash[key][field] = fmt.Sprintf("%v", value)
	return nil
}

// HGet gets a field from a hash (in-memory fallback)
func HGet(key, field string) (string, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	if hash, ok := fallbackHash[key]; ok {
		if val, ok := hash[field]; ok {
			return val, nil
		}
	}
	return "", fmt.Errorf("field not found")
}

// HGetAll gets all fields from a hash (in-memory fallback)
func HGetAll(key string) (map[string]string, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	if hash, ok := fallbackHash[key]; ok {
		result := make(map[string]string, len(hash))
		for k, v := range hash {
			result[k] = v
		}
		return result, nil
	}
	return make(map[string]string), nil
}

// HDel deletes a field from a hash (in-memory fallback)
func HDel(key, field string) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	if hash, ok := fallbackHash[key]; ok {
		delete(hash, field)
	}
	return nil
}

// SAdd adds member to set (in-memory fallback)
func SAdd(key string, members ...interface{}) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	if fallbackSets[key] == nil {
		fallbackSets[key] = make(map[string]bool)
	}

	for _, member := range members {
		fallbackSets[key][fmt.Sprintf("%v", member)] = true
	}
	return nil
}

// SIsMember checks if member is in set (in-memory fallback)
func SIsMember(key string, member interface{}) (bool, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	if set, ok := fallbackSets[key]; ok {
		return set[fmt.Sprintf("%v", member)], nil
	}
	return false, nil
}

// SMembers gets all members of a set (in-memory fallback)
func SMembers(key string) ([]string, error) {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	if set, ok := fallbackSets[key]; ok {
		members := make([]string, 0, len(set))
		for member := range set {
			members = append(members, member)
		}
		return members, nil
	}
	return []string{}, nil
}

// SRem removes member from set (in-memory fallback)
func SRem(key string, members ...interface{}) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	if set, ok := fallbackSets[key]; ok {
		for _, member := range members {
			delete(set, fmt.Sprintf("%v", member))
		}
	}
	return nil
}

// ZAdd adds member to sorted set with score (in-memory fallback - simplified)
func ZAdd(key string, score float64, member string) error {
	// Simplified implementation - store as hash with score as value
	return HSet(key+":zset", member, fmt.Sprintf("%f", score))
}

// ZRange gets members from sorted set by range (in-memory fallback - simplified)
func ZRange(key string, start, stop int64) ([]string, error) {
	// Simplified implementation
	hash, err := HGetAll(key + ":zset")
	if err != nil {
		return []string{}, err
	}

	members := make([]string, 0, len(hash))
	for member := range hash {
		members = append(members, member)
	}

	// Simple range (no sorting by score)
	if start < 0 {
		start = 0
	}
	if stop >= int64(len(members)) {
		stop = int64(len(members)) - 1
	}
	if start > stop {
		return []string{}, nil
	}

	return members[start : stop+1], nil
}

// ZRem removes member from sorted set (in-memory fallback)
func ZRem(key string, members ...interface{}) error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	hashKey := key + ":zset"
	if hash, ok := fallbackHash[hashKey]; ok {
		for _, member := range members {
			delete(hash, fmt.Sprintf("%v", member))
		}
	}
	return nil
}

// Close closes Redis connection
func Close() error {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	fallbackStore = make(map[string]fallbackEntry)
	fallbackSets = make(map[string]map[string]bool)
	fallbackHash = make(map[string]map[string]string)
	return nil
}

// CleanExpired removes expired entries (call periodically)
func CleanExpired() {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()

	now := time.Now()
	for key, entry := range fallbackStore {
		if !entry.expiration.IsZero() && now.After(entry.expiration) {
			delete(fallbackStore, key)
		}
	}
}
