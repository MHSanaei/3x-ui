// Package global provides global variables and interfaces for accessing web and subscription servers.
package global

import (
	"context"
	"sync"
	_ "unsafe"

	"github.com/robfig/cron/v3"
)

var (
	webServer WebServer
	subServer SubServer

	restartHookMu sync.RWMutex
	restartHook   func()
)

// WebServer interface defines methods for accessing the web server instance.
type WebServer interface {
	GetCron() *cron.Cron     // Get the cron scheduler
	GetCtx() context.Context // Get the server context
	GetWSHub() any           // Get the WebSocket hub (using any to avoid circular dependency)
}

// SubServer interface defines methods for accessing the subscription server instance.
type SubServer interface {
	GetCtx() context.Context // Get the server context
}

// SetWebServer sets the global web server instance.
func SetWebServer(s WebServer) {
	webServer = s
}

// GetWebServer returns the global web server instance.
func GetWebServer() WebServer {
	return webServer
}

// SetSubServer sets the global subscription server instance.
func SetSubServer(s SubServer) {
	subServer = s
}

// GetSubServer returns the global subscription server instance.
func GetSubServer() SubServer {
	return subServer
}

// SetRestartHook registers a callback that triggers an in-process panel
// restart. main.go sets this up to push SIGHUP into its own signal channel
// so the restart path works on Windows (where p.Signal(SIGHUP) is unsupported).
func SetRestartHook(fn func()) {
	restartHookMu.Lock()
	defer restartHookMu.Unlock()
	restartHook = fn
}

// TriggerRestart fires the registered restart hook. Returns false if none is set.
func TriggerRestart() bool {
	restartHookMu.RLock()
	fn := restartHook
	restartHookMu.RUnlock()
	if fn == nil {
		return false
	}
	fn()
	return true
}
