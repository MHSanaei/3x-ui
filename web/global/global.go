package global

import (
	"context"
	_ "unsafe"

	"github.com/robfig/cron/v3"
	"github.com/patrickmn/go-cache"
)

var (
	webServer WebServer
	subServer SubServer
	caching Cache
)

type WebServer interface {
	GetCron() *cron.Cron
	GetCtx() context.Context
}

type SubServer interface {
	GetCtx() context.Context
}

type Cache interface {
	Memory() *cache.Cache
	GetCtx() context.Context
}

func SetWebServer(s WebServer) {
	webServer = s
}

func GetWebServer() WebServer {
	return webServer
}

func SetSubServer(s SubServer) {
	subServer = s
}

func GetSubServer() SubServer {
	return subServer
}

func SetCache(c Cache) {
	caching = c
}

func GetCache() Cache {
	return caching
}
