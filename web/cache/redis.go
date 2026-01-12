// Package cache provides Redis caching functionality for the 3x-ui web panel.
// It supports both embedded Redis (miniredis) and external Redis server.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/redis/go-redis/v9"
)

var (
	client      *redis.Client
	miniRedis   *miniredis.Miniredis
	ctx         = context.Background()
	isEmbedded  = true
)

// InitRedis initializes Redis client. If redisAddr is empty, starts embedded Redis.
// If redisAddr is provided, connects to external Redis server.
func InitRedis(redisAddr string) error {
	if redisAddr == "" {
		// Use embedded Redis
		mr, err := miniredis.Run()
		if err != nil {
			return fmt.Errorf("failed to start embedded Redis: %w", err)
		}
		miniRedis = mr
		client = redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})
		isEmbedded = true
		logger.Info("Embedded Redis started on", mr.Addr())
	} else {
		// Use external Redis
		client = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: "", // Can be extended to support password
			DB:       0,
		})
		isEmbedded = false
		
		// Test connection
		_, err := client.Ping(ctx).Result()
		if err != nil {
			return fmt.Errorf("failed to connect to Redis at %s: %w", redisAddr, err)
		}
		logger.Info("Connected to external Redis at", redisAddr)
	}
	
	return nil
}

// GetClient returns the Redis client instance.
func GetClient() *redis.Client {
	return client
}

// IsEmbedded returns true if using embedded Redis.
func IsEmbedded() bool {
	return isEmbedded
}

// Close closes the Redis connection and stops embedded Redis if running.
func Close() error {
	if client != nil {
		if err := client.Close(); err != nil {
			return err
		}
	}
	if miniRedis != nil {
		miniRedis.Close()
	}
	return nil
}

// Set stores a value in Redis with expiration.
func Set(key string, value interface{}, expiration time.Duration) error {
	if client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value from Redis.
func Get(key string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("Redis client not initialized")
	}
	result, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist - this is expected, not an error
		return "", fmt.Errorf("redis: nil")
	}
	return result, err
}

// Delete removes a key from Redis.
func Delete(key string) error {
	if client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return client.Del(ctx, key).Err()
}

// DeletePattern removes all keys matching a pattern.
func DeletePattern(pattern string) error {
	if client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()
	keys := make([]string, 0)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	
	if len(keys) > 0 {
		return client.Del(ctx, keys...).Err()
	}
	return nil
}

// Exists checks if a key exists in Redis.
func Exists(key string) (bool, error) {
	if client == nil {
		return false, fmt.Errorf("Redis client not initialized")
	}
	count, err := client.Exists(ctx, key).Result()
	return count > 0, err
}
