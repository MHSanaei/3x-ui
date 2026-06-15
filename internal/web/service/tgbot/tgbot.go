package tgbot

import (
	"context"
	"crypto/rand"
	"embed"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

var (
	bot *telego.Bot

	// botCancel stores the function to cancel the context, stopping Long Polling gracefully.
	botCancel context.CancelFunc
	// tgBotMutex protects concurrent access to botCancel variable
	tgBotMutex sync.Mutex
	// botWG waits for the OnReceive Long Polling goroutine to finish.
	botWG sync.WaitGroup

	botHandler  *th.BotHandler
	adminIds    []int64
	isRunning   bool
	hostname    string
	hashStorage *global.HashStorage

	// EventBus is set from web layer to publish login/security events.
	EventBus *eventbus.Bus

	// Performance improvements
	messageWorkerPool   chan struct{} // Semaphore for limiting concurrent message processing
	optimizedHTTPClient *http.Client  // HTTP client with connection pooling and timeouts

	// Simple cache for frequently accessed data
	statusCache struct {
		data      *service.Status
		timestamp time.Time
		mutex     sync.RWMutex
	}

	serverStatsCache struct {
		data      string
		timestamp time.Time
		mutex     sync.RWMutex
	}

	// clients data to adding new client. receiver_inbound_IDs is the set of
	// inbounds the new client will be attached to; receiver_inbound_ID mirrors
	// the primary pick for the legacy attach-picker entry point. Per-protocol
	// secrets (UUID, password, flow, method) are filled per-inbound on submit
	// by ClientService.fillProtocolDefaults, so the bot only tracks universal
	// client fields here.
	receiver_inbound_ID  int
	receiver_inbound_IDs []int
	client_Email         string
	client_LimitIP       int
	client_TotalGB       int64
	client_ExpiryTime    int64
	client_Enable        bool
	client_TgID          string
	client_SubID         string
	client_Comment       string
	client_Reset         int
)

var userStates = make(map[int64]string)

// LoginStatus represents the result of a login attempt.
type LoginStatus byte

// Login status constants
const (
	LoginSuccess        LoginStatus = 1        // Login was successful
	LoginFail           LoginStatus = 0        // Login failed
	EmptyTelegramUserID             = int64(0) // Default value for empty Telegram user ID
)

// LoginAttempt contains safe metadata for panel login notifications.
// It intentionally does not include attempted passwords.
type LoginAttempt struct {
	Username string
	IP       string
	Time     string
	Status   LoginStatus
	Reason   string
}

// Tgbot provides business logic for Telegram bot integration.
// It handles bot commands, user interactions, and status reporting via Telegram.
type Tgbot struct {
	inboundService service.InboundService
	clientService  service.ClientService
	settingService service.SettingService
	serverService  service.ServerService
	xrayService    service.XrayService
	lastStatus     *service.Status
}

// NewTgbot creates a new Tgbot instance.
func (t *Tgbot) NewTgbot() *Tgbot {
	return new(Tgbot)
}

// I18nBot retrieves a localized message for the bot interface.
func (t *Tgbot) I18nBot(name string, params ...string) string {
	return locale.I18n(locale.Bot, name, params...)
}

// GetHashStorage returns the hash storage instance for callback queries.
func (t *Tgbot) GetHashStorage() *global.HashStorage {
	return hashStorage
}

// getCachedStatus returns cached server status if it's fresh enough (less than 5 seconds old)
func (t *Tgbot) getCachedStatus() (*service.Status, bool) {
	statusCache.mutex.RLock()
	defer statusCache.mutex.RUnlock()

	if statusCache.data != nil && time.Since(statusCache.timestamp) < 5*time.Second {
		return statusCache.data, true
	}
	return nil, false
}

// setCachedStatus updates the status cache
func (t *Tgbot) setCachedStatus(status *service.Status) {
	statusCache.mutex.Lock()
	defer statusCache.mutex.Unlock()

	statusCache.data = status
	statusCache.timestamp = time.Now()
}

// getCachedServerStats returns cached server stats if it's fresh enough (less than 10 seconds old)
func (t *Tgbot) getCachedServerStats() (string, bool) {
	serverStatsCache.mutex.RLock()
	defer serverStatsCache.mutex.RUnlock()

	if serverStatsCache.data != "" && time.Since(serverStatsCache.timestamp) < 10*time.Second {
		return serverStatsCache.data, true
	}
	return "", false
}

// setCachedServerStats updates the server stats cache
func (t *Tgbot) setCachedServerStats(stats string) {
	serverStatsCache.mutex.Lock()
	defer serverStatsCache.mutex.Unlock()

	serverStatsCache.data = stats
	serverStatsCache.timestamp = time.Now()
}

// Start initializes and starts the Telegram bot with the provided translation files.
func (t *Tgbot) Start(i18nFS embed.FS) error {
	// Initialize localizer
	err := locale.InitLocalizer(i18nFS, &t.settingService)
	if err != nil {
		return err
	}

	// If Start is called again (e.g. during reload), ensure any previous long-polling
	// loop is stopped before creating a new bot / receiver.
	StopBot()

	// Initialize hash storage to store callback queries
	hashStorage = global.NewHashStorage(20 * time.Minute)

	// Initialize worker pool for concurrent message processing (max 10 concurrent handlers)
	messageWorkerPool = make(chan struct{}, 10)

	// Initialize optimized HTTP client with connection pooling
	optimizedHTTPClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	t.SetHostname()

	// Get Telegram bot token
	tgBotToken, err := t.settingService.GetTgBotToken()
	if err != nil || tgBotToken == "" {
		logger.Warning("Failed to get Telegram bot token:", err)
		return err
	}

	// Get Telegram bot chat ID(s)
	tgBotID, err := t.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("Failed to get Telegram bot chat ID:", err)
		return err
	}

	parsedAdminIds := make([]int64, 0)
	// Parse admin IDs from comma-separated string
	if tgBotID != "" {
		for adminID := range strings.SplitSeq(tgBotID, ",") {
			id, err := strconv.ParseInt(adminID, 10, 64)
			if err != nil {
				logger.Warning("Failed to parse admin ID from Telegram bot chat ID:", err)
				return err
			}
			parsedAdminIds = append(parsedAdminIds, int64(id))
		}
	}
	tgBotMutex.Lock()
	adminIds = parsedAdminIds
	tgBotMutex.Unlock()

	// Get Telegram bot proxy URL
	tgBotProxy, err := t.settingService.GetTgBotProxy()
	if err != nil {
		logger.Warning("Failed to get Telegram bot proxy URL:", err)
	}

	// Fall back to the panel-wide egress bridge when no dedicated bot proxy is
	// set. Resolved once at bot start: if Xray comes up later, the bot keeps
	// its direct connection until it is restarted.
	if tgBotProxy == "" {
		if egress := t.settingService.PanelEgressProxyURL(); egress != "" && isSupportedBotProxyScheme(egress) {
			tgBotProxy = egress
		}
	}

	// Get Telegram bot API server URL
	tgBotAPIServer, err := t.settingService.GetTgBotAPIServer()
	if err != nil {
		logger.Warning("Failed to get Telegram bot API server URL:", err)
	}

	// Create new Telegram bot instance
	bot, err = t.NewBot(tgBotToken, tgBotProxy, tgBotAPIServer)
	if err != nil {
		logger.Error("Failed to initialize Telegram bot API:", err)
		return err
	}

	t.trySetBotCommands(bot)

	// Start receiving Telegram bot messages
	tgBotMutex.Lock()
	alreadyRunning := isRunning || botCancel != nil
	tgBotMutex.Unlock()
	if !alreadyRunning {
		logger.Info("Telegram bot receiver started")
		go t.OnReceive()
	}

	return nil
}

func (t *Tgbot) trySetBotCommands(bot *telego.Bot) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warning("Failed to register bot commands (Telegram may be rate-limiting); bot will continue without them:", r)
		}
	}()

	err := bot.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
		Commands: []telego.BotCommand{
			{Command: "start", Description: t.I18nBot("tgbot.commands.startDesc")},
			{Command: "help", Description: t.I18nBot("tgbot.commands.helpDesc")},
			{Command: "status", Description: t.I18nBot("tgbot.commands.statusDesc")},
			{Command: "id", Description: t.I18nBot("tgbot.commands.idDesc")},
		},
	})
	if err != nil {
		logger.Warning("Failed to set bot commands:", err)
	}
}

func isSupportedBotProxyScheme(proxyUrl string) bool {
	return strings.HasPrefix(proxyUrl, "socks5://") ||
		strings.HasPrefix(proxyUrl, "http://") ||
		strings.HasPrefix(proxyUrl, "https://")
}

// createRobustFastHTTPClient creates a fasthttp.Client with proper connection handling
func (t *Tgbot) createRobustFastHTTPClient(proxyUrl string) *fasthttp.Client {
	client := &fasthttp.Client{
		// Connection timeouts
		ReadTimeout:                   30 * time.Second,
		WriteTimeout:                  30 * time.Second,
		MaxIdleConnDuration:           60 * time.Second,
		MaxConnDuration:               0, // unlimited, but controlled by MaxIdleConnDuration
		MaxIdemponentCallAttempts:     3,
		ReadBufferSize:                4096,
		WriteBufferSize:               4096,
		MaxConnsPerHost:               100,
		MaxConnWaitTimeout:            10 * time.Second,
		DisableHeaderNamesNormalizing: false,
		DisablePathNormalizing:        false,
		// Retry on connection errors
		RetryIf: func(request *fasthttp.Request) bool {
			// Retry on connection errors for GET requests
			return string(request.Header.Method()) == "GET" || string(request.Header.Method()) == "POST"
		},
	}

	if proxyUrl != "" {
		if strings.HasPrefix(proxyUrl, "socks5://") {
			client.Dial = fasthttpproxy.FasthttpSocksDialer(proxyUrl)
		} else {
			client.Dial = fasthttpproxy.FasthttpHTTPDialer(proxyUrl)
		}
	}

	return client
}

// NewBot creates a new Telegram bot instance with optional proxy and API server settings.
func (t *Tgbot) NewBot(token string, proxyUrl string, apiServerUrl string) (*telego.Bot, error) {
	// Validate proxy URL if provided
	if proxyUrl != "" {
		if !isSupportedBotProxyScheme(proxyUrl) {
			logger.Warning("Unsupported proxy scheme (want socks5:// or http(s)://), ignoring proxy")
			proxyUrl = "" // Clear invalid proxy
		} else if _, err := url.Parse(proxyUrl); err != nil {
			logger.Warningf("Can't parse proxy URL, ignoring proxy: %v", err)
			proxyUrl = ""
		}
	}

	// Validate API server URL if provided
	if apiServerUrl != "" {
		safeURL, err := service.SanitizePublicHTTPURL(apiServerUrl, false)
		if err != nil {
			logger.Warningf("Invalid or blocked API server URL, using default: %v", err)
			apiServerUrl = ""
		} else {
			apiServerUrl = safeURL
		}
	}

	// Create robust fasthttp client
	client := t.createRobustFastHTTPClient(proxyUrl)

	// Build bot options
	var options []telego.BotOption
	options = append(options, telego.WithFastHTTPClient(client))

	if apiServerUrl != "" {
		options = append(options, telego.WithAPIServer(apiServerUrl))
	}

	return telego.NewBot(token, options...)
}

// IsRunning checks if the Telegram bot is currently running.
func (t *Tgbot) IsRunning() bool {
	tgBotMutex.Lock()
	defer tgBotMutex.Unlock()
	return isRunning
}

// SetHostname sets the hostname for the bot.
func (t *Tgbot) SetHostname() {
	host, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		hostname = ""
		return
	}
	hostname = host
}

// Stop safely stops the Telegram bot's Long Polling operation.
// This method now calls the global StopBot function and cleans up other resources.
func (t *Tgbot) Stop() {
	StopBot()
	logger.Info("Stop Telegram receiver ...")
	tgBotMutex.Lock()
	adminIds = nil
	tgBotMutex.Unlock()
}

// StopBot safely stops the Telegram bot's Long Polling operation by cancelling its context.
// This is the global function called from main.go's signal handler and t.Stop().
func StopBot() {
	// Don't hold the mutex while cancelling/waiting.
	tgBotMutex.Lock()
	cancel := botCancel
	botCancel = nil
	handler := botHandler
	botHandler = nil
	isRunning = false
	tgBotMutex.Unlock()

	if handler != nil {
		handler.Stop()
	}

	if cancel != nil {
		logger.Info("Sending cancellation signal to Telegram bot...")
		// Cancels the context passed to UpdatesViaLongPolling; this closes updates channel
		// and lets botHandler.Start() exit cleanly.
		cancel()
		botWG.Wait()
		logger.Info("Telegram bot successfully stopped.")
	}
}

// encodeQuery encodes the query string if it's longer than 64 characters.
func (t *Tgbot) encodeQuery(query string) string {
	// NOTE: we only need to hash for more than 64 chars
	if len(query) <= 64 {
		return query
	}

	return hashStorage.SaveHash(query)
}

// decodeQuery decodes a hashed query string back to its original form.
func (t *Tgbot) decodeQuery(query string) (string, error) {
	if !hashStorage.IsMD5(query) {
		return query, nil
	}

	decoded, exists := hashStorage.GetValue(query)
	if !exists {
		return "", common.NewError("hash not found in storage!")
	}

	return decoded, nil
}

// randomLowerAndNum generates a random string of lowercase letters and numbers.
func (t *Tgbot) randomLowerAndNum(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	for i := range bytes {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		bytes[i] = charset[randomIndex.Int64()]
	}
	return string(bytes)
}

// int64Contains checks if an int64 slice contains a specific item.
func int64Contains(slice []int64, item int64) bool {
	return slices.Contains(slice, item)
}

// isSingleWord checks if the text contains only a single word.
func (t *Tgbot) isSingleWord(text string) bool {
	text = strings.TrimSpace(text)
	re := regexp.MustCompile(`\s+`)
	return re.MatchString(text)
}
