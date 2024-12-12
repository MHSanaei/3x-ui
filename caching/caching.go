package caching

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
)

type Cache struct {
	memoryCache  *cache.Cache

	ctx    context.Context
	cancel context.CancelFunc
}

func NewCache() *Cache {
	ctx, cancel := context.WithCancel(context.Background())
	return &Cache{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Cache) Init() (err error) {
	defer func() {
		if err != nil {
			s.Flush()
		}
	}()

	s.memoryCache = cache.New(10*time.Minute, 10*time.Minute)

	return nil
}

func (s *Cache) Flush() error {
	if s.memoryCache != nil {
		s.memoryCache.Flush()
	}
	s.cancel()

	return nil
}

func (s *Cache) GetCtx() context.Context {
	return s.ctx
}

func (s *Cache) Memory() *cache.Cache {
	return s.memoryCache
}