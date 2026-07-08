// Package web provides the main web server implementation for the 3x-ui panel,
// including HTTP/HTTPS serving, routing, templates, and background job scheduling.
package web

import (
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
	"github.com/mhsanaei/3x-ui/v3/internal/web/controller"
	"github.com/mhsanaei/3x-ui/v3/internal/web/job"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/network"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/email"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

//go:embed translation/*
var i18nFS embed.FS

// distFS embeds the Vite-built frontend (internal/web/dist/). Every user-facing
// HTML route is served straight out of this FS — the legacy Go
// templates and `web/assets/` tree are gone post-Phase 8.

//go:embed all:dist
var distFS embed.FS

var startTime = time.Now()

// cronPanicLogger adapts the package logger to cron's Printf-style logger so a
// panicking scheduled job is recovered and logged instead of crashing the panel.
type cronPanicLogger struct{}

func (cronPanicLogger) Printf(format string, args ...any) { logger.Errorf(format, args...) }

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

	xrayService    service.XrayService
	settingService service.SettingService
	tgbotService   tgbot.Tgbot

	wsHub *websocket.Hub

	bus  *eventbus.Bus
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
	sendHSTS := directHTTPS && !config.IsSkipHSTS()
	engine.Use(middleware.SecurityHeadersMiddleware(sendHSTS))

	// Cap request bodies on state-changing requests so a stolen session/API
	// token or a buggy client can't force large allocations or long DB
	// transactions via bulk create/attach/import endpoints. GET/HEAD/OPTIONS
	// carry no body and are left untouched. Database restore legitimately accepts
	// large backups and streams them to disk, so only its exact route suffix is
	// exempt. Follow-up: make the limit a setting.
	const maxRequestBodyBytes = 10 << 20 // 10 MiB
	engine.Use(middleware.MaxBodyBytes(maxRequestBodyBytes, "/panel/api/server/importDB"))

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
		engine.StaticFS(basePath+"assets", http.FS(os.DirFS("internal/web/dist/assets")))
	} else {
		engine.StaticFS(basePath+"assets", http.FS(&wrapDistFS{FS: distFS}))
	}

	// Hand the embedded `dist/` filesystem to the controller package
	// before any HTML-serving controller is constructed. Phase 8
	// cutover: every HTML route reads from internal/web/dist/ instead of
	// rendering a legacy template.
	controller.SetDistFS(distFS)

	g := engine.Group(basePath)

	s.index = controller.NewIndexController(g)
	s.panel = controller.NewXUIController(g)
	g.GET("/panel/api/openapi.json", controller.ServeOpenAPISpec)
	s.api = controller.NewAPIController(g)

	// Initialize WebSocket hub
	s.wsHub = websocket.NewHub()
	go s.wsHub.Run()

	// Initialize WebSocket controller — service owns per-connection pumps,
	// controller is HTTP-layer only (auth + upgrade).
	s.ws = controller.NewWebSocketController(panel.NewWebSocketService(s.wsHub))
	// Register WebSocket route with basePath (g already has basePath prefix)
	g.GET("/ws", s.ws.HandleWebSocket)

	// Chrome DevTools endpoint for debugging web apps
	engine.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	// Let unknown panel document routes fall back to the SPA shell, while every
	// non-SPA miss still returns a hard 404.
	engine.NoRoute(func(c *gin.Context) {
		if s.panel.HandleNoRoutePanelSPA(c) {
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	})

	return engine, nil
}

// Background-job cadences. Centralized here as the single tuning surface; the
// values are unchanged from the historical hardcoded cron specs. Follow-up:
// make these configurable via settings, add per-tick jitter to de-synchronize
// fleet load, skip expensive jobs when no WebSocket clients are connected or
// node/xray state is unchanged, and export per-job duration/skipped/error
// counters.
const (
	cadenceXrayRunning   = "@every 1s"
	cadenceXrayRestart   = "@every 30s"
	cadenceXrayTraffic   = "@every 5s"
	cadenceMtproto       = "@every 10s"
	cadenceClientIPScan  = "@every 10s"
	cadenceNodeHeartbeat = "@every 5s"
	cadenceNodeTraffic   = "@every 5s"
	cadenceOutboundSub   = "@every 5m"
	cadenceXrayLogPrune  = "@every 10m"
	cadenceCheckHash     = "@every 2m"
	// cpu.Percent samples over a full minute (blocking), so a finer cadence just
	// stacks overlapping samplers; subscribers rate-limit alerts to 1/min anyway.
	cadenceCPUAlarm    = "@every 1m"
	cadenceMemoryAlarm = "@every 1m"
)

// startTask schedules background jobs (Xray checks, traffic jobs, cron
// jobs) which the panel relies on for periodic maintenance and monitoring.
func (s *Server) startTask(restartXray bool) {
	if restartXray {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Warning("start xray failed:", err)
		}
	}
	// Check whether xray is running every second
	_, _ = s.cron.AddJob(cadenceXrayRunning, job.NewCheckXrayRunningJob())

	// Check if xray needs to be restarted every 30 seconds
	_, _ = s.cron.AddFunc(cadenceXrayRestart, func() {
		if s.xrayService.IsNeedRestartAndSetFalse() {
			err := s.xrayService.RestartXray(false)
			if err != nil {
				logger.Error("restart xray failed:", err)
			}
		}
	})

	go func() {
		time.Sleep(time.Second * 5)
		_, _ = s.cron.AddJob(cadenceXrayTraffic, job.NewXrayTrafficJob())
	}()

	// Reconcile mtproto (mtg) sidecars and scrape their traffic
	mtJob := job.NewMtprotoJob()
	_, _ = s.cron.AddJob(cadenceMtproto, mtJob)
	go mtJob.Run()

	// check client ips from log file every 10 sec
	_, _ = s.cron.AddJob(cadenceClientIPScan, job.NewCheckClientIpJob())

	_, _ = s.cron.AddJob(cadenceNodeHeartbeat, job.NewNodeHeartbeatJob())

	_, _ = s.cron.AddJob(cadenceNodeTraffic, job.NewNodeTrafficSyncJob())

	// Outbound subscription auto-refresh (respects per-sub updateInterval)
	_, _ = s.cron.AddJob(cadenceOutboundSub, job.NewOutboundSubscriptionJob())

	// check client ips from log file every day
	_, _ = s.cron.AddJob("@daily", job.NewClearLogsJob())
	_, _ = s.cron.AddJob(cadenceXrayLogPrune, job.NewPruneXrayLogsJob())
	_, _ = s.cron.AddJob("@hourly", job.NewWarpIpJob())

	// Inbound traffic reset jobs
	// Run every hour
	_, _ = s.cron.AddJob("@hourly", job.NewPeriodicTrafficResetJob("hourly"))
	// Run once a day, midnight
	_, _ = s.cron.AddJob("@daily", job.NewPeriodicTrafficResetJob("daily"))
	// Run once a week, midnight between Sat/Sun
	_, _ = s.cron.AddJob("@weekly", job.NewPeriodicTrafficResetJob("weekly"))
	// Run once a month, midnight, first of month
	_, _ = s.cron.AddJob("@monthly", job.NewPeriodicTrafficResetJob("monthly"))

	// LDAP sync scheduling
	if ldapEnabled, _ := s.settingService.GetLdapEnable(); ldapEnabled {
		runtime, err := s.settingService.GetLdapSyncCron()
		if err != nil || runtime == "" {
			runtime = "@every 1m"
		}
		j := job.NewLdapSyncJob()
		// job has zero-value services with method receivers that read settings on demand
		_, _ = s.cron.AddJob(runtime, j)
	}

	// Telegram-bot–dependent jobs: periodic stats report + callback-hash cleanup.
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
		if _, err = s.cron.AddJob(runtime, job.NewStatsNotifyJob()); err != nil {
			logger.Warningf("Add NewStatsNotifyJob: failed to schedule runtime %q: %v", runtime, err)
		}

		// check for Telegram bot callback query hash storage reset
		_, _ = s.cron.AddJob(cadenceCheckHash, job.NewCheckHashStorageJob())
	}

	// CPU monitor publishes cpu.high events; register it whenever any notifier
	// (Telegram or Email) wants them, independent of the Telegram bot being on.
	if s.cpuAlarmWanted() {
		_, _ = s.cron.AddJob(cadenceCPUAlarm, job.NewCheckCpuJob())
	}
	// Memory monitor publishes memory.high events; register it whenever any notifier wants them.
	if s.memoryAlarmWanted() {
		_, _ = s.cron.AddJob(cadenceMemoryAlarm, job.NewCheckMemJob())
	}

	if mins := sys.MemoryReleaseIntervalMinutes(); mins > 0 {
		_, _ = s.cron.AddJob(fmt.Sprintf("@every %dm", mins), job.NewMemoryReleaseJob())
		go func() {
			time.Sleep(time.Minute)
			job.NewMemoryReleaseJob().Run()
		}()
	}
}

// cpuAlarmWanted reports whether any notifier is configured to receive cpu.high
// alerts, so the minute-long blocking CPU sampler only runs when it's needed.
func (s *Server) cpuAlarmWanted() bool {
	wants := func(events string, threshold int) bool {
		if threshold <= 0 {
			return false
		}
		for e := range strings.SplitSeq(events, ",") {
			if strings.TrimSpace(e) == string(eventbus.EventCPUHigh) {
				return true
			}
		}
		return false
	}
	if on, _ := s.settingService.GetTgbotEnabled(); on {
		events, _ := s.settingService.GetTgEnabledEvents()
		cpu, _ := s.settingService.GetTgCpu()
		if wants(events, cpu) {
			return true
		}
	}
	if on, _ := s.settingService.GetSmtpEnable(); on {
		events, _ := s.settingService.GetSmtpEnabledEvents()
		cpu, _ := s.settingService.GetSmtpCpu()
		if wants(events, cpu) {
			return true
		}
	}
	return false
}

// memoryAlarmWanted reports whether any notifier is configured to receive memory.high alerts.
func (s *Server) memoryAlarmWanted() bool {
	wants := func(events string, threshold int) bool {
		if threshold <= 0 {
			return false
		}
		for e := range strings.SplitSeq(events, ",") {
			if strings.TrimSpace(e) == string(eventbus.EventMemoryHigh) {
				return true
			}
		}
		return false
	}
	if on, _ := s.settingService.GetTgbotEnabled(); on {
		events, _ := s.settingService.GetTgEnabledEvents()
		mem, _ := s.settingService.GetTgMemory()
		if wants(events, mem) {
			return true
		}
	}
	if on, _ := s.settingService.GetSmtpEnable(); on {
		events, _ := s.settingService.GetSmtpEnabledEvents()
		mem, _ := s.settingService.GetSmtpMemory()
		if wants(events, mem) {
			return true
		}
	}
	return false
}

// Start initializes and starts the web server with configured settings, routes, and background jobs.
func (s *Server) Start() (err error) {
	return s.start(true, true)
}

func (s *Server) StartPanelOnly() (err error) {
	return s.start(false, true)
}

func (s *Server) start(restartXray bool, startTgBot bool) (err error) {
	// This is an anonymous function, no function name
	defer func() {
		if err != nil {
			_ = s.Stop()
		}
	}()

	loc, err := s.settingService.GetTimeLocation()
	if err != nil {
		return err
	}
	service.StartTrafficWriter()

	// SkipIfStillRunning stops a slow job (e.g. the 5s traffic poll on a large
	// install) from overlapping itself: two concurrent runs of the same job race
	// the shared xrayAPI — leaking a grpc connection — and the StatsLastValues
	// map, whose concurrent write is a fatal runtime throw cron.Recover can't
	// catch. cron.Recover then logs any panic and keeps the scheduler alive.
	s.cron = cron.New(
		cron.WithLocation(loc),
		cron.WithSeconds(),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DiscardLogger),
			cron.Recover(cron.PrintfLogger(cronPanicLogger{})),
		),
	)
	s.cron.Start()

	// Wire the inbound-runtime manager once so InboundService can route
	// add/update/delete to either the local xray or a remote node panel.
	// The closures bridge into XrayService (which owns the running xray
	// process state) without forcing the runtime package to import service.
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
		APIPort:        func() int { return s.xrayService.GetXrayAPIPort() },
		SetNeedRestart: func() { s.xrayService.SetToNeedRestart() },
	}))
	runtime.GetManager().SetNodeEgressResolver(&s.settingService)
	// Supply the master client certificate for nodes in mtls mode. Issued lazily
	// from the node CA on first use; runtime stays free of a service import.
	runtime.SetMasterClientCertProvider(func() (tls.Certificate, error) {
		ck, err := s.settingService.EnsureMasterClientCert()
		if err != nil {
			return tls.Certificate{}, err
		}
		return tls.X509KeyPair(ck.CertPEM, ck.KeyPEM)
	})

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
	if envPort, configured, envErr := config.GetPortOverride(); configured {
		if envErr != nil {
			logger.Warning("Ignoring invalid XUI_PORT; using configured web port:", port, envErr)
		} else {
			port = envPort
			logger.Info("Using XUI_PORT override for web panel port:", port)
		}
	}
	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", listenAddr)
	if err != nil {
		return err
	}
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			c := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			// Opt-in node mTLS: when a trust CA is configured, request and verify
			// client certs (VerifyClientCertIfGiven keeps browsers working). With
			// no CA the listener is unchanged.
			if pool, perr := s.settingService.NodeMtlsClientCAPool(); perr != nil {
				logger.Warning("node mTLS: failed to build client CA trust pool:", perr)
			} else if pool != nil {
				applyNodeMtls(c, pool)
				logger.Info("Node mTLS enabled: verifying client certificates for the node API")
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
		_ = s.httpServer.Serve(listener)
	}()

	// Create event bus before startTask so jobs can use it
	s.bus = eventbus.New(eventbus.DefaultBufferSize)
	service.SetEventBus(s.bus)
	job.EventBus = s.bus
	tgbot.EventBus = s.bus

	// Wire xray crash callback BEFORE startTask so it's ready
	xray.OnCrash = func(err error) {
		if s.bus != nil {
			s.bus.Publish(eventbus.Event{
				Type: eventbus.EventXrayCrash,
				Data: err.Error(),
			})
		}
	}

	// Register email subscriber (always — it checks smtpEnable at runtime)
	emailService := email.NewEmailService(s.settingService)
	emailSub := email.NewSubscriber(s.settingService, emailService)
	s.bus.Subscribe("email-notifier", emailSub.HandleEvent)

	// Wire email service to controller for test endpoint
	controller.SetEmailService(emailService)

	// Wire Telegram test function to controller
	controller.SetTestTgFunc(func() error {
		if !s.tgbotService.IsRunning() {
			return fmt.Errorf("telegram bot is not running (check token and chat ID)")
		}
		if err := s.tgbotService.TestConnection(); err != nil {
			return fmt.Errorf("telegram API test failed: %w", err)
		}
		s.tgbotService.SendMsgToTgbotAdmins("✅ Test message from 3x-ui")
		return nil
	})

	controller.SetReloadTgbotFunc(func() {
		enabled, err := s.settingService.GetTgbotEnabled()
		if err != nil || !enabled {
			if s.tgbotService.IsRunning() {
				s.tgbotService.Stop()
			}
			if s.bus != nil {
				s.bus.Unsubscribe("tg-notifier")
			}
			return
		}
		// Start() stops any previous receiver first, so it is safe whether or not the bot is already running.
		tgBot := s.tgbotService.NewTgbot()
		if startErr := tgBot.Start(i18nFS); startErr != nil {
			logger.Warning("reload Telegram bot failed:", startErr)
			return
		}
		if s.bus != nil {
			s.bus.Subscribe("tg-notifier", s.tgbotService.HandleEvent)
		}
	})

	s.startTask(restartXray)

	if startTgBot {
		isTgbotenabled, err := s.settingService.GetTgbotEnabled()
		if (err == nil) && (isTgbotenabled) {
			tgBot := s.tgbotService.NewTgbot()
			_ = tgBot.Start(i18nFS)
			// Subscribe Telegram notifications for event bus
			s.bus.Subscribe("tg-notifier", s.tgbotService.HandleEvent)
		}
	}

	return nil
}

// Stop gracefully shuts down the web server, stops Xray, cron jobs, and Telegram bot.
func (s *Server) Stop() error {
	return s.stop(true, true)
}

func (s *Server) StopPanelOnly() error {
	return s.stop(false, true)
}

func (s *Server) stop(stopXray bool, stopTgBot bool) error {
	s.cancel()
	if stopXray {
		_ = s.xrayService.StopXray()
		mtproto.GetManager().StopAll()
	}
	if s.cron != nil {
		s.cron.Stop()
	}
	if s.bus != nil {
		s.bus.Stop()
	}
	if err := service.PersistSystemMetrics(); err != nil {
		logger.Warning("persist system metrics on shutdown failed:", err)
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
