package service

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/web/global"
	"github.com/mhsanaei/3x-ui/v3/web/locale"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/skip2/go-qrcode"
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

	// Performance improvements
	messageWorkerPool   chan struct{} // Semaphore for limiting concurrent message processing
	optimizedHTTPClient *http.Client  // HTTP client with connection pooling and timeouts

	// Simple cache for frequently accessed data
	statusCache struct {
		data      *Status
		timestamp time.Time
		mutex     sync.RWMutex
	}

	serverStatsCache struct {
		data      string
		timestamp time.Time
		mutex     sync.RWMutex
	}

	// clients data to adding new client
	clientStates      map[int64]*ClientState
	clientStatesMutex sync.RWMutex
)


type ClientState struct {
	ReceiverInboundID int
	Id                string
	Flow              string
	Email             string
	LimitIP           int
	TotalGB           int64
	ExpiryTime        int64
	Enable            bool
	TgID              string
	SubID             string
	Comment           string
	Reset             int
	Security          string
	ShPassword        string
	TrPassword        string
	Method            string
}

func (t *Tgbot) getClientState(chatId int64) *ClientState {
	clientStatesMutex.RLock()
	state, exists := clientStates[chatId]
	clientStatesMutex.RUnlock()
	if !exists {
		clientStatesMutex.Lock()
		if clientStates == nil {
			clientStates = make(map[int64]*ClientState)
		}
		state = &ClientState{}
		clientStates[chatId] = state
		clientStatesMutex.Unlock()
	}
	return state
}

func (t *Tgbot) clearClientState(chatId int64) {
	clientStatesMutex.Lock()
	delete(clientStates, chatId)
	clientStatesMutex.Unlock()
}

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
	inboundService InboundService
	settingService SettingService
	serverService  ServerService
	xrayService    XrayService
	lastStatus     *Status
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
func (t *Tgbot) getCachedStatus() (*Status, bool) {
	statusCache.mutex.RLock()
	defer statusCache.mutex.RUnlock()

	if statusCache.data != nil && time.Since(statusCache.timestamp) < 5*time.Second {
		return statusCache.data, true
	}
	return nil, false
}

// setCachedStatus updates the status cache
func (t *Tgbot) setCachedStatus(status *Status) {
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
		for _, adminID := range strings.Split(tgBotID, ",") {
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

	// Set proxy if provided
	if proxyUrl != "" {
		client.Dial = fasthttpproxy.FasthttpSocksDialer(proxyUrl)
	}

	return client
}

// NewBot creates a new Telegram bot instance with optional proxy and API server settings.
func (t *Tgbot) NewBot(token string, proxyUrl string, apiServerUrl string) (*telego.Bot, error) {
	// Validate proxy URL if provided
	if proxyUrl != "" {
		if !strings.HasPrefix(proxyUrl, "socks5://") {
			logger.Warning("Invalid socks5 URL, ignoring proxy")
			proxyUrl = "" // Clear invalid proxy
		} else {
			_, err := url.Parse(proxyUrl)
			if err != nil {
				logger.Warningf("Can't parse proxy URL, ignoring proxy: %v", err)
				proxyUrl = ""
			}
		}
	}

	// Validate API server URL if provided
	if apiServerUrl != "" {
		if !strings.HasPrefix(apiServerUrl, "http") {
			logger.Warning("Invalid http(s) URL for API server, using default")
			apiServerUrl = ""
		} else {
			_, err := url.Parse(apiServerUrl)
			if err != nil {
				logger.Warningf("Can't parse API server URL, using default: %v", err)
				apiServerUrl = ""
			}
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

// OnReceive starts the message receiving loop for the Telegram bot.
func (t *Tgbot) OnReceive() {
	params := telego.GetUpdatesParams{
		Timeout: 20, // Reduced timeout to detect connection issues faster
	}
	// Strict singleton: never start a second long-polling loop.
	tgBotMutex.Lock()
	if botCancel != nil || isRunning {
		tgBotMutex.Unlock()
		logger.Warning("TgBot OnReceive called while already running; ignoring.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	botCancel = cancel
	isRunning = true
	// Add to WaitGroup before releasing the lock so StopBot() can't return
	// before this receiver goroutine is accounted for.
	botWG.Add(1)
	tgBotMutex.Unlock()

	// Get updates channel using the context with shorter timeout for better error recovery
	updates, _ := bot.UpdatesViaLongPolling(ctx, &params)
	go func() {
		defer botWG.Done()
		h, _ := th.NewBotHandler(bot, updates)
		tgBotMutex.Lock()
		botHandler = h
		tgBotMutex.Unlock()

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			delete(userStates, message.Chat.ID)
			t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.keyboardClosed"), tu.ReplyKeyboardRemove())
			return nil
		}, th.TextEqual(t.I18nBot("tgbot.buttons.closeKeyboard")))

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			logger.Debug("Telegram command received:", message.Text)
			// Use goroutine with worker pool for concurrent command processing
			go func() {
				messageWorkerPool <- struct{}{}        // Acquire worker
				defer func() { <-messageWorkerPool }() // Release worker

				delete(userStates, message.Chat.ID)
				t.answerCommand(&message, message.Chat.ID, checkAdmin(message.From.ID))
			}()
			return nil
		}, th.AnyCommand())

		h.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
			logger.Debug("Telegram callback received:", query.Data)
			// Use goroutine with worker pool for concurrent callback processing
			go func() {
				messageWorkerPool <- struct{}{}        // Acquire worker
				defer func() { <-messageWorkerPool }() // Release worker

				delete(userStates, query.Message.GetChat().ID)
				t.answerCallback(&query, checkAdmin(query.From.ID))
			}()
			return nil
		}, th.AnyCallbackQueryWithMessage())

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			logger.Debug("Telegram message received:", message.Text)
			chatId := message.Chat.ID
			state := t.getClientState(chatId)
			if userState, exists := userStates[chatId]; exists {
				switch userState {
				case "awaiting_id":
					if state.Id == strings.TrimSpace(message.Text) {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
						return nil
					}

					state.Id = strings.TrimSpace(message.Text)
					if t.isSingleWord(state.Id) {
						userStates[message.Chat.ID] = "awaiting_id"

						cancel_btn_markup := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
							),
						)

						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.messages.incorrect_input"), cancel_btn_markup)
					} else {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_id"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
					}
				case "awaiting_subid":
					newSubID := strings.TrimSpace(message.Text)

					if state.SubID == newSubID {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
						return nil
					}

					isValidURI, _ := regexp.MatchString(`^[\p{L}\p{N}\-_]+$`, newSubID)

					if !isValidURI {
						userStates[message.Chat.ID] = "awaiting_subid"

						cancel_btn_markup := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
							),
						)

						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.messages.invalid_subid"), cancel_btn_markup)
					} else {
						state.SubID = newSubID
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_subid"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
					}
				case "awaiting_password_tr":
					if state.TrPassword == strings.TrimSpace(message.Text) {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						return nil
					}

					state.TrPassword = strings.TrimSpace(message.Text)
					if t.isSingleWord(state.TrPassword) {
						userStates[message.Chat.ID] = "awaiting_password_tr"

						cancel_btn_markup := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
							),
						)

						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.messages.incorrect_input"), cancel_btn_markup)
					} else {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_password"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
					}
				case "awaiting_password_sh":
					if state.ShPassword == strings.TrimSpace(message.Text) {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						return nil
					}

					state.ShPassword = strings.TrimSpace(message.Text)
					if t.isSingleWord(state.ShPassword) {
						userStates[message.Chat.ID] = "awaiting_password_sh"

						cancel_btn_markup := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
							),
						)

						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.messages.incorrect_input"), cancel_btn_markup)
					} else {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_password"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
					}
				case "awaiting_email":
					if state.Email == strings.TrimSpace(message.Text) {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						return nil
					}

					state.Email = strings.TrimSpace(message.Text)
					if t.isSingleWord(state.Email) {
						userStates[message.Chat.ID] = "awaiting_email"

						cancel_btn_markup := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
							),
						)

						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.messages.incorrect_input"), cancel_btn_markup)
					} else {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_email"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
						message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
						t.addClient(message.Chat.ID, message_text)
					}
				case "awaiting_comment":
					if state.Comment == strings.TrimSpace(message.Text) {
						t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
						delete(userStates, message.Chat.ID)
						return nil
					}

					state.Comment = strings.TrimSpace(message.Text)
					t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_comment"), 3, tu.ReplyKeyboardRemove())
					delete(userStates, message.Chat.ID)
					inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
					message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
					t.addClient(message.Chat.ID, message_text)
				}

			} else {
				if message.UsersShared != nil {
					if checkAdmin(message.From.ID) {
						for _, sharedUser := range message.UsersShared.Users {
							userID := sharedUser.UserID
							needRestart, err := t.inboundService.SetClientTelegramUserID(message.UsersShared.RequestID, userID)
							if needRestart {
								t.xrayService.SetToNeedRestart()
							}
							output := ""
							if err != nil {
								output += t.I18nBot("tgbot.messages.selectUserFailed")
							} else {
								output += t.I18nBot("tgbot.messages.userSaved")
							}
							t.SendMsgToTgbot(message.Chat.ID, output, tu.ReplyKeyboardRemove())
						}
					} else {
						t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.noResult"), tu.ReplyKeyboardRemove())
					}
				}
			}
			return nil
		}, th.AnyMessage())

		h.Start()
	}()
}

func checkAdmin(tgId int64) bool {
	for _, adminId := range adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
}

// SendAnswer sends a response message with an inline keyboard to the specified chat.
func (t *Tgbot) SendAnswer(chatId int64, msg string, isAdmin bool) {
	numericKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.SortedTrafficUsageReport")).WithCallbackData(t.encodeQuery("get_sorted_traffic_usage_report")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.serverUsage")).WithCallbackData(t.encodeQuery("get_usage")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ResetAllTraffics")).WithCallbackData(t.encodeQuery("reset_all_traffics")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.dbBackup")).WithCallbackData(t.encodeQuery("get_backup")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getInbounds")).WithCallbackData(t.encodeQuery("inbounds")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.depleteSoon")).WithCallbackData(t.encodeQuery("deplete_soon")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getBanLogs")).WithCallbackData(t.encodeQuery("get_banlogs")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.allClients")).WithCallbackData(t.encodeQuery("get_inbounds")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.addClient")).WithCallbackData(t.encodeQuery("add_client")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.onlines")).WithCallbackData(t.encodeQuery("onlines")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("commands")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.selectTGUser")).WithCallbackData(t.encodeQuery("admin_client_tg_user_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetTraffic")).WithCallbackData(t.encodeQuery("admin_client_individual_links")),
			tu.InlineKeyboardButton(t.I18nBot("qrCode")).WithCallbackData(t.encodeQuery("admin_client_qr_links")),
		),
	)

	clientKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientUsage")).WithCallbackData(t.encodeQuery("client_traffic")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("client_commands")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.selectTGUser")).WithCallbackData(t.encodeQuery("client_sub_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetTraffic")).WithCallbackData(t.encodeQuery("client_individual_links")),
			tu.InlineKeyboardButton(t.I18nBot("qrCode")).WithCallbackData(t.encodeQuery("client_qr_links")),
		),
	)

	if isAdmin {
		t.SendMsgToTgbot(chatId, msg, numericKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, msg, clientKeyboard)
	}
}

// answerCommand processes incoming command messages from Telegram users.
func (t *Tgbot) answerCommand(message *telego.Message, chatId int64, isAdmin bool) {
	msg, onlyMessage := "", false

	command, _, commandArgs := tu.ParseCommand(message.Text)

	// Helper function to handle unknown commands.
	handleUnknownCommand := func() {
		msg += t.I18nBot("tgbot.commands.unknown")
	}

	// Handle the command.
	switch command {
	case "help":
		msg += t.I18nBot("tgbot.commands.help")
		msg += t.I18nBot("tgbot.commands.pleaseChoose")
	case "start":
		msg += t.I18nBot("tgbot.commands.start", "Firstname=="+html.EscapeString(message.From.FirstName))
		if isAdmin {
			msg += t.I18nBot("tgbot.commands.welcome", "Hostname=="+hostname)
		}
		msg += "\n\n" + t.I18nBot("tgbot.commands.pleaseChoose")
	case "status":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.status")
	case "id":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10))
	case "usage":
		onlyMessage = true
		if len(commandArgs) > 0 {
			if isAdmin {
				t.searchClient(chatId, commandArgs[0])
			} else {
				t.getClientUsage(chatId, int64(message.From.ID), commandArgs[0])
			}
		} else {
			msg += t.I18nBot("tgbot.commands.usage")
		}
	case "inbound":
		onlyMessage = true
		if isAdmin && len(commandArgs) > 0 {
			t.searchInbound(chatId, commandArgs[0])
		} else {
			handleUnknownCommand()
		}
	case "restart":
		onlyMessage = true
		if isAdmin {
			if len(commandArgs) == 0 {
				if t.xrayService.IsXrayRunning() {
					err := t.xrayService.RestartXray(true)
					if err != nil {
						msg += t.I18nBot("tgbot.commands.restartFailed", "Error=="+err.Error())
					} else {
						msg += t.I18nBot("tgbot.commands.restartSuccess")
					}
				} else {
					msg += t.I18nBot("tgbot.commands.xrayNotRunning")
				}
			} else {
				handleUnknownCommand()
				msg += t.I18nBot("tgbot.commands.restartUsage")
			}
		} else {
			handleUnknownCommand()
		}
	default:
		handleUnknownCommand()
	}

	if msg != "" {
		t.sendResponse(chatId, msg, onlyMessage, isAdmin)
	}
}

// sendResponse sends the response message based on the onlyMessage flag.
func (t *Tgbot) sendResponse(chatId int64, msg string, onlyMessage, isAdmin bool) {
	if onlyMessage {
		t.SendMsgToTgbot(chatId, msg)
	} else {
		t.SendAnswer(chatId, msg, isAdmin)
	}
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

// randomShadowSocksPassword generates a random password for Shadowsocks.
func (t *Tgbot) randomShadowSocksPassword() string {
	array := make([]byte, 32)
	_, err := rand.Read(array)
	if err != nil {
		return t.randomLowerAndNum(32)
	}
	return base64.StdEncoding.EncodeToString(array)
}

// answerCallback processes callback queries from inline keyboards.
func (t *Tgbot) answerCallback(callbackQuery *telego.CallbackQuery, isAdmin bool) {
	chatId := callbackQuery.Message.GetChat().ID
	state := t.getClientState(chatId)

	// get query from hash storage
	decodedQuery, err := t.decodeQuery(callbackQuery.Data)
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noQuery"))
			return
		}
	dataArray := strings.Split(decodedQuery, " ")
	cmd := dataArray[0]

	if isAdmin {
		switch cmd {
		case "admin_client_sub_links":
			inbounds, _ := t.inboundService.GetAllInbounds()
			var buttons []telego.InlineKeyboardButton
			for _, inbound := range inbounds {
				buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients_for_sub "+fmt.Sprint(inbound.Id))))
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), tu.InlineKeyboard(tu.InlineKeyboardCols(1, buttons...)...))
			return
		case "admin_client_tg_user_links":
			inbounds, _ := t.inboundService.GetAllInbounds()
			var buttons []telego.InlineKeyboardButton
			for _, inbound := range inbounds {
				buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients_for_tg_user "+fmt.Sprint(inbound.Id))))
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), tu.InlineKeyboard(tu.InlineKeyboardCols(1, buttons...)...))
			return
		case "admin_client_individual_links":
			inbounds, _ := t.inboundService.GetAllInbounds()
			var buttons []telego.InlineKeyboardButton
			for _, inbound := range inbounds {
				buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients_for_individual "+fmt.Sprint(inbound.Id))))
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), tu.InlineKeyboard(tu.InlineKeyboardCols(1, buttons...)...))
			return
		case "admin_client_qr_links":
			inbounds, _ := t.inboundService.GetAllInbounds()
			var buttons []telego.InlineKeyboardButton
			for _, inbound := range inbounds {
				buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients_for_qr "+fmt.Sprint(inbound.Id))))
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), tu.InlineKeyboard(tu.InlineKeyboardCols(1, buttons...)...))
			return
		case "get_inbounds":
			t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.allClients"))
			inbounds, _ := t.inboundService.GetAllInbounds()
			var buttons []telego.InlineKeyboardButton
			for _, inbound := range inbounds {
				buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients "+fmt.Sprint(inbound.Id))))
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), tu.InlineKeyboard(tu.InlineKeyboardCols(1, buttons...)...))
			return
		case "get_sorted_traffic_usage_report":
			t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
			emails, err := t.inboundService.getAllEmails()
			if err != nil {
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"), tu.ReplyKeyboardRemove())
				return
			}
			traffics := make([]*xray.ClientTraffic, 0, len(emails))
			for _, email := range emails {
				traffic, err := t.inboundService.GetClientTrafficByEmail(email)
				if err == nil && traffic != nil {
					traffics = append(traffics, traffic)
				}
			}
			sort.Slice(traffics, func(i, j int) bool {
				return traffics[i].Up+traffics[i].Down > traffics[j].Up+traffics[j].Down
			})
			var output strings.Builder
			output.WriteString("📊 " + t.I18nBot("tgbot.buttons.SortedTrafficUsageReport") + ":\n\n")
			for i, traffic := range traffics {
				if i >= 50 {
					break
				}
				output.WriteString(t.clientInfoMsg(traffic, true, false, false, false, false, false))
				output.WriteString("\n")
			}
			t.SendMsgToTgbot(chatId, output.String())
			return
		case "reset_all_traffics":
			inlineKeyboard := tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("reset_all_traffics_cancel")),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmResetTraffic")).WithCallbackData(t.encodeQuery("reset_all_traffics_c")),
				),
			)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.AreYouSure"), inlineKeyboard)
			return
		case "reset_all_traffics_c":
			t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
			emails, err := t.inboundService.getAllEmails()
			if err != nil {
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"), tu.ReplyKeyboardRemove())
				return
			}
			for _, email := range emails {
				_ = t.inboundService.ResetClientTrafficByEmail(email)
			}
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.FinishProcess"), tu.ReplyKeyboardRemove())
			return
		case "reset_all_traffics_cancel":
			t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
			t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.cancel"), 1, tu.ReplyKeyboardRemove())
			return
		}

		if len(dataArray) >= 2 && len(dataArray[1]) > 0 {
			email := dataArray[1]
			switch dataArray[0] {
			case "get_clients_for_sub":
				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				clientsKB, err := t.getInboundClientsFor(inboundIdInt, "client_sub_links")
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				inbound, _ := t.inboundService.GetInbound(inboundIdInt)
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clientsKB)
			case "get_clients_for_tg_user":
				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				clientsKB, err := t.getInboundClientsFor(inboundIdInt, "tg_user")
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				inbound, _ := t.inboundService.GetInbound(inboundIdInt)
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clientsKB)
			case "get_clients_for_individual":
				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				clientsKB, err := t.getInboundClientsFor(inboundIdInt, "client_individual_links")
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				inbound, _ := t.inboundService.GetInbound(inboundIdInt)
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clientsKB)
			case "get_clients_for_qr":
				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				clientsKB, err := t.getInboundClientsFor(inboundIdInt, "client_qr_links")
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				inbound, _ := t.inboundService.GetInbound(inboundIdInt)
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clientsKB)
			case "client_sub_links":
				t.sendClientSubLinks(chatId, email)
				return
			case "client_individual_links":
				t.sendClientIndividualLinks(chatId, email)
				return
			case "client_qr_links":
				t.sendClientQRLinks(chatId, email)
				return
			case "client_get_usage":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.messages.email", "Email=="+email))
				t.searchClient(chatId, email)
			case "client_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clientRefreshSuccess", "Email=="+email))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "client_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "ips_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.IpRefreshSuccess", "Email=="+email))
				t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
			case "ips_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
			case "tgid_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.TGIdRefreshSuccess", "Email=="+email))
				t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
			case "tgid_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
			case "reset_traffic":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmResetTraffic")).WithCallbackData(t.encodeQuery("reset_traffic_c "+email)),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "reset_traffic_c":
				err := t.inboundService.ResetClientTrafficByEmail(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetTrafficSuccess", "Email=="+email))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				}
			case "limit_traffic":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 1")),
						tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 5")),
						tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 10")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 20")),
						tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 30")),
						tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 40")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 50")),
						tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 60")),
						tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 80")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 100")),
						tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 150")),
						tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 200")),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "limit_traffic_c":
				if len(dataArray) == 3 {
					limitTraffic, err := strconv.Atoi(dataArray[2])
					if err == nil {
						needRestart, err := t.inboundService.ResetClientTrafficLimitByEmail(email, limitTraffic)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.setTrafficLimitSuccess", "Email=="+email))
							t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "limit_traffic_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "add_client_limit_traffic_c":
				limitTraffic, _ := strconv.ParseInt(dataArray[1], 10, 64)
				state.TotalGB = limitTraffic * 1024 * 1024 * 1024
				messageId := callbackQuery.Message.GetMessageID()
				inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}

				t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_limit_traffic_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_limit_traffic_c "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
			case "reset_exp":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("reset_exp_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 7")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 10")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 14")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 20")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 30")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 90")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 180")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 365")),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "reset_exp_c":
				if len(dataArray) == 3 {
					days, err := strconv.ParseInt(dataArray[2], 10, 64)
					if err == nil {
						var date int64
						if days > 0 {
							traffic, err := t.inboundService.GetClientTrafficByEmail(email)
							if err != nil {
								logger.Warning(err)
								msg := t.I18nBot("tgbot.wentWrong")
								t.SendMsgToTgbot(chatId, msg)
								return
							}
							if traffic == nil {
								msg := t.I18nBot("tgbot.noResult")
								t.SendMsgToTgbot(chatId, msg)
								return
							}

							if traffic.ExpiryTime > 0 {
								if traffic.ExpiryTime-time.Now().Unix()*1000 < 0 {
									date = -int64(days * 24 * 60 * 60000)
								} else {
									date = traffic.ExpiryTime + int64(days*24*60*60000)
								}
							} else {
								date = traffic.ExpiryTime - int64(days*24*60*60000)
							}

						}
						needRestart, err := t.inboundService.ResetClientExpiryTimeByEmail(email, date)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.expireResetSuccess", "Email=="+email))
							t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "reset_exp_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "add_client_reset_exp_c":
				state.ExpiryTime = 0
				days, _ := strconv.ParseInt(dataArray[1], 10, 64)
				var date int64
				if state.ExpiryTime > 0 {
					if state.ExpiryTime-time.Now().Unix()*1000 < 0 {
						date = -int64(days * 24 * 60 * 60000)
					} else {
						date = state.ExpiryTime + int64(days*24*60*60000)
					}
				} else {
					date = state.ExpiryTime - int64(days*24*60*60000)
				}
				state.ExpiryTime = date

				messageId := callbackQuery.Message.GetMessageID()
				inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}

				t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_reset_exp_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_reset_exp_c "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
			case "ip_limit":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelIpLimit")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("ip_limit_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 1")),
						tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 2")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 3")),
						tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 4")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 5")),
						tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 6")),
						tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 7")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 8")),
						tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 9")),
						tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 10")),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "ip_limit_c":
				if len(dataArray) == 3 {
					count, err := strconv.Atoi(dataArray[2])
					if err == nil {
						needRestart, err := t.inboundService.ResetClientIpLimitByEmail(email, count)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetIpSuccess", "Email=="+email, "Count=="+strconv.Itoa(count)))
							t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "ip_limit_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("ip_limit_c "+email+" "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
			case "add_client_ip_limit_c":
				if len(dataArray) == 2 {
					count, _ := strconv.Atoi(dataArray[1])
					state.LimitIP = count
				}

				messageId := callbackQuery.Message.GetMessageID()
				inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}

				t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_ip_limit_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								switch num {
								case -2:
									inputNumber = 0
								case -1:
									if inputNumber > 0 {
										inputNumber = (inputNumber / 10)
									}
								default:
									inputNumber = (inputNumber * 10) + num
								}
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := tu.InlineKeyboard(
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_ip_limit")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_ip_limit_c "+strconv.Itoa(inputNumber))),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 1")),
								tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 2")),
								tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 3")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 4")),
								tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 5")),
								tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 6")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 7")),
								tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 8")),
								tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 9")),
							),
							tu.InlineKeyboardRow(
								tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" -2")),
								tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 0")),
								tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" -1")),
							),
						)
						t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
						return
					}
				}
			case "clear_ips":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("ips_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmClearIps")).WithCallbackData(t.encodeQuery("clear_ips_c "+email)),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "clear_ips_c":
				err := t.inboundService.ClearClientIps(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clearIpSuccess", "Email=="+email))
					t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				}
			case "ip_log":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getIpLog", "Email=="+email))
				t.searchClientIps(chatId, email)
			case "tg_user":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getUserInfo", "Email=="+email))
				t.clientTelegramUserInfo(chatId, email)
			case "tgid_remove":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("tgid_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmRemoveTGUser")).WithCallbackData(t.encodeQuery("tgid_remove_c "+email)),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "tgid_remove_c":
				traffic, err := t.inboundService.GetClientTrafficByEmail(email)
				if err != nil || traffic == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					return
				}
				needRestart, err := t.inboundService.SetClientTelegramUserID(traffic.Id, EmptyTelegramUserID)
				if needRestart {
					t.xrayService.SetToNeedRestart()
				}
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.removedTGUserSuccess", "Email=="+email))
					t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				}
			case "toggle_enable":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmToggle")).WithCallbackData(t.encodeQuery("toggle_enable_c "+email)),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
			case "toggle_enable_c":
				enabled, needRestart, err := t.inboundService.ToggleClientEnableByEmail(email)
				if needRestart {
					t.xrayService.SetToNeedRestart()
				}
				if err == nil {
					if enabled {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.enableSuccess", "Email=="+email))
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.disableSuccess", "Email=="+email))
					}
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				}
			case "get_clients":
				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				inbound, err := t.inboundService.GetInbound(inboundIdInt)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				clients, err := t.getInboundClients(inboundIdInt)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clients)
			case "add_client_to":
				// assign default values to clients variables
				state.Id = uuid.New().String()
				state.Flow = ""
				state.Email = t.randomLowerAndNum(8)
				state.LimitIP = 0
				state.TotalGB = 0
				state.ExpiryTime = 0
				state.Enable = true
				state.TgID = ""
				state.SubID = t.randomLowerAndNum(16)
				state.Comment = ""
				state.Reset = 0
				state.Security = "auto"
				state.ShPassword = t.randomShadowSocksPassword()
				state.TrPassword = t.randomLowerAndNum(10)
				state.Method = ""

				inboundId := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundId)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				state.ReceiverInboundID = inboundIdInt
				inbound, err := t.inboundService.GetInbound(inboundIdInt)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}

				message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}

				t.addClient(callbackQuery.Message.GetChat().ID, message_text)
			}
		}
	}

	switch cmd {
	case "get_usage":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.serverUsage"))
		t.getServerUsage(chatId)
	case "usage_refresh":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
		t.getServerUsage(chatId, callbackQuery.Message.GetMessageID())
	case "inbounds":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.getInbounds"))
		t.SendMsgToTgbot(chatId, t.getInboundUsages())
	case "deplete_soon":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.depleteSoon"))
		t.getExhausted(chatId)
	case "get_backup":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.dbBackup"))
		t.sendBackup(chatId)
	case "get_banlogs":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.getBanLogs"))
		t.sendBanLogs(chatId, true)
	case "client_traffic":
		tgUserID := callbackQuery.From.ID
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.clientUsage"))
		t.getClientUsage(chatId, tgUserID)
	case "client_commands":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.commands"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.helpClientCommands"))
	case "client_sub_links":
		// show user's own clients to choose one for sub links
		tgUserID := callbackQuery.From.ID
		traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
		if err != nil {
			// fallback to message
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
			return
		}
		if len(traffics) == 0 {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(tgUserID, 10)))
			return
		}
		var buttons []telego.InlineKeyboardButton
		for _, tr := range traffics {
			buttons = append(buttons, tu.InlineKeyboardButton(tr.Email).WithCallbackData(t.encodeQuery("client_sub_links "+tr.Email)))
		}
		cols := 1
		if len(buttons) >= 6 {
			cols = 2
		}
		keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), keyboard)
	case "client_individual_links":
		// show user's clients to choose for individual links
		tgUserID := callbackQuery.From.ID
		traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
			return
		}
		if len(traffics) == 0 {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(tgUserID, 10)))
			return
		}
		var buttons2 []telego.InlineKeyboardButton
		for _, tr := range traffics {
			buttons2 = append(buttons2, tu.InlineKeyboardButton(tr.Email).WithCallbackData(t.encodeQuery("client_individual_links "+tr.Email)))
		}
		cols2 := 1
		if len(buttons2) >= 6 {
			cols2 = 2
		}
		keyboard2 := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols2, buttons2...))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), keyboard2)
	case "client_qr_links":
		// show user's clients to choose for QR codes
		tgUserID := callbackQuery.From.ID
		traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
			return
		}
		if len(traffics) == 0 {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(tgUserID, 10)))
			return
		}
		var buttons3 []telego.InlineKeyboardButton
		for _, tr := range traffics {
			buttons3 = append(buttons3, tu.InlineKeyboardButton(tr.Email).WithCallbackData(t.encodeQuery("client_qr_links "+tr.Email)))
		}
		cols3 := 1
		if len(buttons3) >= 6 {
			cols3 = 2
		}
		keyboard3 := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols3, buttons3...))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), keyboard3)
	case "onlines":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.onlines"))
		t.onlineClients(chatId)
	case "onlines_refresh":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
		t.onlineClients(chatId, callbackQuery.Message.GetMessageID())
	case "commands":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.commands"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.helpAdminCommands"))
	case "add_client":
		// assign default values to clients variables
		state.Id = uuid.New().String()
		state.Flow = ""
		state.Email = t.randomLowerAndNum(8)
		state.LimitIP = 0
		state.TotalGB = 0
		state.ExpiryTime = 0
		state.Enable = true
		state.TgID = ""
		state.SubID = t.randomLowerAndNum(16)
		state.Comment = ""
		state.Reset = 0
		state.Security = "auto"
		state.ShPassword = t.randomShadowSocksPassword()
		state.TrPassword = t.randomLowerAndNum(10)
		state.Method = ""

		inbounds, err := t.getInboundsAddClient()
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.addClient"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), inbounds)
	case "add_client_ch_default_email":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_email"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.email_prompt", "ClientEmail=="+state.Email)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_subid":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_subid"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.subid_prompt", "ClientSubId=="+state.SubID)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)

	case "add_client_ch_default_flow":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("xtls-rprx-vision").WithCallbackData("set_flow_vision"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("xtls-rprx-vision_udp443").WithCallbackData("set_flow_vision_udp443"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.flow_none")).WithCallbackData("set_flow_none"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("add_client_default_info"),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)

	case "set_flow_vision":
		state.Flow = "xtls-rprx-vision"
		messageId := callbackQuery.Message.GetMessageID()
		inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
		message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		t.addClient(chatId, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))

	case "set_flow_vision_udp443":
		state.Flow = "xtls-rprx-vision-udp443"
		messageId := callbackQuery.Message.GetMessageID()
		inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
		message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		t.addClient(chatId, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))

	case "set_flow_none":
		state.Flow = ""
		messageId := callbackQuery.Message.GetMessageID()
		inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
		message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		t.addClient(chatId, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
	case "add_client_ch_default_id":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_id"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.id_prompt", "ClientId=="+state.Id)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_pass_tr":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_password_tr"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.pass_prompt", "ClientPassword=="+state.TrPassword)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_pass_sh":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_password_sh"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.pass_prompt", "ClientPassword=="+state.ShPassword)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_comment":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_comment"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.comment_prompt", "ClientComment=="+state.Comment)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_traffic":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 1")),
				tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 5")),
				tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 20")),
				tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 30")),
				tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 40")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 50")),
				tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 60")),
				tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 80")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 100")),
				tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 150")),
				tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 200")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_ch_default_exp":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_reset_exp_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 7")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 14")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 20")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 30")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 90")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 180")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 365")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_ch_default_ip_limit":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_ip_limit")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_ip_limit_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_ip_limit_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 1")),
				tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 2")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 3")),
				tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 4")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 5")),
				tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 6")),
				tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 7")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 8")),
				tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 9")),
				tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 10")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_default_info":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
		delete(userStates, chatId)
		inbound, _ := t.inboundService.GetInbound(state.ReceiverInboundID)
		message_text, _ := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		t.addClient(chatId, message_text)
	case "add_client_cancel":
		delete(userStates, chatId)
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.cancel"), 3, tu.ReplyKeyboardRemove())
	case "add_client_default_traffic_exp":
		messageId := callbackQuery.Message.GetMessageID()
		inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}

		t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
	case "add_client_default_ip_limit":
		messageId := callbackQuery.Message.GetMessageID()
		inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		message_text, err := t.BuildInboundClientDataMessage(chatId, inbound.Remark, inbound.Protocol)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}

		t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
	case "add_client_submit_enable":
		state.Enable = true
		t.submitAddClient(callbackQuery)
	case "add_client_submit_disable":
		state.Enable = false
		t.submitAddClient(callbackQuery)
	}
}
func (t *Tgbot) submitAddClient(callbackQuery *telego.CallbackQuery) {
	chatId := callbackQuery.Message.GetChat().ID
	needRestart, err := t.SubmitAddClient(chatId)
	if err == nil {
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.answers.successfulOperation"), 3, tu.ReplyKeyboardRemove())
		if needRestart {
			t.xrayService.SetToNeedRestart()
		}
	} else {
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
		errMsg := ""
		if err != nil {
			errMsg = "\r\n" + err.Error()
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+errMsg)
	}
}

// getCommonClientButtons returns the shared inline keyboard rows for client configuration
func (t *Tgbot) getCommonClientButtons() [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_subid")).WithCallbackData("add_client_ch_default_subid"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData("add_client_ch_default_traffic"),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData("add_client_ch_default_exp"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_comment")).WithCallbackData("add_client_ch_default_comment"),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ipLimit")).WithCallbackData("add_client_ch_default_ip_limit"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitDisable")).WithCallbackData("add_client_submit_disable"),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitEnable")).WithCallbackData("add_client_submit_enable"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("add_client_cancel"),
		),
	}
}

// addClient handles the process of adding a new client to an inbound.
func (t *Tgbot) addClient(chatId int64, msg string, messageID ...int) {
	state := t.getClientState(chatId)
	inbound, err := t.inboundService.GetInbound(state.ReceiverInboundID)
	if err != nil {
		t.SendMsgToTgbot(chatId, err.Error())
		return
	}

	protocol := inbound.Protocol

	var protocolRows [][]telego.InlineKeyboardButton
	switch protocol {
	case model.VMESS, model.VLESS:
		protocolRows = [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_id")).WithCallbackData("add_client_ch_default_id"),
			),
		}
		if protocol == model.VLESS {
			canUseFlow := false
			var streamSettings map[string]interface{}
			if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err == nil {
				network, _ := streamSettings["network"].(string)
				security, _ := streamSettings["security"].(string)

				// Strict: only TCP and only with TLS or REALITY
				if network == "tcp" && (security == "tls" || security == "reality") {
					canUseFlow = true
				}
			}
			if canUseFlow {
				protocolRows = append(protocolRows, tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_flow")).WithCallbackData("add_client_ch_default_flow"),
				))
			} else {
				state.Flow = ""
			}
		}
	case model.Trojan:
		protocolRows = [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_password")).WithCallbackData("add_client_ch_default_pass_tr"),
			),
		}
	case model.Shadowsocks:
		protocolRows = [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_password")).WithCallbackData("add_client_ch_default_pass_sh"),
			),
		}
	}

	commonRows := t.getCommonClientButtons()
	inlineKeyboard := tu.InlineKeyboard(append(protocolRows, commonRows...)...)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], msg, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
	}
}

// searchInbound searches for inbounds by remark and sends the results.
func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbounds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(inbounds) == 0 {
		msg := t.I18nBot("tgbot.noInbounds")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbounds {
		info := ""
		info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
		info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
		info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

		if inbound.ExpiryTime == 0 {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
		} else {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		if len(inbound.ClientStats) > 0 {
			var output strings.Builder
			for _, traffic := range inbound.ClientStats {
				output.WriteString(t.clientInfoMsg(&traffic, true, true, true, true, true, true))
			}
			t.SendMsgToTgbot(chatId, output.String())
		}
	}
}

// getExhausted retrieves and sends information about exhausted clients.
func (t *Tgbot) getExhausted(chatId int64) {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	var exhaustedInbounds []model.Inbound
	var exhaustedClients []xray.ClientTraffic
	var disabledInbounds []model.Inbound
	var disabledClients []xray.ClientTraffic

	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		exDiff = int64(ExpireThreshold) * 86400000
	}

	inbounds, _ := t.inboundService.GetAllInbounds()
	for _, inbound := range inbounds {
		if !inbound.Enable {
			disabledInbounds = append(disabledInbounds, *inbound)
			continue
		}
		if (inbound.Total > 0 && inbound.Total-(inbound.Up+inbound.Down) < trDiff) || (inbound.ExpiryTime > 0 && inbound.ExpiryTime-now < exDiff) {
			exhaustedInbounds = append(exhaustedInbounds, *inbound)
		}

		clients, _ := t.inboundService.GetClients(inbound)
		for _, client := range clients {
			if !client.Enable {
				disabledClients = append(disabledClients, xray.ClientTraffic{Email: client.Email})
				continue
			}
		}
	}

	msg := ""
	if len(exhaustedInbounds) > 0 {
		msg += "⚠️ " + t.I18nBot("tgbot.messages.exhaustedInbounds") + ":\n"
		for _, inbound := range exhaustedInbounds {
			msg += fmt.Sprintf("- %s (Port: %d)\n", inbound.Remark, inbound.Port)
		}
		msg += "\n"
	}
	if len(exhaustedClients) > 0 {
		msg += "⚠️ " + t.I18nBot("tgbot.messages.exhaustedClients") + ":\n"
		for _, client := range exhaustedClients {
			msg += fmt.Sprintf("- %s\n", client.Email)
		}
		msg += "\n"
	}
	if len(disabledInbounds) > 0 {
		msg += "🚫 " + t.I18nBot("tgbot.messages.disabledInbounds") + ":\n"
		for _, inbound := range disabledInbounds {
			msg += fmt.Sprintf("- %s (Port: %d)\n", inbound.Remark, inbound.Port)
		}
		msg += "\n"
	}
	if len(disabledClients) > 0 {
		msg += "🚫 " + t.I18nBot("tgbot.messages.disabledClients") + ":\n"
		for _, client := range disabledClients {
			msg += fmt.Sprintf("- %s\n", client.Email)
		}
	}

	if msg == "" {
		msg = t.I18nBot("tgbot.messages.noExhausted")
	}

	t.SendMsgToTgbot(chatId, msg)
}

// searchClient searches for a client by email and sends its status.
func (t *Tgbot) searchClient(chatId int64, email string, messageID ...int) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := t.I18nBot("tgbot.noResult")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	output := t.clientInfoMsg(traffic, true, true, true, true, true, len(messageID) > 0)

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("client_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData(t.encodeQuery("limit_traffic "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData(t.encodeQuery("reset_exp "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetTraffic")).WithCallbackData(t.encodeQuery("reset_traffic "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ipLimit")).WithCallbackData(t.encodeQuery("ip_limit "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.setTGUser")).WithCallbackData(t.encodeQuery("tg_user "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.toggle")).WithCallbackData(t.encodeQuery("toggle_enable "+email)),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

// getInboundsFor creates an inline keyboard with inbounds for a specific action.
func (t *Tgbot) getInboundsFor(action string) (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery(action+" "+strconv.Itoa(inbound.Id))))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

// getInboundClientsFor creates an inline keyboard with clients of a specific inbound for a specific action.
func (t *Tgbot) getInboundClientsFor(id int, action string) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		return nil, err
	}
	clients, err := t.inboundService.GetClients(inbound)
	if err != nil {
		return nil, err
	}

	var buttons []telego.InlineKeyboardButton
	for _, client := range clients {
		buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery(action+" "+client.Email)))
	}

	cols := 2
	if len(buttons) >= 10 {
		cols = 3
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

// sendClientSubLinks sends subscription links for a client.
func (t *Tgbot) sendClientSubLinks(chatId int64, email string) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	subUrl, err := t.settingService.GetSubURI()
	if err != nil || subUrl == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.subUrlNotSet"))
		return
	}

	msg := t.I18nBot("tgbot.messages.subLinks", "Email=="+email)
	msg += fmt.Sprintf("\n`%s%s`", subUrl, traffic.SubId)
	t.SendMsgToTgbot(chatId, msg)
}

// sendClientIndividualLinks sends individual xray links for a client.
func (t *Tgbot) sendClientIndividualLinks(chatId int64, email string) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	host, _ := t.settingService.GetWebDomain()
	links, err := t.inboundService.GetClientLinks(host, traffic.InboundId, email)
	if err != nil || len(links) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	msg := t.I18nBot("tgbot.messages.individualLinks", "Email=="+email)
	for _, link := range links {
		msg += fmt.Sprintf("\n`%s`", link)
	}
	t.SendMsgToTgbot(chatId, msg)
}

// sendClientQRLinks sends QR codes for a client's links.
func (t *Tgbot) sendClientQRLinks(chatId int64, email string) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		return
	}

	host, _ := t.settingService.GetWebDomain()
	links, err := t.inboundService.GetClientLinks(host, traffic.InboundId, email)
	if err != nil || len(links) == 0 {
		return
	}

	for _, link := range links {
		qr, err := qrcode.Encode(link, qrcode.Medium, 256)
		if err != nil {
			continue
		}
		t.SendPhotoToTgbot(chatId, qr, link)
	}
}

// onlineClients retrieves and sends information about currently online clients.
func (t *Tgbot) onlineClients(chatId int64, messageID ...int) {
	if !p.IsRunning() {
		return
	}

	onlines := p.GetOnlineClients()
	onlinesCount := len(onlines)
	output := t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(onlinesCount))
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("onlines_refresh"))))

	if onlinesCount > 0 {
		var buttons []telego.InlineKeyboardButton
		for _, online := range onlines {
			buttons = append(buttons, tu.InlineKeyboardButton(online).WithCallbackData(t.encodeQuery("client_get_usage "+online)))
		}
		cols := 2
		if onlinesCount >= 10 {
			cols = 3
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tu.InlineKeyboardCols(cols, buttons...)...)
	}

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, keyboard)
	}
}

// getServerUsage retrieves and sends the server's usage statistics.
func (t *Tgbot) getServerUsage(chatId int64, messageID ...int) {
	status, exists := t.getCachedStatus()
	if !exists {
		status = t.serverService.GetStatus(t.lastStatus)
		t.setCachedStatus(status)
	}

	msg := "📊 " + t.I18nBot("tgbot.buttons.serverUsage") + ":\n"
	msg += t.I18nBot("tgbot.messages.cpu", "Usage=="+fmt.Sprintf("%.2f", status.Cpu))
	msg += t.I18nBot("tgbot.messages.mem", "Usage=="+common.FormatTraffic(int64(status.Mem.Current)), "Total=="+common.FormatTraffic(int64(status.Mem.Total)))
	msg += t.I18nBot("tgbot.messages.swap", "Usage=="+common.FormatTraffic(int64(status.Swap.Current)), "Total=="+common.FormatTraffic(int64(status.Swap.Total)))
	msg += t.I18nBot("tgbot.messages.disk", "Usage=="+common.FormatTraffic(int64(status.Disk.Current)), "Total=="+common.FormatTraffic(int64(status.Disk.Total)))
	msg += t.I18nBot("tgbot.messages.uptime", "Time=="+strconv.FormatUint(status.Uptime/86400, 10))

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("usage_refresh")),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], msg, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
	}
}

// getInboundUsages retrieves and sends the usage statistics for all inbounds.
func (t *Tgbot) getInboundUsages() string {
	inbounds, _ := t.inboundService.GetAllInbounds()
	var msg strings.Builder
	msg.WriteString("📥 " + t.I18nBot("tgbot.buttons.getInbounds") + ":\n")
	for _, inbound := range inbounds {
		msg.WriteString(fmt.Sprintf("- %s: %s (Port: %d)\n", inbound.Remark, common.FormatTraffic(inbound.Up+inbound.Down), inbound.Port))
	}
	return msg.String()
}

// getClientUsage retrieves and sends the usage statistics for a client by its Telegram ID or email.
func (t *Tgbot) getClientUsage(chatId int64, tgUserID int64, email ...string) {
	var traffics []*xray.ClientTraffic
	var err error

	if len(email) > 0 {
		traffic, err := t.inboundService.GetClientTrafficByEmail(email[0])
		if err == nil && traffic != nil {
			traffics = append(traffics, traffic)
		}
	} else {
		traffics, err = t.inboundService.GetClientTrafficTgBot(tgUserID)
	}

	if err != nil || len(traffics) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	for _, traffic := range traffics {
		t.SendMsgToTgbot(chatId, t.clientInfoMsg(traffic, true, true, true, true, true, false))
	}
}

// searchClientIps retrieves and sends the IP log for a client by email.
func (t *Tgbot) searchClientIps(chatId int64, email string, messageID ...int) {
	ips, err := t.inboundService.GetInboundClientIps(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.wentWrong"))
		return
	}

	output := t.I18nBot("tgbot.messages.email", "Email=="+email) + "\n" + ips

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("ips_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clearIPs")).WithCallbackData(t.encodeQuery("clear_ips "+email)),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

// clientTelegramUserInfo retrieves and sends Telegram user info for a client.
func (t *Tgbot) clientTelegramUserInfo(chatId int64, email string, messageID ...int) {
	traffic, client, err := t.inboundService.GetClientByEmail(email)
	if err != nil || client == nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	tgId := "None"
	if client.TgID != 0 {
		tgId = strconv.FormatInt(client.TgID, 10)
	}

	output := t.I18nBot("tgbot.messages.email", "Email=="+email) + "\nTelegram ID: " + tgId

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("tgid_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.removeTGUser")).WithCallbackData(t.encodeQuery("tgid_remove "+email)),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}

	requestUsers := &telego.KeyboardButtonRequestUsers{
		RequestID: int32(traffic.Id),
		UserIsBot: new(bool), // false
	}
	keyboard := tu.Keyboard(
		tu.KeyboardRow(
			tu.KeyboardButton(t.I18nBot("tgbot.buttons.selectTGUser")).WithRequestUsers(requestUsers),
		),
		tu.KeyboardRow(
			tu.KeyboardButton(t.I18nBot("tgbot.buttons.closeKeyboard")),
		),
	).WithIsPersistent().WithResizeKeyboard()
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.buttons.selectOneTGUser"), keyboard)
}

// sendBackup sends a database backup file to the specified chat.
func (t *Tgbot) sendBackup(chatId int64) {
	dbPath := config.GetDBPath()
	file, err := os.Open(dbPath)
	if err == nil {
		defer file.Close()
		doc := tu.Document(tu.ID(chatId), tu.FileFromReader(file, "x-ui.db"))
		_, _ = bot.SendDocument(context.Background(), doc)
	}
}

// sendBanLogs sends the ban logs to the specified chat.
func (t *Tgbot) sendBanLogs(chatId int64, dt bool) {
	file, err := os.Open(xray.GetIPLimitBannedLogPath())
	if err == nil {
		defer file.Close()
		stat, _ := file.Stat()
		if stat.Size() > 0 {
			doc := tu.Document(tu.ID(chatId), tu.FileFromReader(file, "ban.log"))
			_, _ = bot.SendDocument(context.Background(), doc)
			return
		}
	}
	t.SendMsgToTgbot(chatId, "🚫 "+t.I18nBot("tgbot.noResult"))
}

// SendPhotoToTgbot sends a photo to the Telegram bot.
func (t *Tgbot) SendPhotoToTgbot(chatId int64, photo []byte, caption string) {
	reader := strings.NewReader(string(photo))
	photoMsg := tu.Photo(tu.ID(chatId), tu.FileFromReader(reader, "qr.png"))
	photoMsg.Caption = caption
	photoMsg.ParseMode = "HTML"
	_, _ = bot.SendPhoto(context.Background(), photoMsg)
}

// sendCallbackAnswerTgBot sends an answer to a callback query.
func (t *Tgbot) sendCallbackAnswerTgBot(callbackQueryID string, text string) {
	_ = bot.AnswerCallbackQuery(context.Background(), &telego.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	})
}

// editMessageTgBot edits an existing message in the Telegram bot.
func (t *Tgbot) editMessageTgBot(chatId int64, messageID int, text string, replyMarkup ...*telego.InlineKeyboardMarkup) {
	editMsg := &telego.EditMessageTextParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
	}
	if len(replyMarkup) > 0 {
		editMsg.ReplyMarkup = replyMarkup[0]
	}
	_, _ = bot.EditMessageText(context.Background(), editMsg)
}

// editMessageCallbackTgBot edits a message's reply markup based on a callback query.
func (t *Tgbot) editMessageCallbackTgBot(chatId int64, messageID int, replyMarkup *telego.InlineKeyboardMarkup) {
	editMsg := &telego.EditMessageReplyMarkupParams{
		ChatID:      tu.ID(chatId),
		MessageID:   messageID,
		ReplyMarkup: replyMarkup,
	}
	_, _ = bot.EditMessageReplyMarkup(context.Background(), editMsg)
}

// deleteMessageTgBot deletes a message in the Telegram bot.
func (t *Tgbot) deleteMessageTgBot(chatId int64, messageID int) {
	_ = bot.DeleteMessage(context.Background(), &telego.DeleteMessageParams{ChatID: tu.ID(chatId), MessageID: messageID})
}

// SendMsgToTgbotDeleteAfter sends a message and deletes it after a specified time.
func (t *Tgbot) SendMsgToTgbotDeleteAfter(chatId int64, msg string, seconds int, replyMarkup ...telego.ReplyMarkup) {
	var replyMarkupParam telego.ReplyMarkup
	if len(replyMarkup) > 0 {
		replyMarkupParam = replyMarkup[0]
	}

	sentMsg, err := bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:    tu.ID(chatId),
		Text:      msg,
		ParseMode: "HTML",
		ReplyMarkup: replyMarkupParam,
	})
	if err == nil {
		go func() {
			time.Sleep(time.Duration(seconds) * time.Second)
			t.deleteMessageTgBot(chatId, sentMsg.MessageID)
			delete(userStates, chatId)
		}()
	}
}

// isSingleWord checks if a string consists of a single word.
func (t *Tgbot) isSingleWord(s string) bool {
	return len(strings.Fields(s)) > 1
}

// BuildInboundClientDataMessage builds a message with the current client configuration.
func (t *Tgbot) BuildInboundClientDataMessage(chatId int64, remark string, protocol model.Protocol) (string, error) {
	state := t.getClientState(chatId)
	msg := fmt.Sprintf("📝 *%s* (%s)\n\n", remark, protocol)
	msg += fmt.Sprintf("📧 %s: `%s`\n", t.I18nBot("pages.inbounds.email"), state.Email)

	switch protocol {
	case model.VMESS, model.VLESS:
		msg += fmt.Sprintf("🆔 %s: `%s`\n", "ID", state.Id)
		if protocol == model.VLESS && state.Flow != "" {
			msg += fmt.Sprintf("🌊 %s `%s`\n", t.I18nBot("tgbot.messages.client_flow"), state.Flow)
		}
	case model.Trojan:
		msg += fmt.Sprintf("🔑 %s: `%s`\n", t.I18nBot("password"), state.TrPassword)
	case model.Shadowsocks:
		msg += fmt.Sprintf("🔑 %s: `%s`\n", t.I18nBot("password"), state.ShPassword)
	}

	msg += fmt.Sprintf("📝 %s `%s`\n", t.I18nBot("tgbot.messages.client_subid"), state.SubID)

	trafficLimit := t.I18nBot("tgbot.unlimited")
	if state.TotalGB > 0 {
		trafficLimit = common.FormatTraffic(state.TotalGB)
	}
	msg += fmt.Sprintf("📊 %s: %s\n", t.I18nBot("tgbot.buttons.limitTraffic"), trafficLimit)

	expireTime := t.I18nBot("tgbot.unlimited")
	if state.ExpiryTime < 0 {
		expireTime = fmt.Sprintf("%d %s", -state.ExpiryTime/(24*60*60*1000), t.I18nBot("tgbot.days"))
	} else if state.ExpiryTime > 0 {
		expireTime = time.Unix(state.ExpiryTime/1000, 0).Format("2006-01-02 15:04:05")
	}
	msg += fmt.Sprintf("📅 %s: %s\n", t.I18nBot("pages.client.expireDate"), expireTime)

	ipLimit := t.I18nBot("tgbot.unlimited")
	if state.LimitIP > 0 {
		ipLimit = strconv.Itoa(state.LimitIP)
	}
	msg += fmt.Sprintf("🚫 %s: %s\n", t.I18nBot("pages.inbounds.IPLimit"), ipLimit)

	if state.Comment != "" {
		msg += fmt.Sprintf("💬 %s: %s\n", t.I18nBot("comment"), state.Comment)
	}

	return msg, nil
}

func (t *Tgbot) getInbounds() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("get_clients "+strconv.Itoa(inbound.Id))))
	}
	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundClients(id int) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		return nil, err
	}
	clients, err := t.inboundService.GetClients(inbound)
	if err != nil {
		return nil, err
	}
	var buttons []telego.InlineKeyboardButton
	for _, client := range clients {
		buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery("client_get_usage "+client.Email)))
	}
	cols := 2
	if len(buttons) >= 10 {
		cols = 3
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundsAddClient() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		buttons = append(buttons, tu.InlineKeyboardButton(inbound.Remark).WithCallbackData(t.encodeQuery("add_client_to "+strconv.Itoa(inbound.Id))))
	}
	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) clientInfoMsg(traffic *xray.ClientTraffic, showTraffic, showExpiry, showIP, showTG, showOnline, showRefresh bool) string {
	msg := ""
	msg += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
	if showTraffic {
		msg += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(traffic.Up+traffic.Down), "Upload=="+common.FormatTraffic(traffic.Up), "Download=="+common.FormatTraffic(traffic.Down))
		limit := t.I18nBot("tgbot.unlimited")
		if traffic.Total > 0 {
			limit = common.FormatTraffic(traffic.Total)
		}
		msg += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic(traffic.Up+traffic.Down), "Total=="+limit)
	}
	if showExpiry {
		if traffic.ExpiryTime == 0 {
			msg += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
		} else {
			msg += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix(traffic.ExpiryTime/1000, 0).Format("2006-01-02 15:04:05"))
		}
	}
	if showOnline {
		status := t.I18nBot("tgbot.offline")
		if p != nil && slices.Contains(p.GetOnlineClients(), traffic.Email) {
			status = t.I18nBot("tgbot.online")
		}
		msg += t.I18nBot("tgbot.messages.online", "Status=="+status)
	}
	return msg + "\r\n"
}

func (t *Tgbot) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) {
	if !isRunning {
		return
	}
	params := &telego.SendMessageParams{
		ChatID:    tu.ID(chatId),
		Text:      msg,
		ParseMode: "HTML",
	}
	if len(replyMarkup) > 0 {
		params.ReplyMarkup = replyMarkup[0]
	}
	_, _ = bot.SendMessage(context.Background(), params)
}

func (t *Tgbot) SubmitAddClient(chatId int64) (bool, error) {
	state := t.getClientState(chatId)
	client := model.Client{
		ID:         state.Id,
		Flow:       state.Flow,
		Email:      state.Email,
		LimitIP:    state.LimitIP,
		TotalGB:    state.TotalGB,
		ExpiryTime: state.ExpiryTime,
		Enable:     state.Enable,
		TgID:       0,
		SubID:      state.SubID,
		Comment:    state.Comment,
		Reset:      state.Reset,
	}

	settings, _ := json.Marshal(map[string][]model.Client{
		"clients": {client},
	})

	inbound := &model.Inbound{
		Id:       state.ReceiverInboundID,
		Settings: string(settings),
	}

	return t.inboundService.AddInboundClient(inbound)
}

func (t *Tgbot) SendMsgToTgbotAdmins(msg string, replyMarkup ...telego.ReplyMarkup) {
	if len(replyMarkup) > 0 {
		for _, adminId := range adminIds {
			t.SendMsgToTgbot(adminId, msg, replyMarkup[0])
		}
	} else {
		for _, adminId := range adminIds {
			t.SendMsgToTgbot(adminId, msg)
		}
	}
}

func (t *Tgbot) SendReport() {
	runTime, err := t.settingService.GetTgbotRuntime()
	if err == nil && len(runTime) > 0 {
		msg := ""
		msg += t.I18nBot("tgbot.messages.report", "RunTime=="+runTime)
		msg += t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
		t.SendMsgToTgbotAdmins(msg)
	}

	info := t.sendServerUsage()
	t.SendMsgToTgbotAdmins(info)

	t.sendExhaustedToAdmins()
	t.notifyExhausted()

	backupEnable, err := t.settingService.GetTgBotBackup()
	if err == nil && backupEnable {
		t.SendBackupToAdmins()
	}
}

func (t *Tgbot) SendBackupToAdmins() {
	if !t.IsRunning() {
		return
	}
	for i, adminId := range adminIds {
		t.sendBackup(int64(adminId))
		if i < len(adminIds)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

func (t *Tgbot) sendExhaustedToAdmins() {
	if !t.IsRunning() {
		return
	}
	for _, adminId := range adminIds {
		t.getExhausted(int64(adminId))
	}
}

func (t *Tgbot) sendServerUsage() string {
	return t.prepareServerUsageInfo()
}

func (t *Tgbot) prepareServerUsageInfo() string {
	if cachedStats, found := t.getCachedServerStats(); found {
		return cachedStats
	}

	info, ipv4, ipv6 := "", "", ""

	if cachedStatus, found := t.getCachedStatus(); found {
		t.lastStatus = cachedStatus
	} else {
		t.lastStatus = t.serverService.GetStatus(t.lastStatus)
		t.setCachedStatus(t.lastStatus)
	}
	onlines := p.GetOnlineClients()

	info += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	info += t.I18nBot("tgbot.messages.version", "Version=="+config.GetVersion())
	info += t.I18nBot("tgbot.messages.xrayVersion", "XrayVersion=="+fmt.Sprint(t.lastStatus.Xray.Version))

	interfaces, err := net.Interfaces()
	if err == nil {
		for _, i := range interfaces {
			if (i.Flags & net.FlagUp) != 0 {
				addrs, _ := i.Addrs()
				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipv4 += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}
		info += t.I18nBot("tgbot.messages.ipv4", "IPv4=="+ipv4)
		info += t.I18nBot("tgbot.messages.ipv6", "IPv6=="+ipv6)
	}

	info += t.I18nBot("tgbot.messages.serverUpTime", "UpTime=="+strconv.FormatUint(t.lastStatus.Uptime/86400, 10), "Unit=="+t.I18nBot("tgbot.days"))
	info += t.I18nBot("tgbot.messages.serverLoad", "Load1=="+strconv.FormatFloat(t.lastStatus.Loads[0], 'f', 2, 64), "Load2=="+strconv.FormatFloat(t.lastStatus.Loads[1], 'f', 2, 64), "Load3=="+strconv.FormatFloat(t.lastStatus.Loads[2], 'f', 2, 64))
	info += t.I18nBot("tgbot.messages.serverMemory", "Current=="+common.FormatTraffic(int64(t.lastStatus.Mem.Current)), "Total=="+common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(len(onlines)))
	info += t.I18nBot("tgbot.messages.tcpCount", "Count=="+strconv.Itoa(t.lastStatus.TcpCount))
	info += t.I18nBot("tgbot.messages.udpCount", "Count=="+strconv.Itoa(t.lastStatus.UdpCount))
	info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), "Upload=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), "Download=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += t.I18nBot("tgbot.messages.xrayStatus", "State=="+fmt.Sprint(t.lastStatus.Xray.State))

	t.setCachedServerStats(info)
	return info
}

func (t *Tgbot) UserLoginNotify(attempt LoginAttempt) {
	if !t.IsRunning() {
		return
	}
	loginNotifyEnabled, err := t.settingService.GetTgBotLoginNotify()
	if err != nil || !loginNotifyEnabled {
		return
	}
	msg := ""
	switch attempt.Status {
	case LoginSuccess:
		msg += t.I18nBot("tgbot.messages.loginSuccess")
	case LoginFail:
		msg += t.I18nBot("tgbot.messages.loginFailed")
		if attempt.Reason != "" {
			msg += t.I18nBot("tgbot.messages.reason", "Reason=="+attempt.Reason)
		}
	}
	msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	msg += t.I18nBot("tgbot.messages.username", "Username=="+attempt.Username)
	msg += t.I18nBot("tgbot.messages.ip", "IP=="+attempt.IP)
	msg += t.I18nBot("tgbot.messages.time", "Time=="+attempt.Time)
	t.SendMsgToTgbotAdmins(msg)
}

func (t *Tgbot) notifyExhausted() {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, _ := t.inboundService.GetAllInbounds()

	var chatIDsDone []int64
	for _, inbound := range inbounds {
		if inbound.Enable {
			clients, err := t.inboundService.GetClients(inbound)
			if err == nil {
				for _, client := range clients {
					if client.TgID != 0 && !int64Contains(chatIDsDone, client.TgID) && !checkAdmin(client.TgID) {
						traffics, err := t.inboundService.GetClientTrafficTgBot(client.TgID)
						if err == nil && len(traffics) > 0 {
							var exhausted []xray.ClientTraffic
							for _, tr := range traffics {
								if tr.Enable {
									if (tr.ExpiryTime > 0 && tr.ExpiryTime-now < exDiff) || (tr.Total > 0 && tr.Total-(tr.Up+tr.Down) < trDiff) {
										exhausted = append(exhausted, *tr)
									}
								}
							}
							if len(exhausted) > 0 {
								output := t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients"))
								for _, ex := range exhausted {
									output += t.clientInfoMsg(&ex, true, false, false, true, true, false)
								}
								t.SendMsgToTgbot(client.TgID, output)
							}
							chatIDsDone = append(chatIDsDone, client.TgID)
						}
					}
				}
			}
		}
	}
}

func int64Contains(slice []int64, item int64) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
