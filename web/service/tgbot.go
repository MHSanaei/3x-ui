package service

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"x-ui/config"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	"github.com/gin-gonic/gin"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var bot *telego.Bot
var botHandler *th.BotHandler
var adminIds []int64
var isRunning bool

type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1
	LoginFail    LoginStatus = 0
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

func (t *Tgbot) Start() error {
	tgBottoken, err := t.settingService.GetTgBotToken()
	if err != nil || tgBottoken == "" {
		logger.Warning("Get TgBotToken failed:", err)
		return err
	}

	tgBotid, err := t.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("Get GetTgBotChatId failed:", err)
		return err
	}

	for _, adminId := range strings.Split(tgBotid, ",") {
		id, err := strconv.Atoi(adminId)
		if err != nil {
			logger.Warning("Failed to get IDs from GetTgBotChatId:", err)
			return err
		}
		adminIds = append(adminIds, int64(id))
	}

	bot, err = telego.NewBot(tgBottoken)
	if err != nil {
		fmt.Println("Get tgbot's api error:", err)
		return err
	}

	// listen for TG bot income messages
	if !isRunning {
		logger.Info("Starting Telegram receiver ...")
		go t.OnReceive()
		isRunning = true
	}

	return nil
}

func (t *Tgbot) IsRunning() bool {
	return isRunning
}

func (t *Tgbot) Stop() {
	botHandler.Stop()
	bot.StopLongPolling()
	logger.Info("Stop Telegram receiver ...")
	isRunning = false
	adminIds = nil
}

func (t *Tgbot) OnReceive() {
	params := telego.GetUpdatesParams{
		Timeout: 10,
	}

	updates, _ := bot.UpdatesViaLongPolling(&params)

	botHandler, _ = th.NewBotHandler(bot, updates)

	botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
		t.SendMsgToTgbot(message.Chat.ID, "Custom Keyboard Closed!", tu.ReplyKeyboardRemove())
	}, th.TextEqual("❌ Close Keyboard"))

	botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
		t.answerCommand(&message, message.Chat.ID, checkAdmin(message.From.ID))
	}, th.AnyCommand())

	botHandler.HandleCallbackQuery(func(_ *telego.Bot, query telego.CallbackQuery) {
		t.asnwerCallback(&query, checkAdmin(query.From.ID))
	}, th.AnyCallbackQueryWithMessage())

	botHandler.HandleMessage(func(_ *telego.Bot, message telego.Message) {
		if message.UserShared != nil {
			if checkAdmin(message.From.ID) {
				err := t.inboundService.SetClientTelegramUserID(message.UserShared.RequestID, strconv.FormatInt(message.UserShared.UserID, 10))
				var output string
				if err != nil {
					output = "❌ Error in user selection!"
				} else {
					output = "✅ Telegram User saved."
				}
				t.SendMsgToTgbot(message.Chat.ID, output, tu.ReplyKeyboardRemove())
			} else {
				t.SendMsgToTgbot(message.Chat.ID, "No result!", tu.ReplyKeyboardRemove())
			}
		}
	}, th.AnyMessage())

	botHandler.Start()
}

func (t *Tgbot) answerCommand(message *telego.Message, chatId int64, isAdmin bool) {
	msg := ""

	command, commandArgs := tu.ParseCommand(message.Text)

	// Extract the command from the Message.
	switch command {
	case "help":
		msg = "This bot is providing you some specefic data from the server.\n\n Please choose:"
	case "start":
		msg = "Hello <i>" + message.From.FirstName + "</i> 👋"
		if isAdmin {
			hostname, _ := os.Hostname()
			msg += "\nWelcome to <b>" + hostname + "</b> management bot"
		}
		msg += "\n\nI can do some magics for you, please choose:"
	case "status":
		msg = "bot is ok ✅"
	case "usage":
		if len(commandArgs) > 0 {
			if isAdmin {
				t.searchClient(chatId, commandArgs[0])
			} else {
				t.searchForClient(chatId, commandArgs[0])
			}
		} else {
			msg = "❗Please provide a text for search!"
		}
	case "inbound":
		if isAdmin && len(commandArgs) > 0 {
			t.searchInbound(chatId, commandArgs[0])
		} else {
			msg = "❗ Unknown command"
		}
	default:
		msg = "❗ Unknown command"
	}
	t.SendAnswer(chatId, msg, isAdmin)
}

func (t *Tgbot) asnwerCallback(callbackQuery *telego.CallbackQuery, isAdmin bool) {

	chatId := callbackQuery.Message.Chat.ID

	if isAdmin {
		dataArray := strings.Split(callbackQuery.Data, " ")
		if len(dataArray) >= 2 && len(dataArray[1]) > 0 {
			email := dataArray[1]
			switch dataArray[0] {
			case "client_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Client refreshed successfully.", email))
				t.searchClient(chatId, email, callbackQuery.Message.MessageID)
			case "client_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("❌ %s : Operation canceled.", email))
				t.searchClient(chatId, email, callbackQuery.Message.MessageID)
			case "ips_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IPs refreshed successfully.", email))
				t.searchClientIps(chatId, email, callbackQuery.Message.MessageID)
			case "ips_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("❌ %s : Operation canceled.", email))
				t.searchClientIps(chatId, email, callbackQuery.Message.MessageID)
			case "tgid_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Client's Telegram User refreshed successfully.", email))
				t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.MessageID)
			case "tgid_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("❌ %s : Operation canceled.", email))
				t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.MessageID)
			case "reset_traffic":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ Cancel Reset").WithCallbackData("client_cancel "+email),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("✅ Confirm Reset Traffic?").WithCallbackData("reset_traffic_c "+email),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.MessageID, inlineKeyboard)
			case "reset_traffic_c":
				err := t.inboundService.ResetClientTrafficByEmail(email)
				if err == nil {
					t.xrayService.SetToNeedRestart()
					t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Traffic reset successfully.", email))
					t.searchClient(chatId, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			case "reset_exp":
				var inlineKeyboard = tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ Cancel Reset").WithCallbackData("client_cancel "+email),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("♾ Unlimited").WithCallbackData("reset_exp_c "+email+" 0"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1 Month").WithCallbackData("reset_exp_c "+email+" 30"),
						tu.InlineKeyboardButton("2 Months").WithCallbackData("reset_exp_c "+email+" 60"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("3 Months").WithCallbackData("reset_exp_c "+email+" 90"),
						tu.InlineKeyboardButton("6 Months").WithCallbackData("reset_exp_c "+email+" 180"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("9 Months").WithCallbackData("reset_exp_c "+email+" 270"),
						tu.InlineKeyboardButton("12 Months").WithCallbackData("reset_exp_c "+email+" 360"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("10 Days").WithCallbackData("reset_exp_c "+email+" 10"),
						tu.InlineKeyboardButton("20 Days").WithCallbackData("reset_exp_c "+email+" 20"),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.MessageID, inlineKeyboard)
			case "reset_exp_c":
				if len(dataArray) == 3 {
					days, err := strconv.Atoi(dataArray[2])
					if err == nil {
						var date int64 = 0
						if days > 0 {
							date = int64(-(days * 24 * 60 * 60000))
						}
						err := t.inboundService.ResetClientExpiryTimeByEmail(email, date)
						if err == nil {
							t.xrayService.SetToNeedRestart()
							t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Expire days reset successfully.", email))
							t.searchClient(chatId, email, callbackQuery.Message.MessageID)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				t.searchClient(chatId, email, callbackQuery.Message.MessageID)
			case "ip_limit":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ Cancel IP Limit").WithCallbackData("client_cancel "+email),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("♾ Unlimited").WithCallbackData("ip_limit_c "+email+" 0"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1").WithCallbackData("ip_limit_c "+email+" 1"),
						tu.InlineKeyboardButton("2").WithCallbackData("ip_limit_c "+email+" 2"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("3").WithCallbackData("ip_limit_c "+email+" 3"),
						tu.InlineKeyboardButton("4").WithCallbackData("ip_limit_c "+email+" 4"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("5").WithCallbackData("ip_limit_c "+email+" 5"),
						tu.InlineKeyboardButton("6").WithCallbackData("ip_limit_c "+email+" 6"),
						tu.InlineKeyboardButton("7").WithCallbackData("ip_limit_c "+email+" 7"),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("8").WithCallbackData("ip_limit_c "+email+" 8"),
						tu.InlineKeyboardButton("9").WithCallbackData("ip_limit_c "+email+" 9"),
						tu.InlineKeyboardButton("10").WithCallbackData("ip_limit_c "+email+" 10"),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.MessageID, inlineKeyboard)
			case "ip_limit_c":
				if len(dataArray) == 3 {
					count, err := strconv.Atoi(dataArray[2])
					if err == nil {
						err := t.inboundService.ResetClientIpLimitByEmail(email, count)
						if err == nil {
							t.xrayService.SetToNeedRestart()
							t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IP limit %d saved successfully.", email, count))
							t.searchClient(chatId, email, callbackQuery.Message.MessageID)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				t.searchClient(chatId, email, callbackQuery.Message.MessageID)
			case "clear_ips":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ Cancel").WithCallbackData("ips_cancel "+email),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("✅ Confirm Clear IPs?").WithCallbackData("clear_ips_c "+email),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.MessageID, inlineKeyboard)
			case "clear_ips_c":
				err := t.inboundService.ClearClientIps(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IPs cleared successfully.", email))
					t.searchClientIps(chatId, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			case "ip_log":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Get IP Log.", email))
				t.searchClientIps(chatId, email)
			case "tg_user":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Get Telegram User Info.", email))
				t.clientTelegramUserInfo(chatId, email)
			case "tgid_remove":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ Cancel").WithCallbackData("tgid_cancel "+email),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("✅ Confirm Remove Telegram User?").WithCallbackData("tgid_remove_c "+email),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.MessageID, inlineKeyboard)
			case "tgid_remove_c":
				traffic, err := t.inboundService.GetClientTrafficByEmail(email)
				if err != nil || traffic == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
					return
				}
				err = t.inboundService.SetClientTelegramUserID(traffic.Id, "")
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Telegram User removed successfully.", email))
					t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			case "toggle_enable":
				enabled, err := t.inboundService.ToggleClientEnableByEmail(email)
				if err == nil {
					t.xrayService.SetToNeedRestart()
					if enabled {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Enabled successfully.", email))
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Disabled successfully.", email))
					}
					t.searchClient(chatId, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			}
			return
		}
	}

	// Respond to the callback query, telling Telegram to show the user
	// a message with the data received.
	t.sendCallbackAnswerTgBot(callbackQuery.ID, callbackQuery.Data)

	switch callbackQuery.Data {
	case "get_usage":
		t.SendMsgToTgbot(chatId, t.getServerUsage())
	case "inbounds":
		t.SendMsgToTgbot(chatId, t.getInboundUsages())
	case "deplete_soon":
		t.SendMsgToTgbot(chatId, t.getExhausted())
	case "get_backup":
		t.sendBackup(chatId)
	case "client_traffic":
		t.getClientUsage(chatId, callbackQuery.From.Username, strconv.FormatInt(callbackQuery.From.ID, 10))
	case "client_commands":
		t.SendMsgToTgbot(chatId, "To search for statistics, just use folowing command:\r\n \r\n<code>/usage [UID|Password]</code>\r\n \r\nUse UID for vmess/vless and Password for Trojan.")
	case "commands":
		t.SendMsgToTgbot(chatId, "Search for a client email:\r\n<code>/usage email</code>\r\n \r\nSearch for inbounds (with client stats):\r\n<code>/inbound [remark]</code>")
	}
}

func checkAdmin(tgId int64) bool {
	for _, adminId := range adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
}

func (t *Tgbot) SendAnswer(chatId int64, msg string, isAdmin bool) {
	numericKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Server Usage").WithCallbackData("get_usage"),
			tu.InlineKeyboardButton("Get DB Backup").WithCallbackData("get_backup"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Get Inbounds").WithCallbackData("inbounds"),
			tu.InlineKeyboardButton("Deplete soon").WithCallbackData("deplete_soon"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Commands").WithCallbackData("commands"),
		),
	)
	numericKeyboardClient := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Get Usage").WithCallbackData("client_traffic"),
			tu.InlineKeyboardButton("Commands").WithCallbackData("client_commands"),
		),
	)
	var ReplyMarkup telego.ReplyMarkup
	if isAdmin {
		ReplyMarkup = numericKeyboard
	} else {
		ReplyMarkup = numericKeyboardClient
	}
	t.SendMsgToTgbot(chatId, msg, ReplyMarkup)
}

func (t *Tgbot) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) {
	if !isRunning {
		return
	}
	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return
	}

	var allMessages []string
	limit := 2000

	// paging message if it is big
	if len(msg) > limit {
		messages := strings.Split(msg, "\r\n \r\n")
		lastIndex := -1

		for _, message := range messages {
			if (len(allMessages) == 0) || (len(allMessages[lastIndex])+len(message) > limit) {
				allMessages = append(allMessages, message)
				lastIndex++
			} else {
				allMessages[lastIndex] += "\r\n \r\n" + message
			}
		}
	} else {
		allMessages = append(allMessages, msg)
	}
	for _, message := range allMessages {
		params := telego.SendMessageParams{
			ChatID:    tu.ID(chatId),
			Text:      message,
			ParseMode: "HTML",
		}
		if len(replyMarkup) > 0 {
			params.ReplyMarkup = replyMarkup[0]
		}
		_, err := bot.SendMessage(&params)
		if err != nil {
			logger.Warning("Error sending telegram message :", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (t *Tgbot) SendMsgToTgbotAdmins(msg string) {
	for _, adminId := range adminIds {
		t.SendMsgToTgbot(adminId, msg)
	}
}

func (t *Tgbot) SendReport() {
	runTime, err := t.settingService.GetTgbotRuntime()
	if err == nil && len(runTime) > 0 {
		t.SendMsgToTgbotAdmins("🕰 Scheduled reports: " + runTime + "\r\nDate-Time: " + time.Now().Format("2006-01-02 15:04:05"))
	}
	info := t.getServerUsage()
	t.SendMsgToTgbotAdmins(info)
	exhausted := t.getExhausted()
	t.SendMsgToTgbotAdmins(exhausted)
	backupEnable, err := t.settingService.GetTgBotBackup()
	if err == nil && backupEnable {
		for _, adminId := range adminIds {
			t.sendBackup(int64(adminId))
		}
	}
}

func (t *Tgbot) SendBackUP(c *gin.Context) {
	for _, adminId := range adminIds {
		t.sendBackup(int64(adminId))
	}
}

func (t *Tgbot) getServerUsage() string {
	var info string
	//get hostname
	hostname, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		hostname = ""
	}
	info = fmt.Sprintf("💻 Hostname: %s\r\n", hostname)
	info += fmt.Sprintf("🚀X-UI Version: %s\r\n", config.GetVersion())
	//get ip address
	var ip string
	var ipv6 string
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err:", err.Error())
		info += "🌐 IP: Unknown\r\n \r\n"
	} else {
		for i := 0; i < len(netInterfaces); i++ {
			if (netInterfaces[i].Flags & net.FlagUp) != 0 {
				addrs, _ := netInterfaces[i].Addrs()

				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ip += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}
		info += fmt.Sprintf("🌐IP: %s\r\n🌐IPv6: %s\r\n", ip, ipv6)
	}

	// get latest status of server
	t.lastStatus = t.serverService.GetStatus(t.lastStatus)
	info += fmt.Sprintf("🔌Server Uptime: %d days\r\n", int(t.lastStatus.Uptime/86400))
	info += fmt.Sprintf("📈Server Load: %.1f, %.1f, %.1f\r\n", t.lastStatus.Loads[0], t.lastStatus.Loads[1], t.lastStatus.Loads[2])
	info += fmt.Sprintf("📋Server Memory: %s/%s\r\n", common.FormatTraffic(int64(t.lastStatus.Mem.Current)), common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += fmt.Sprintf("🔹TcpCount: %d\r\n", t.lastStatus.TcpCount)
	info += fmt.Sprintf("🔸UdpCount: %d\r\n", t.lastStatus.UdpCount)
	info += fmt.Sprintf("🚦Traffic: %s (↑%s,↓%s)\r\n", common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += fmt.Sprintf("ℹXray status: %s", t.lastStatus.Xray.State)

	return info
}

func (t *Tgbot) UserLoginNotify(username string, ip string, time string, status LoginStatus) {
	if !t.IsRunning() {
		return
	}

	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed, invalid info!")
		return
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warning("get hostname error:", err)
		return
	}

	msg := ""
	if status == LoginSuccess {
		msg = fmt.Sprintf("✅ Successfully logged-in to the panel\r\nHostname:%s\r\n", hostname)
	} else if status == LoginFail {
		msg = fmt.Sprintf("❗ Login to the panel was unsuccessful\r\nHostname:%s\r\n", hostname)
	}
	msg += fmt.Sprintf("⏰ Time:%s\r\n", time)
	msg += fmt.Sprintf("🆔 Username:%s\r\n", username)
	msg += fmt.Sprintf("🌐 IP:%s\r\n", ip)

	t.SendMsgToTgbotAdmins(msg)
}

func (t *Tgbot) getInboundUsages() string {
	info := ""
	// get traffic
	inbouds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		info += "❌ Failed to get inbounds"
	} else {
		// NOTE:If there no any sessions here,need to notify here
		// TODO:Sub-node push, automatic conversion format
		for _, inbound := range inbouds {
			info += fmt.Sprintf("📍Inbound:%s\r\nPort:%d\r\n", inbound.Remark, inbound.Port)
			info += fmt.Sprintf("Traffic: %s (↑%s,↓%s)\r\n", common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				info += "Expire date: ♾ Unlimited\r\n \r\n"
			} else {
				info += fmt.Sprintf("Expire date:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}
	return info
}

func (t *Tgbot) getClientUsage(chatId int64, tgUserName string, tgUserID string) {
	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(traffics) == 0 {
		if len(tgUserName) == 0 {
			msg := "Your configuration is not found!\nPlease ask your Admin to use your telegram user id in your configuration(s).\n\nYour user id: <b>" + tgUserID + "</b>"
			t.SendMsgToTgbot(chatId, msg)
			return
		}
		traffics, err = t.inboundService.GetClientTrafficTgBot(tgUserName)
	}
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(traffics) == 0 {
		msg := "Your configuration is not found!\nPlease ask your Admin to use your telegram username or user id in your configuration(s).\n\nYour username: <b>@" + tgUserName + "</b>\n\nYour user id: <b>" + tgUserID + "</b>"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	for _, traffic := range traffics {
		expiryTime := ""
		if traffic.ExpiryTime == 0 {
			expiryTime = "♾Unlimited"
		} else if traffic.ExpiryTime < 0 {
			expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
		} else {
			expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
		}
		total := ""
		if traffic.Total == 0 {
			total = "♾Unlimited"
		} else {
			total = common.FormatTraffic((traffic.Total))
		}
		output := fmt.Sprintf("💡 Active: %t\r\n📧 Email: %s\r\n🔼 Upload↑: %s\r\n🔽 Download↓: %s\r\n🔄 Total: %s / %s\r\n📅 Expire in: %s\r\n",
			traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
			total, expiryTime)
		t.SendMsgToTgbot(chatId, output)
	}
	t.SendAnswer(chatId, "Please choose:", false)
}

func (t *Tgbot) searchClientIps(chatId int64, email string, messageID ...int) {
	ips, err := t.inboundService.GetInboundClientIps(email)
	if err != nil || len(ips) == 0 {
		ips = "No IP Record"
	}
	output := fmt.Sprintf("📧 Email: %s\r\n🔢 IPs: \r\n%s\r\n", email, ips)
	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Refresh").WithCallbackData("ips_refresh "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Clear IPs").WithCallbackData("clear_ips "+email),
		),
	)
	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

func (t *Tgbot) clientTelegramUserInfo(chatId int64, email string, messageID ...int) {
	traffic, client, err := t.inboundService.GetClientByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if client == nil {
		msg := "No result!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	tgId := "None"
	if len(client.TgID) > 0 {
		tgId = client.TgID
	}

	output := fmt.Sprintf("📧 Email: %s\r\n👤 Telegram User: %s\r\n", email, tgId)
	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Refresh").WithCallbackData("tgid_refresh "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Remove Telegram User").WithCallbackData("tgid_remove "+email),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
		requestUser := telego.KeyboardButtonRequestUser{
			RequestID: int32(traffic.Id),
			UserIsBot: false,
		}
		keyboard := tu.Keyboard(
			tu.KeyboardRow(
				tu.KeyboardButton("👤 Select Telegram User").WithRequestUser(&requestUser),
			),
			tu.KeyboardRow(
				tu.KeyboardButton("❌ Close Keyboard"),
			),
		).WithIsPersistent().WithResizeKeyboard()
		t.SendMsgToTgbot(chatId, "👤 Select a telegram user:", keyboard)
	}
}

func (t *Tgbot) searchClient(chatId int64, email string, messageID ...int) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := "No result!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = "♾Unlimited"
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
	} else {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	}

	total := ""
	if traffic.Total == 0 {
		total = "♾Unlimited"
	} else {
		total = common.FormatTraffic((traffic.Total))
	}

	output := fmt.Sprintf("💡 Active: %t\r\n📧 Email: %s\r\n🔼 Upload↑: %s\r\n🔽 Download↓: %s\r\n🔄 Total: %s / %s\r\n📅 Expire in: %s\r\n",
		traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
		total, expiryTime)

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Refresh").WithCallbackData("client_refresh "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📈 Reset Traffic").WithCallbackData("reset_traffic "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📅 Reset Expire Days").WithCallbackData("reset_exp "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔢 IP Log").WithCallbackData("ip_log "+email),
			tu.InlineKeyboardButton("🔢 IP Limit").WithCallbackData("ip_limit "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("👤 Set Telegram User").WithCallbackData("tg_user "+email),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔘 Enable / Disable").WithCallbackData("toggle_enable "+email),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbouds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(inbouds) == 0 {
		msg := "❌ No inbounds found!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbouds {
		info := ""
		info += fmt.Sprintf("📍Inbound:%s\r\nPort:%d\r\n", inbound.Remark, inbound.Port)
		info += fmt.Sprintf("Traffic: %s (↑%s,↓%s)\r\n", common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
		if inbound.ExpiryTime == 0 {
			info += "Expire date: ♾ Unlimited\r\n \r\n"
		} else {
			info += fmt.Sprintf("Expire date:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		for _, traffic := range inbound.ClientStats {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = "♾Unlimited"
			} else if traffic.ExpiryTime < 0 {
				expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}

			total := ""
			if traffic.Total == 0 {
				total = "♾Unlimited"
			} else {
				total = common.FormatTraffic((traffic.Total))
			}
			output := fmt.Sprintf("💡 Active: %t\r\n📧 Email: %s\r\n🔼 Upload↑: %s\r\n🔽 Download↓: %s\r\n🔄 Total: %s / %s\r\n📅 Expire in: %s\r\n",
				traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
				total, expiryTime)
			t.SendMsgToTgbot(chatId, output)
		}
	}
}

func (t *Tgbot) searchForClient(chatId int64, query string) {
	traffic, err := t.inboundService.SearchClientTraffic(query)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := "No result!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = "♾Unlimited"
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
	} else {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	}

	total := ""
	if traffic.Total == 0 {
		total = "♾Unlimited"
	} else {
		total = common.FormatTraffic((traffic.Total))
	}

	output := fmt.Sprintf("💡 Active: %t\r\n📧 Email: %s\r\n🔼 Upload↑: %s\r\n🔽 Download↓: %s\r\n🔄 Total: %s / %s\r\n📅 Expire in: %s\r\n",
		traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
		total, expiryTime)
	t.SendMsgToTgbot(chatId, output)
}

func (t *Tgbot) getExhausted() string {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	var exhaustedInbounds []model.Inbound
	var exhaustedClients []xray.ClientTraffic
	var disabledInbounds []model.Inbound
	var disabledClients []xray.ClientTraffic

	output := ""
	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load Inbounds", err)
	}

	for _, inbound := range inbounds {
		if inbound.Enable {
			if (inbound.ExpiryTime > 0 && (inbound.ExpiryTime-now < exDiff)) ||
				(inbound.Total > 0 && (inbound.Total-(inbound.Up+inbound.Down) < trDiff)) {
				exhaustedInbounds = append(exhaustedInbounds, *inbound)
			}
			if len(inbound.ClientStats) > 0 {
				for _, client := range inbound.ClientStats {
					if client.Enable {
						if (client.ExpiryTime > 0 && (client.ExpiryTime-now < exDiff)) ||
							(client.Total > 0 && (client.Total-(client.Up+client.Down) < trDiff)) {
							exhaustedClients = append(exhaustedClients, client)
						}
					} else {
						disabledClients = append(disabledClients, client)
					}
				}
			}
		} else {
			disabledInbounds = append(disabledInbounds, *inbound)
		}
	}

	output += fmt.Sprintf("Exhausted Inbounds count:\r\n🛑 Disabled: %d\r\n🔜 Deplete soon: %d\r\n \r\n", len(disabledInbounds), len(exhaustedInbounds))
	if len(exhaustedInbounds) > 0 {
		output += "Exhausted Inbounds:\r\n"
		for _, inbound := range exhaustedInbounds {
			output += fmt.Sprintf("📍Inbound:%s\r\nPort:%d\r\nTraffic: %s (↑%s,↓%s)\r\n", inbound.Remark, inbound.Port, common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				output += "Expire date: ♾Unlimited\r\n \r\n"
			} else {
				output += fmt.Sprintf("Expire date:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}

	output += fmt.Sprintf("Exhausted Clients count:\r\n🛑 Exhausted: %d\r\n🔜 Deplete soon: %d\r\n \r\n", len(disabledClients), len(exhaustedClients))
	if len(exhaustedClients) > 0 {
		output += "Exhausted Clients:\r\n"
		for _, traffic := range exhaustedClients {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = "♾Unlimited"
			} else if traffic.ExpiryTime < 0 {
				expiryTime += fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}
			total := ""
			if traffic.Total == 0 {
				total = "♾Unlimited"
			} else {
				total = common.FormatTraffic((traffic.Total))
			}
			output += fmt.Sprintf("💡 Active: %t\r\n📧 Email: %s\r\n🔼 Upload↑: %s\r\n🔽 Download↓: %s\r\n🔄 Total: %s / %s\r\n📅 Expire date: %s\r\n \r\n",
				traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
				total, expiryTime)
		}
	}

	return output
}

func (t *Tgbot) sendBackup(chatId int64) {
	if !t.IsRunning() {
		return
	}

	sendingTime := time.Now().Format("2006-01-02 15:04:05")
	t.SendMsgToTgbot(chatId, "Backup time: "+sendingTime)
	file, err := os.Open(config.GetDBPath())
	if err != nil {
		logger.Warning("Error in opening db file for backup: ", err)
	}
	document := tu.Document(
		tu.ID(chatId),
		tu.File(file),
	)
	_, err = bot.SendDocument(document)
	if err != nil {
		logger.Warning("Error in uploading backup: ", err)
	}
	file, err = os.Open(xray.GetConfigPath())
	if err != nil {
		logger.Warning("Error in opening config.json file for backup: ", err)
	}
	document = tu.Document(
		tu.ID(chatId),
		tu.File(file),
	)
	_, err = bot.SendDocument(document)
	if err != nil {
		logger.Warning("Error in uploading config.json: ", err)
	}
}

func (t *Tgbot) sendCallbackAnswerTgBot(id string, message string) {
	params := telego.AnswerCallbackQueryParams{
		CallbackQueryID: id,
		Text:            message,
	}
	if err := bot.AnswerCallbackQuery(&params); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageCallbackTgBot(chatId int64, messageID int, inlineKeyboard *telego.InlineKeyboardMarkup) {
	params := telego.EditMessageReplyMarkupParams{
		ChatID:      tu.ID(chatId),
		MessageID:   messageID,
		ReplyMarkup: inlineKeyboard,
	}
	if _, err := bot.EditMessageReplyMarkup(&params); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageTgBot(chatId int64, messageID int, text string, inlineKeyboard ...*telego.InlineKeyboardMarkup) {
	params := telego.EditMessageTextParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
	}
	if len(inlineKeyboard) > 0 {
		params.ReplyMarkup = inlineKeyboard[0]
	}
	if _, err := bot.EditMessageText(&params); err != nil {
		logger.Warning(err)
	}
}
