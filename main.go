// Package main is the entry point for the 3x-ui web panel application.
// It initializes the database, web server, and handles command-line operations for managing the panel.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	_ "unsafe"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/tunnelmonitor"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
	"github.com/mhsanaei/3x-ui/v3/internal/web"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"

	"github.com/joho/godotenv"
	"github.com/op/go-logging"
)

// cliFallbackTokenName is the single token the CLI regenerates, so `-getApiToken`
// cannot accumulate admin-equivalent credentials that are never revoked.
const cliFallbackTokenName = "cli-fallback"

// installTokenName is minted once on a panel with no tokens and is deliberately
// not the rotated slot, so the credential the installer recorded keeps working.
const installTokenName = "install"

// initNodeTokenCrypto loads the process codec, preferring the key file over
// the environment and failing closed when an enabled policy lacks a key.
func initNodeTokenCrypto() error {
	mode, err := nodetoken.ParseMode(config.GetNodeTokenEncryptionMode())
	if err != nil {
		return err
	}
	if mode == nodetoken.ModeOff {
		c, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
		nodetoken.Init(c)
		return nil
	}
	ring, ferr := (nodetoken.FileKeySource{Path: config.GetNodeTokenKeyFile()}).Load()
	if ferr != nil {
		var eerr error
		if ring, eerr = (nodetoken.EnvKeySource{Var: config.GetNodeTokenKeyEnv()}).Load(); eerr != nil {
			return fmt.Errorf("load node-token key: file: %w; env: %w", ferr, eerr)
		}
	}
	c, err := nodetoken.NewCodec(mode, ring)
	if err != nil {
		return err
	}
	nodetoken.Init(c)
	// The CLI runs before package logger initialization, so use log.Printf.
	log.Printf("node-token encryption enabled (mode=%s, active-key=%s)", config.GetNodeTokenEncryptionMode(), c.ActiveKeyID())
	return nil
}

// runWebServer initializes and starts the web server for the 3x-ui panel.
func runWebServer() {
	log.Printf("Starting %v %v", config.GetName(), config.GetPanelVersion())

	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Notice:
		logger.InitLogger(logging.NOTICE)
	case config.Warning:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	_ = godotenv.Load()

	for _, line := range sys.ApplyMemoryTuning() {
		logger.Info(line)
	}

	if os.Getenv("XUI_PPROF") == "true" {
		go func() {
			logger.Info("pprof profiling server listening on 127.0.0.1:6060")
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				logger.Warning("pprof server stopped: ", err)
			}
		}()
	}

	if err := initNodeTokenCrypto(); err != nil {
		log.Fatalf("Error initializing node-token encryption: %v", err)
	}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	server := web.NewServer()
	global.SetWebServer(server)
	err = server.Start()
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
	}

	sub.SetDistFS(web.EmbeddedDist())
	service.RegisterSubLinkProvider(sub.NewLinkProvider())
	subServer := sub.NewServer()
	global.SetSubServer(subServer)
	err = subServer.Start()
	if err != nil {
		log.Fatalf("Error starting sub server: %v", err)
	}

	sigCh := make(chan os.Signal, 8)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, sys.SIGUSR1, os.Interrupt)
	global.SetRestartHook(func() {
		select {
		case sigCh <- syscall.SIGHUP:
		default:
		}
	})

	var stopTunnelHealthMonitor context.CancelFunc
	monitorCfg := tunnelmonitor.ConfigFromEnv()
	if monitorCfg.Enabled {
		if monitorCfg.ProxyURL == "" {
			logger.Warning("Tunnel health monitor enabled without XUI_TUNNEL_HEALTH_PROXY: the probe measures host connectivity, not the xray tunnel, so failures will restart xray without fixing host network issues")
		}

		monitorCtx, cancel := context.WithCancel(context.Background())
		stopTunnelHealthMonitor = cancel

		monitor, err := tunnelmonitor.New(monitorCfg, func(_ context.Context) error {
			logger.Warning("Tunnel health monitor threshold reached, restarting xray-core")
			return server.RestartXray()
		})
		if err != nil {
			logger.Warning("Tunnel health monitor disabled: ", err)
		} else {
			go monitor.Run(monitorCtx)
		}
	}
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting servers...")

			err := server.StopPanelOnly()
			if err != nil {
				logger.Debug("Error stopping web server:", err)
			}
			err = subServer.Stop()
			if err != nil {
				logger.Debug("Error stopping sub server:", err)
			}

			server = web.NewServer()
			global.SetWebServer(server)
			err = server.StartPanelOnly()
			if err != nil {
				log.Fatalf("Error restarting web server: %v", err)
			}
			log.Println("Web server restarted successfully.")

			sub.SetDistFS(web.EmbeddedDist())
			subServer = sub.NewServer()
			global.SetSubServer(subServer)
			err = subServer.Start()
			if err != nil {
				log.Fatalf("Error restarting sub server: %v", err)
			}
			log.Println("Sub server restarted successfully.")
		case sys.SIGUSR1:
			logger.Info("Received USR1 signal, restarting xray-core...")
			err := server.RestartXray()
			if err != nil {
				logger.Error("Failed to restart xray-core:", err)
			}

		default:
			if stopTunnelHealthMonitor != nil {
				stopTunnelHealthMonitor()
			}

			// --- FIX FOR TELEGRAM BOT CONFLICT (409) on full shutdown ---
			tgbot.StopBot()
			// ------------------------------------------------------------

			_ = server.Stop()
			_ = subServer.Stop()
			log.Println("Shutting down servers.")
			return
		}
	}
}

// resetSetting resets all panel settings to their default values.
func resetSetting() error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return err
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings:", err)
		return err
	} else {
		fmt.Println("Settings successfully reset.")
	}
	return nil
}

// showSetting displays the current panel settings if show is true.
func showSetting(show bool) {
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println("get current port failed, error info:", err)
		}

		webBasePath, err := settingService.GetBasePath()
		if err != nil {
			fmt.Println("get webBasePath failed, error info:", err)
		}

		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		userService := panel.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println("get current user info failed, error info:", err)
		}

		if userModel.Username == "" || userModel.Password == "" {
			fmt.Println("current username or password is empty")
		}

		fmt.Println("current panel settings as follows:")
		if certFile == "" || keyFile == "" {
			fmt.Println("Warning: Panel is not secure with SSL")
		} else {
			fmt.Println("Panel is secure with SSL")
		}

		hasDefaultCredential := func() bool {
			return userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
		}()

		fmt.Println("hasDefaultCredential:", hasDefaultCredential)
		fmt.Println("port:", port)
		fmt.Println("webBasePath:", webBasePath)
	}
}

// updateTgbotEnableSts enables or disables the Telegram bot notifications based on the status parameter.
func updateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotEnabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotEnabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotEnabled[%v] success", status)
		}
	}
}

// updateTgbotSetting updates Telegram bot settings including token, chat ID, and runtime schedule.
func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}

	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Printf("Error setting Telegram bot token: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot token.")
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Printf("Error setting Telegram bot runtime: %v\n", err)
			return
		}
		logger.Infof("Successfully updated Telegram bot runtime to [%s].", tgBotRuntime)
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Printf("Error setting Telegram bot chat ID: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot chat ID.")
	}
}

// encryptNodeTokens re-encrypts stored tokens after enablement or rotation.
// It requires migration|required mode and a configured key.
func encryptNodeTokens() {
	_ = godotenv.Load()
	if err := initNodeTokenCrypto(); err != nil {
		fmt.Println("node-token encryption init failed:", err)
		os.Exit(1)
	}
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println("database initialization failed:", err)
		os.Exit(1)
	}
	changed, skipped, err := (&service.NodeService{}).MigrateNodeTokensToActiveKey()
	if err != nil {
		fmt.Println("token migration failed:", err)
		os.Exit(1)
	}
	fmt.Printf("node-token migration complete: %d re-encrypted, %d already current/skipped\n", changed, skipped)
}

// updateSetting updates various panel settings including port, credentials, base path, listen IP, and two-factor authentication.
func updateSetting(port int, username string, password string, webBasePath string, listenIP string, resetTwoFactor bool) error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Database initialization failed:", err)
		return err
	}

	settingService := service.SettingService{}
	userService := panel.UserService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("Failed to set port:", err)
		} else {
			fmt.Printf("Port set successfully: %v\n", port)
		}
	}

	if username != "" || password != "" {
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("Failed to update username and password:", err)
		} else {
			fmt.Println("Username and password updated successfully")
		}
	}

	if webBasePath != "" {
		err := settingService.SetBasePath(webBasePath)
		if err != nil {
			fmt.Println("Failed to set base URI path:", err)
		} else {
			fmt.Println("Base URI path set successfully")
		}
	}

	if resetTwoFactor {
		err := settingService.SetTwoFactorEnable(false)

		if err != nil {
			fmt.Println("Failed to reset two-factor authentication:", err)
		} else {
			_ = settingService.SetTwoFactorToken("")
			fmt.Println("Two-factor authentication reset successfully")
		}
	}

	if listenIP != "" {
		err := settingService.SetListen(listenIP)
		if err != nil {
			fmt.Println("Failed to set listen IP:", err)
		} else {
			fmt.Printf("listen %v set successfully\n", listenIP)
		}
	}

	return nil
}

// updateCert updates the SSL certificate files for the panel.
func updateCert(publicKey string, privateKey string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if (privateKey != "" && publicKey != "") || (privateKey == "" && publicKey == "") {
		settingService := service.SettingService{}
		err = settingService.SetCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate public key failed:", err)
		} else {
			fmt.Println("set certificate public key success")
		}

		err = settingService.SetKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate private key failed:", err)
		} else {
			fmt.Println("set certificate private key success")
		}

		err = settingService.SetSubCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate for subscription public key failed:", err)
		} else {
			fmt.Println("set certificate for subscription public key success")
		}

		err = settingService.SetSubKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate for subscription private key failed:", err)
		} else {
			fmt.Println("set certificate for subscription private key success")
		}
	} else {
		fmt.Println("both public and private key should be entered.")
	}
}

// GetCertificate displays the current SSL certificate settings if getCert is true.
func GetCertificate(getCert bool) {
	if getCert {
		settingService := service.SettingService{}
		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		fmt.Println("cert:", certFile)
		fmt.Println("key:", keyFile)
	}
}

// GetListenIP displays the current panel listen IP address if getListen is true.
func GetListenIP(getListen bool) {
	if getListen {

		settingService := service.SettingService{}
		ListenIP, err := settingService.GetListen()
		if err != nil {
			log.Printf("Failed to retrieve listen IP: %v", err)
			return
		}

		fmt.Println("listenIP:", ListenIP)
	}
}

func GetApiToken(getApiToken bool, tokenName string) {
	if !getApiToken {
		return
	}
	// An explicit name applies to both branches below; without one each keeps
	// the name it already used, so every existing invocation is unaffected.
	name := strings.TrimSpace(tokenName)
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("open database failed, error info:", err)
		return
	}
	apiTokenService := panel.ApiTokenService{}
	tokens, err := apiTokenService.List()
	if err != nil {
		fmt.Println("get apiToken failed, error info:", err)
		return
	}
	if len(tokens) > 0 {
		fmt.Printf("There are %d API token(s) configured. Existing tokens cannot be retrieved in plaintext because only hashes are stored.\n", len(tokens))
		fmt.Println("If you have lost your token, you can manage and generate new tokens through the Panel UI (Settings -> API Tokens).")

		// Rotate one token per name so repeated calls cannot pile up
		// indefinitely many admin-equivalent tokens that never expire.
		rotated := name
		if rotated == "" {
			rotated = cliFallbackTokenName
		}
		created, err := apiTokenService.RecreateByName(rotated)
		if err != nil {
			fmt.Println("Failed to create a fallback API token:", err)
			return
		}
		fmt.Printf("\nThe API token %q has been regenerated (any previous one is now invalid):\n", rotated)
		fmt.Println("apiToken:", created.Token)
		return
	}
	if name == "" {
		name = installTokenName
	}
	created, err := apiTokenService.Create(name, "", 0)
	if err != nil {
		fmt.Println("create apiToken failed, error info:", err)
		return
	}
	fmt.Println("apiToken:", created.Token)
}

// migrateDb performs database migration operations for the 3x-ui panel.
func migrateDb() {
	inboundService := service.InboundService{}

	logger.InitLogger(logging.INFO)
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database...")
	inboundService.MigrateDB()
	fmt.Println("Migration done!")
}

// loadServiceEnvFile loads the systemd EnvironmentFile so CLI subcommands like
// "x-ui setting" hit the same database backend as the panel. godotenv.Load does
// not override variables already in the environment, so it is a no-op for the
// systemd-managed service.
func loadServiceEnvFile() {
	for _, path := range config.GetEnvFilePaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Printf("warning: failed to load env file %s: %v", path, err)
		}
		return
	}
}

// main is the entry point of the 3x-ui application.
// It parses command-line arguments to run the web server, migrate database, or update settings.
func main() {
	loadServiceEnvFile()

	if len(os.Args) < 2 {
		runWebServer()
		return
	}

	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)

	migrateDbCmd := flag.NewFlagSet("migrate-db", flag.ExitOnError)
	var migrateDsn string
	var migrateSrc string
	var migrateDump string
	var migrateRestore string
	var migrateOut string
	migrateDbCmd.StringVar(&migrateDsn, "dsn", "", "Destination PostgreSQL DSN (postgres://user:pass@host:port/db?sslmode=disable)")
	migrateDbCmd.StringVar(&migrateSrc, "src", "", "Source SQLite file (defaults to the configured x-ui.db)")
	migrateDbCmd.StringVar(&migrateDump, "dump", "", "Write a portable SQL text dump of --src to this file (.db -> .dump)")
	migrateDbCmd.StringVar(&migrateRestore, "restore", "", "Rebuild a SQLite database from this SQL text dump (.dump -> .db); requires --out")
	migrateDbCmd.StringVar(&migrateOut, "out", "", "Destination SQLite file for --restore (must not already exist)")

	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var port int
	var username string
	var password string
	var webBasePath string
	var listenIP string
	var getListen bool
	var webCertFile string
	var webKeyFile string
	var tgbottoken string
	var tgbotchatid string
	var enabletgbot bool
	var tgbotRuntime string
	var reset bool
	var show bool
	var getCert bool
	var getApiToken bool
	var tokenName string
	var resetTwoFactor bool
	settingCmd.BoolVar(&reset, "reset", false, "Reset all settings")
	settingCmd.BoolVar(&show, "show", false, "Display current settings")
	settingCmd.IntVar(&port, "port", 0, "Set panel port number")
	settingCmd.StringVar(&username, "username", "", "Set login username")
	settingCmd.StringVar(&password, "password", "", "Set login password")
	settingCmd.StringVar(&webBasePath, "webBasePath", "", "Set base path for Panel")
	settingCmd.StringVar(&listenIP, "listenIP", "", "set panel listenIP IP")
	settingCmd.BoolVar(&resetTwoFactor, "resetTwoFactor", false, "Reset two-factor authentication settings")
	settingCmd.BoolVar(&getListen, "getListen", false, "Display current panel listenIP IP")
	settingCmd.BoolVar(&getCert, "getCert", false, "Display current certificate settings")
	settingCmd.BoolVar(&getApiToken, "getApiToken", false, "Print an API token for CLI use, regenerating it and invalidating the previous one; on a panel with no tokens yet it mints one instead")
	settingCmd.StringVar(&tokenName, "tokenName", "", "Name of the token -getApiToken acts on (default: "+cliFallbackTokenName+", or "+installTokenName+" on a panel with no tokens)")
	settingCmd.StringVar(&webCertFile, "webCert", "", "Set path to public key file for panel")
	settingCmd.StringVar(&webKeyFile, "webCertKey", "", "Set path to private key file for panel")
	settingCmd.StringVar(&tgbottoken, "tgbottoken", "", "Set token for Telegram bot")
	settingCmd.StringVar(&tgbotRuntime, "tgbotRuntime", "", "Set cron time for Telegram bot notifications")
	settingCmd.StringVar(&tgbotchatid, "tgbotchatid", "", "Set chat ID for Telegram bot notifications")
	settingCmd.BoolVar(&enabletgbot, "enabletgbot", false, "Enable notifications via Telegram bot")

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Print(commandHelp())
	}

	flag.Parse()
	if showVersion {
		fmt.Println(config.GetPanelVersion())
		return
	}

	switch os.Args[1] {
	case "run":
		err := runCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		runWebServer()
	case "migrate":
		migrateDb()
	case "encrypt-tokens":
		encryptNodeTokens()
	case "migrate-db":
		if err := migrateDbCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println(err)
			return
		}
		src := migrateSrc
		if src == "" {
			src = config.GetDBPath()
		}
		switch {
		case migrateDump != "":
			if err := database.DumpSQLite(src, migrateDump); err != nil {
				fmt.Println("dump failed:", err)
				os.Exit(1)
			}
			fmt.Printf("Dumped %s -> %s\n", src, migrateDump)
		case migrateRestore != "":
			if migrateOut == "" {
				fmt.Println("--out is required when using --restore: the destination .db path (must not exist)")
				return
			}
			if err := database.RestoreSQLite(migrateRestore, migrateOut); err != nil {
				fmt.Println("restore failed:", err)
				os.Exit(1)
			}
			fmt.Printf("Restored %s -> %s\n", migrateRestore, migrateOut)
		case migrateDsn != "":
			if err := database.MigrateData(src, migrateDsn); err != nil {
				fmt.Println("migration failed:", err)
				os.Exit(1)
			}
		default:
			fmt.Println("nothing to do: pass --dump <file>, --restore <file> --out <db>, or --dsn <postgres-dsn>")
		}
	case "setting":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		// flag stops parsing at the first non-flag argument, so the `-getApiToken true`
		// form drops every flag written after it. Say so instead of acting on a default.
		if rest := settingCmd.Args(); len(rest) > 0 {
			fmt.Printf("warning: ignored %q and any flags after it; put flags before positional arguments\n", strings.Join(rest, " "))
		}
		if reset {
			if err = resetSetting(); err != nil {
				return
			}
		} else {
			if err = updateSetting(port, username, password, webBasePath, listenIP, resetTwoFactor); err != nil {
				return
			}
		}
		if webCertFile != "" || webKeyFile != "" {
			updateCert(webCertFile, webKeyFile)
		}
		if show {
			showSetting(show)
		}
		if getListen {
			GetListenIP(getListen)
		}
		if getCert {
			GetCertificate(getCert)
		}
		if getApiToken {
			GetApiToken(getApiToken, tokenName)
		}
		if (tgbottoken != "") || (tgbotchatid != "") || (tgbotRuntime != "") {
			updateTgbotSetting(tgbottoken, tgbotchatid, tgbotRuntime)
		}
		if enabletgbot {
			updateTgbotEnableSts(enabletgbot)
		}
	case "cert":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if reset {
			updateCert("", "")
		} else {
			updateCert(webCertFile, webKeyFile)
		}
	default:
		fmt.Println("Invalid subcommands")
		fmt.Println()
		runCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
	}
}

func commandHelp() string {
	return `
Commands:
    run            run web panel
    migrate        migrate from other/old x-ui
    migrate-db     SQLite <-> .dump (--dump/--restore) or copy into PostgreSQL (--dsn)
    encrypt-tokens encrypt node bearer tokens with the configured active key
    setting        set settings
`
}
