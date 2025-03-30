package service

import (
    "context"
    "crypto/rand"
    "embed"
    "encoding/base64"
    "errors"
    "fmt"
    "math/big"
    "net"
    "net/url"
    "os"
    "regexp"
    "strconv"
    "strings"
    "time"

    "x-ui/config"
    "x-ui/database"
    "x-ui/database/model"
    "x-ui/logger"
    "x-ui/util/common"
    "x-ui/web/global"
    "x-ui/web/locale"
    "x-ui/xray"

    "github.com/google/uuid"
    "github.com/mymmrac/telego"
    th "github.com/mymmrac/telego/telegohandler"
    tu "github.com/mymmrac/telego/telegoutil"
    "github.com/valyala/fasthttp"
    "github.com/valyala/fasthttp/fasthttpproxy"
)

var (
    bot           *telego.Bot
    botHandler    *th.BotHandler
    adminIds      []int64
    isRunning     bool
    hostname      string
    hashStorage   *global.HashStorage
    handler       *th.Handler

    // clients data to adding new client
    receiver_inbound_ID    int
    client_Id              string
    client_Flow            string
    client_Email           string
    client_LimitIP         int
    client_TotalGB         int64
    client_ExpiryTime      int64
    client_Enable          bool
    client_TgID            string
    client_SubID           string
    client_Comment         string
    client_Reset           int
    client_Security        string
    client_ShPassword      string
    client_TrPassword      string
    client_Method          string
)

var userStates = make(map[int64]string)

type LoginStatus byte

const (
    LoginSuccess        LoginStatus = 1
    LoginFail           LoginStatus = 0
    EmptyTelegramUserID             = int64(0)
)

type Tgbot struct {
    inboundService InboundService
    settingService SettingService
    serverService  ServerService
    xrayService    XrayService
    lastStatus     *Status
}

func (t *Tgbot) NewTgbot() *Tgbot {
    return new(Tgbot)
}

func (t *Tgbot) I18nBot(name string, params ...string) string {
    return locale.I18n(locale.Bot, name, params...)
}

func (t *Tgbot) GetHashStorage() *global.HashStorage {
    return hashStorage
}

func (t *Tgbot) Start(i18nFS embed.FS) error {
    err := locale.InitLocalizer(i18nFS, &t.settingService)
    if err != nil {
        return err
    }

    hashStorage = global.NewHashStorage(20 * time.Minute)

    t.SetHostname()

    tgBotToken, err := t.settingService.GetTgBotToken()
    if err != nil || tgBotToken == "" {
        logger.Warning("Failed to get Telegram bot token:", err)
        return err
    }

    tgBotID, err := t.settingService.GetTgBotChatId()
    if err != nil {
        logger.Warning("Failed to get Telegram bot chat ID:", err)
        return err
    }

    if tgBotID != "" {
        for _, adminID := range strings.Split(tgBotID, ",") {
            id, err := strconv.Atoi(adminID)
            if err != nil {
                logger.Warning("Failed to parse admin ID from Telegram bot chat ID:", err)
                return err
            }
            adminIds = append(adminIds, int64(id))
        }
    }

    tgBotProxy, err := t.settingService.GetTgBotProxy()
    if err != nil {
        logger.Warning("Failed to get Telegram bot proxy URL:", err)
    }

    tgBotAPIServer, err := t.settingService.GetTgBotAPIServer()
    if err != nil {
        logger.Warning("Failed to get Telegram bot API server URL:", err)
    }

    bot, err = t.NewBot(tgBotToken, tgBotProxy, tgBotAPIServer)
    if err != nil {
        logger.Error("Failed to initialize Telegram bot API:", err)
        return err
    }

    if !isRunning {
        logger.Info("Telegram bot receiver started")
        go t.OnReceive()
        isRunning = true
    }

    return nil
}

func (t *Tgbot) NewBot(token string, proxyUrl string, apiServerUrl string) (*telego.Bot, error) {
    if proxyUrl == "" && apiServerUrl == "" {
        return telego.NewBot(token)
    }

    if proxyUrl != "" {
        if !strings.HasPrefix(proxyUrl, "socks5://") {
            logger.Warning("Invalid socks5 URL, using default")
            return telego.NewBot(token)
        }

        _, err := url.Parse(proxyUrl)
        if err != nil {
            logger.Warningf("Can't parse proxy URL, using default instance for tgbot: %v", err)
            return telego.NewBot(token)
        }

        return telego.NewBot(token, telego.WithFastHTTPClient(&fasthttp.Client{
            Dial: fasthttpproxy.FasthttpSocksDialer(proxyUrl),
        }))
    }

    if !strings.HasPrefix(apiServerUrl, "http") {
        logger.Warning("Invalid http(s) URL, using default")
        return telego.NewBot(token)
    }

    _, err := url.Parse(apiServerUrl)
    if err != nil {
        logger.Warningf("Can't parse API server URL, using default instance for tgbot: %v", err)
        return telego.NewBot(token)
    }

    return telego.NewBot(token, telego.WithAPIServer(apiServerUrl))
}

func (t *Tgbot) IsRunning() bool {
    return isRunning
}

func (t *Tgbot) SetHostname() {
    host, err := os.Hostname()
    if err != nil {
        logger.Error("get hostname error:", err)
        hostname = ""
        return
    }
    hostname = host
}

func (t *Tgbot) Stop() {
    botHandler.Stop()
    logger.Info("Stop Telegram receiver ...")
    isRunning = false
    adminIds = nil
}

func (t *Tgbot) encodeQuery(query string) string {
    if len(query) <= 64 {
        return query
    }

    return hashStorage.SaveHash(query)
}

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

func (t *Tgbot) OnReceive() {
    params := telego.GetUpdatesParams{
        Timeout: 10,
    }

    updates, _ := bot.UpdatesViaLongPolling(context.Background(), &params)

    botHandler, _ = th.NewBotHandler(bot, updates)

    botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
        delete(userStates, message.Chat.ID)
        t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.keyboardClosed"), tu.ReplyKeyboardRemove())
    }, th.TextEqual(t.I18nBot("tgbot.buttons.closeKeyboard")))

    botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
        delete(userStates, message.Chat.ID)
        t.answerCommand(&message, message.Chat.ID, checkAdmin(message.From.ID))
    }, th.AnyCommand())

    botHandler.HandleCallbackQuery(func(_ *telego.Bot, query telego.CallbackQuery) {
        delete(userStates, query.Message.GetChat().ID)
        t.answerCallback(&query, checkAdmin(query.From.ID))
    }, th.AnyCallbackQueryWithMessage())

    botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
        if userState, exists := userStates[message.Chat.ID]; exists {
            switch userState {
            case "awaiting_id":
                if client_Id == strings.TrimSpace(message.Text) {
                    t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
                    delete(userStates, message.Chat.ID)
                    inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                    message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
                    t.addClient(message.Chat.ID, message_text)
                    return
                }

                client_Id = strings.TrimSpace(message.Text)
                if t.isSingleWord(client_Id) {
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
                    inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                    message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
                    t.addClient(message.Chat.ID, message_text)
                }
            case "awaiting_password_tr":
                if client_TrPassword == strings.TrimSpace(message.Text) {
                    t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
                    delete(userStates, message.Chat.ID)
                    return
                }

                client_TrPassword = strings.TrimSpace(message.Text)
                if t.isSingleWord(client_TrPassword) {
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
                    inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                    message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
                    t.addClient(message.Chat.ID, message_text)
                }
            case "awaiting_password_sh":
                if client_ShPassword == strings.TrimSpace(message.Text) {
                    t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
                    delete(userStates, message.Chat.ID)
                    return
                }

                client_ShPassword = strings.TrimSpace(message.Text)
                if t.isSingleWord(client_ShPassword) {
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
                    inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                    message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
                    t.addClient(message.Chat.ID, message_text)
                }
            case "awaiting_email":
                if client_Email == strings.TrimSpace(message.Text) {
                    t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
                    delete(userStates, message.Chat.ID)
                    return
                }

                client_Email = strings.TrimSpace(message.Text)
                if t.isSingleWord(client_Email) {
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
                    inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                    message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
                    t.addClient(message.Chat.ID, message_text)
                }
            case "awaiting_comment":
                if client_Comment == strings.TrimSpace(message.Text) {
                    t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
                    delete(userStates, message.Chat.ID)
                    return
                }

                client_Comment = strings.TrimSpace(message.Text)
                t.SendMsgToTgbotDeleteAfter(message.Chat.ID, t.I18nBot("tgbot.messages.received_comment"), 3, tu.ReplyKeyboardRemove())
                delete(userStates, message.Chat.ID)
                inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
                message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
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
    }, th.AnyMessage())

    botHandler.Start()
}

func (t *Tgbot) answerCommand(message *telego.Message, chatId int64, isAdmin bool) {
    msg, onlyMessage := "", false

    command, _, commandArgs := tu.ParseCommand(message.Text)

    handleUnknownCommand := func() {
        msg += t.I18nBot("tgbot.commands.unknown")
    }

    switch command {
    case "help":
        msg += t.I18nBot("tgbot.commands.help")
        msg += t.I18nBot("tgbot.commands.pleaseChoose")
    case "start":
        msg += t.I18nBot("tgbot.commands.start", "Firstname=="+message.From.FirstName)
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

// Helper function to send the message based on onlyMessage flag.
func (t *Tgbot) sendResponse(chatId int64, msg string, onlyMessage, isAdmin bool) {
    if onlyMessage {
        t.SendMsgToTgbot(chatId, msg)
    } else {
        t.SendAnswer(chatId, msg, isAdmin)
    }
}

func (t *Tgbot) randomLowerAndNum(length int) string {
    charset := "abcdefghijklmnopqrstuvwxyz0123456789"
    bytes := make([]byte, length)
    for i := range bytes {
        randomIndex
