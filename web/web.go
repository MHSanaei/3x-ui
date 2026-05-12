// Package web provides the main web server implementation for the 3x-ui panel,
// including HTTP/HTTPS serving, routing, templates, and background job scheduling.
package web

import (
	"context"
	"crypto/tls"
	"embed"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/web/controller"
	"github.com/mhsanaei/3x-ui/v3/web/job"
	"github.com/mhsanaei/3x-ui/v3/web/locale"
	"github.com/mhsanaei/3x-ui/v3/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/web/network"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

//go:embed translation/*
var i18nFS embed.FS

// distFS embeds the Vite-built frontend (web/dist/). Every user-facing
// HTML route is served straight out of this FS — the legacy Go
// templates and `web/assets/` tree are gone post-Phase 8.
//
// `all:` is required so files whose names start with `_` are NOT
// silently excluded by go:embed's default rules. Vite/rolldown emits
// `_plugin-vue_export-helper-<hash>.js` for the @vitejs/plugin-vue
// runtime; without `all:` the chunk would be missing from the binary
// at runtime → 404 → blank-page boot failure.
//
//go:embed all:dist
var distFS embed.FS

var startTime = time.Now()

// wrapDistFS adapts the embedded `dist/` directory so it can be mounted
// as the panel's `/assets/` static route. Vite emits its bundled JS/CSS
// under `dist/assets/`; serving the FS rooted at `dist/assets` makes
// `/assets/<hash>.js` URLs resolve directly.
type wrapDistFS struct {
	embed.FS
}

func (f *wrapDistFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open("dist/assets/" + name)
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFile{
		File: file,
	}, nil
}

type wrapAssetsFile struct {
	fs.File
}

func (f *wrapAssetsFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFileInfo{
		FileInfo: info,
	}, nil
}

type wrapAssetsFileInfo struct {
	fs.FileInfo
}

func (f *wrapAssetsFileInfo) ModTime() time.Time {
	return startTime
}

// EmbeddedDist returns the embedded Vite-built frontend filesystem.
// Controllers serve their HTML out of this FS via the dist-page handler
// installed in NewEngine().
func EmbeddedDist() embed.FS {
	return distFS
}

// Server represents the main web server for the 3x-ui panel with controllers, services, and scheduled jobs.
type Server struct {
	httpServer *http.Server
	listener   net.Listener

	index *controller.IndexController
	panel *controller.XUIController
	api   *controller.APIController
	ws    *controller.WebSocketController

	xrayService      service.XrayService
	settingService   service.SettingService
	tgbotService     service.Tgbot
	customGeoService *service.CustomGeoService

	wsHub *websocket.Hub

	cron *cron.Cron

	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new web server instance with a cancellable context.
func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) isDirectHTTPSConfigured() bool {
	certFile, certErr := s.settingService.GetCertFile()
	keyFile, keyErr := s.settingService.GetKeyFile()
	if certErr != nil || keyErr != nil || certFile == "" || keyFile == "" {
		return false
	}
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	return err == nil
}

// initRouter initializes Gin, registers middleware, templates, static
// assets, controllers and returns the configured engine.
func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()
	directHTTPS := s.isDirectHTTPSConfigured()
	engine.Use(middleware.SecurityHeadersMiddleware(directHTTPS))

	webDomain, err := s.settingService.GetWebDomain()
	if err != nil {
		return nil, err
	}

	if webDomain != "" {
		engine.Use(middleware.DomainValidatorMiddleware(webDomain))
	}

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, err
	}
	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	assetsBasePath := basePath + "assets/"

	store := cookie.NewStore(secret)
	// Configure default session cookie options, including expiration (MaxAge)
	sessionOptions := sessions.Options{
		Path:     basePath,
		HttpOnly: true,
		Secure:   directHTTPS,
		SameSite: http.SameSiteLaxMode,
	}
	if sessionMaxAge, err := s.settingService.GetSessionMaxAge(); err == nil && sessionMaxAge > 0 {
		sessionOptions.MaxAge = sessionMaxAge * 60 // minutes -> seconds
	}
	store.Options(sessionOptions)
	engine.Use(sessions.Sessions("3x-ui", store))
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
	})
	engine.Use(func(c *gin.Context) {
		uri := c.Request.RequestURI
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Header("Cache-Control", "max-age=31536000")
		}
	})

	// init i18n — still used by backend strings (errors, log messages,
	// SubPage menu entries) even though the Go template engine is gone.
	err = locale.InitLocalizer(i18nFS, &s.settingService)
	if err != nil {
		return nil, err
	}

	engine.Use(locale.LocalizerMiddleware())

	// `/assets/` serves the Vite-built bundle. In dev we pull from disk
	// so the Vite watcher's incremental rebuilds show up without
	// restarting the binary; in prod we serve the embedded dist FS
	// rooted at `dist/assets/`.
	if config.IsDebug() {
		engine.StaticFS(basePath+"assets", http.FS(os.DirFS("web/dist/assets")))
	} else {
		engine.StaticFS(basePath+"assets", http.FS(&wrapDistFS{FS: distFS}))
	}

	// Apply the redirect middleware (`/xui` to `/panel`)
	engine.Use(middleware.RedirectMiddleware(basePath))

	// Hand the embedded `dist/` filesystem to the controller package
	// before any HTML-serving controller is constructed. Phase 8
	// cutover: every HTML route reads from web/dist/ instead of
	// rendering a legacy template.
	controller.SetDistFS(distFS)

	g := engine.Group(basePath)

	s.index = controller.NewIndexController(g)
	s.panel = controller.NewXUIController(g)
	s.api = controller.NewAPIController(g, s.customGeoService)

	// Initialize WebSocket hub
	s.wsHub = websocket.NewHub()
	go s.wsHub.Run()

	// Initialize WebSocket controller — service owns per-connection pumps,
	// controller is HTTP-layer only (auth + upgrade).
	s.ws = controller.NewWebSocketController(service.NewWebSocketService(s.wsHub))
	// Register WebSocket route with basePath (g already has basePath prefix)
	g.GET("/ws", s.ws.HandleWebSocket)

	// Chrome DevTools endpoint for debugging web apps
	engine.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	// Add a catch-all route to handle undefined paths and return 404
	engine.NoRoute(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotFound)
	})

	return engine, nil
}

// startTask schedules background jobs (Xray checks, traffic jobs, cron
// jobs) which the panel relies on for periodic maintenance and monitoring.
func (s *Server) startTask(restartXray bool) {
	s.customGeoService.EnsureOnStartup()
	if restartXray {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Warning("start xray failed:", err)
		}
	}
	// Check whether xray is running every second
	s.cron.AddJob("@every 1s", job.NewCheckXrayRunningJob())

	// Check if xray needs to be restarted every 30 seconds
	s.cron.AddFunc("@every 30s", func() {
		if s.xrayService.IsNeedRestartAndSetFalse() {
			err := s.xrayService.RestartXray(false)
			if err != nil {
				logger.Error("restart xray failed:", err)
			}
		}
	})

	go func() {
		time.Sleep(time.Second * 5)
		s.cron.AddJob("@every 5s", job.NewXrayTrafficJob())
	}()

	// check client ips from log file every 10 sec
	s.cron.AddJob("@every 10s", job.NewCheckClientIpJob())

	s.cron.AddJob("@every 5s", job.NewNodeHeartbeatJob())

	s.cron.AddJob("@every 5s", job.NewNodeTrafficSyncJob())

	// check client ips from log file every day
	s.cron.AddJob("@daily", job.NewClearLogsJob())

	// Inbound traffic reset jobs
	// Run every hour
	s.cron.AddJob("@hourly", job.NewPeriodicTrafficResetJob("hourly"))
	// Run once a day, midnight
	s.cron.AddJob("@daily", job.NewPeriodicTrafficResetJob("daily"))
	// Run once a week, midnight between Sat/Sun
	s.cron.AddJob("@weekly", job.NewPeriodicTrafficResetJob("weekly"))
	// Run once a month, midnight, first of month
	s.cron.AddJob("@monthly", job.NewPeriodicTrafficResetJob("monthly"))

	// LDAP sync scheduling
	if ldapEnabled, _ := s.settingService.GetLdapEnable(); ldapEnabled {
		runtime, err := s.settingService.GetLdapSyncCron()
		if err != nil || runtime == "" {
			runtime = "@every 1m"
		}
		j := job.NewLdapSyncJob()
		// job has zero-value services with method receivers that read settings on demand
		s.cron.AddJob(runtime, j)
	}

	// Make a traffic condition every day, 8:30
	var entry cron.EntryID
	isTgbotenabled, err := s.settingService.GetTgbotEnabled()
	if (err == nil) && (isTgbotenabled) {
		runtime, err := s.settingService.GetTgbotRuntime()
		if err != nil {
			logger.Warningf("Add NewStatsNotifyJob: failed to load runtime: %v; using default @daily", err)
			runtime = "@daily"
		} else if strings.TrimSpace(runtime) == "" {
			logger.Warning("Add NewStatsNotifyJob runtime is empty, using default @daily")
			runtime = "@daily"
		}
		logger.Infof("Tg notify enabled,run at %s", runtime)
		_, err = s.cron.AddJob(runtime, job.NewStatsNotifyJob())
		if err != nil {
			logger.Warningf("Add NewStatsNotifyJob: failed to schedule runtime %q: %v", runtime, err)
			return
		}

		// check for Telegram bot callback query hash storage reset
		s.cron.AddJob("@every 2m", job.NewCheckHashStorageJob())

		// Check CPU load and alarm to TgBot if threshold passes
		cpuThreshold, err := s.settingService.GetTgCpu()
		if (err == nil) && (cpuThreshold > 0) {
			s.cron.AddJob("@every 10s", job.NewCheckCpuJob())
		}
	} else {
		s.cron.Remove(entry)
	}
}

// Start initializes and starts the web server with configured settings, routes, and background jobs.
func (s *Server) Start() (err error) {
	return s.start(true, true)
}

// StartPanelOnly initializes the panel during an in-process panel restart without cycling Xray.
func (s *Server) StartPanelOnly() (err error) {
	return s.start(false, false)
}

func (s *Server) start(restartXray bool, startTgBot bool) (err error) {
	// This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	loc, err := s.settingService.GetTimeLocation()
	if err != nil {
		return err
	}
	service.StartTrafficWriter()

	s.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	s.cron.Start()

	// Wire the inbound-runtime manager once so InboundService can route
	// add/update/delete to either the local xray or a remote node panel.
	// The closures bridge into XrayService (which owns the running xray
	// process state) without forcing the runtime package to import service.
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
		APIPort:        func() int { return s.xrayService.GetXrayAPIPort() },
		SetNeedRestart: func() { s.xrayService.SetToNeedRestart() },
	}))

	s.customGeoService = service.NewCustomGeoService()

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetPort()
	if err != nil {
		return err
	}
	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			c := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			listener = network.NewAutoHttpsListener(listener)
			listener = tls.NewListener(listener, c)
			logger.Info("Web server running HTTPS on", listener.Addr())
		} else {
			logger.Error("Error loading certificates:", err)
			logger.Info("Web server running HTTP on", listener.Addr())
		}
	} else {
		logger.Info("Web server running HTTP on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		s.httpServer.Serve(listener)
	}()

	s.startTask(restartXray)

	if startTgBot {
		isTgbotenabled, err := s.settingService.GetTgbotEnabled()
		if (err == nil) && (isTgbotenabled) {
			tgBot := s.tgbotService.NewTgbot()
			tgBot.Start(i18nFS)
		}
	}

	return nil
}

// Stop gracefully shuts down the web server, stops Xray, cron jobs, and Telegram bot.
func (s *Server) Stop() error {
	return s.stop(true, true)
}

// StopPanelOnly stops only panel-owned HTTP/background resources for an in-process panel restart.
func (s *Server) StopPanelOnly() error {
	return s.stop(false, false)
}

func (s *Server) stop(stopXray bool, stopTgBot bool) error {
	s.cancel()
	if stopXray {
		s.xrayService.StopXray()
	}
	if s.cron != nil {
		s.cron.Stop()
	}
	if stopXray {
		service.StopTrafficWriter()
	}
	if stopTgBot && s.tgbotService.IsRunning() {
		s.tgbotService.Stop()
	}
	// Gracefully stop WebSocket hub
	if s.wsHub != nil {
		s.wsHub.Stop()
	}
	var err1 error
	var err2 error
	if s.httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		err1 = s.httpServer.Shutdown(shutdownCtx)
	}
	if s.listener != nil {
		err2 = s.listener.Close()
	}
	return common.Combine(err1, err2)
}

// GetCtx returns the server's context for cancellation and deadline management.
func (s *Server) GetCtx() context.Context {
	return s.ctx
}

// GetCron returns the server's cron scheduler instance.
func (s *Server) GetCron() *cron.Cron {
	return s.cron
}

// GetWSHub returns the WebSocket hub instance.
func (s *Server) GetWSHub() any {
	return s.wsHub
}

func (s *Server) RestartXray() error {
	return s.xrayService.RestartXray(true)
}
