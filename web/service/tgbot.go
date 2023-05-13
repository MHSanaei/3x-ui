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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var bot *tgbotapi.BotAPI
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

	bot, err = tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		fmt.Println("Get tgbot's api error:", err)
		return err
	}
	bot.Debug = false

	// listen for TG bot income messages
	if !isRunning {
		logger.Info("Starting Telegram receiver ...")
		go t.OnReceive()
		isRunning = true
	}

	return nil
}

func (t *Tgbot) IsRunnging() bool {
	return isRunning
}

func (t *Tgbot) Stop() {
	bot.StopReceivingUpdates()
	logger.Info("Stop Telegram receiver ...")
	isRunning = false
	adminIds = nil
}

func (t *Tgbot) OnReceive() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		tgId := update.FromChat().ID
		chatId := update.FromChat().ChatConfig().ChatID
		isAdmin := checkAdmin(tgId)
		if update.Message == nil {
			if update.CallbackQuery != nil {
				t.asnwerCallback(update.CallbackQuery, isAdmin)
			}
		} else {
			if update.Message.IsCommand() {
				t.answerCommand(update.Message, chatId, isAdmin)
			}
		}
	}
}

func (t *Tgbot) answerCommand(message *tgbotapi.Message, chatId int64, isAdmin bool) {
	msg := ""
	// Extract the command from the Message.
	switch message.Command() {
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
		if len(message.CommandArguments()) > 1 {
			if isAdmin {
				t.searchClient(chatId, message.CommandArguments())
			} else {
				t.searchForClient(chatId, message.CommandArguments())
			}
		} else {
			msg = "❗Please provide a text for search!"
		}
	case "inbound":
		if isAdmin {
			t.searchInbound(chatId, message.CommandArguments())
		} else {
			msg = "❗ Unknown command"
		}
	default:
		msg = "❗ Unknown command"
	}
	t.SendAnswer(chatId, msg, isAdmin)
}

func (t *Tgbot) asnwerCallback(callbackQuery *tgbotapi.CallbackQuery, isAdmin bool) {

	if isAdmin {
		dataArray := strings.Split(callbackQuery.Data, " ")
		if len(dataArray) >= 2 && len(dataArray[1]) > 0 {
			email := dataArray[1]
			switch dataArray[0] {
			case "client_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Client Refreshed successfully.", email))
				t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "client_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("❌ %s : Operation canceled.", email))
				t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "ips_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IPs Refreshed successfully.", email))
				t.searchClientIps(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "ips_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("❌ %s : Operation canceled.", email))
				t.searchClientIps(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "reset_traffic":
				var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Reset", "client_cancel "+email),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Confirm Reset Traffic?", "reset_traffic_c "+email),
					),
				)
				t.editMessageCallbackTgBot(callbackQuery.From.ID, callbackQuery.Message.MessageID, inlineKeyboard)
			case "reset_traffic_c":
				err := t.inboundService.ResetClientTrafficByEmail(email)
				if err == nil {
					t.xrayService.SetToNeedRestart()
					t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Traffic reset successfully.", email))
					t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			case "reset_exp":
				var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Reset", "client_cancel "+email),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("♾ Unlimited", "reset_exp_c "+email+" 0"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("1 Month", "reset_exp_c "+email+" 30"),
						tgbotapi.NewInlineKeyboardButtonData("2 Months", "reset_exp_c "+email+" 60"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("3 Months", "reset_exp_c "+email+" 90"),
						tgbotapi.NewInlineKeyboardButtonData("6 Months", "reset_exp_c "+email+" 180"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("9 Months", "reset_exp_c "+email+" 270"),
						tgbotapi.NewInlineKeyboardButtonData("12 Months", "reset_exp_c "+email+" 360"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("10 Days", "reset_exp_c "+email+" 10"),
						tgbotapi.NewInlineKeyboardButtonData("20 Days", "reset_exp_c "+email+" 20"),
					),
				)
				t.editMessageCallbackTgBot(callbackQuery.From.ID, callbackQuery.Message.MessageID, inlineKeyboard)
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
							t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "ip_limit":
				var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel IP Limit", "client_cancel "+email),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("♾ Unlimited", "ip_limit_c "+email+" 0"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("1", "ip_limit_c "+email+" 1"),
						tgbotapi.NewInlineKeyboardButtonData("2", "ip_limit_c "+email+" 2"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("3", "ip_limit_c "+email+" 3"),
						tgbotapi.NewInlineKeyboardButtonData("4", "ip_limit_c "+email+" 4"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("5", "ip_limit_c "+email+" 5"),
						tgbotapi.NewInlineKeyboardButtonData("6", "ip_limit_c "+email+" 6"),
						tgbotapi.NewInlineKeyboardButtonData("7", "ip_limit_c "+email+" 7"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("8", "ip_limit_c "+email+" 8"),
						tgbotapi.NewInlineKeyboardButtonData("9", "ip_limit_c "+email+" 9"),
						tgbotapi.NewInlineKeyboardButtonData("10", "ip_limit_c "+email+" 10"),
					),
				)
				t.editMessageCallbackTgBot(callbackQuery.From.ID, callbackQuery.Message.MessageID, inlineKeyboard)
			case "ip_limit_c":
				if len(dataArray) == 3 {
					count, err := strconv.Atoi(dataArray[2])
					if err == nil {
						err := t.inboundService.ResetClientIpLimitByEmail(email, count)
						if err == nil {
							t.xrayService.SetToNeedRestart()
							t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IP limit %d saved successfully.", email, count))
							t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
			case "clear_ips":
				var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "ips_cancel "+email),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Confirm Clear IPs?", "clear_ips_c "+email),
					),
				)
				t.editMessageCallbackTgBot(callbackQuery.From.ID, callbackQuery.Message.MessageID, inlineKeyboard)
			case "clear_ips_c":
				err := t.inboundService.ClearClientIps(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : IPs cleared successfully.", email))
					t.searchClientIps(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			case "ip_log":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Get IP Log.", email))
				t.searchClientIps(callbackQuery.From.ID, email)
			case "toggle_enable":
				enabled, err := t.inboundService.ToggleClientEnableByEmail(email)
				if err == nil {
					t.xrayService.SetToNeedRestart()
					if enabled {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Enabled successfully.", email))
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ %s : Disabled successfully.", email))
					}
					t.searchClient(callbackQuery.From.ID, email, callbackQuery.Message.MessageID)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❗ Error in Operation.")
				}
			}
			return
		}
	}

	// Respond to the callback query, telling Telegram to show the user
	// a message with the data received.
	callback := tgbotapi.NewCallback(callbackQuery.ID, callbackQuery.Data)
	if _, err := bot.Request(callback); err != nil {
		logger.Warning(err)
	}

	switch callbackQuery.Data {
	case "get_usage":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getServerUsage())
	case "inbounds":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getInboundUsages())
	case "deplete_soon":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getExhausted())
	case "get_backup":
		t.sendBackup(callbackQuery.From.ID)
	case "client_traffic":
		t.getClientUsage(callbackQuery.From.ID, callbackQuery.From.UserName, strconv.FormatInt(callbackQuery.From.ID, 10))
	case "client_commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, "To search for statistics, just use folowing command:\r\n \r\n<code>/usage [UID|Password]</code>\r\n \r\nUse UID for vmess/vless and Password for Trojan.")
	case "commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, "Search for a client email:\r\n<code>/usage email</code>\r\n \r\nSearch for inbounds (with client stats):\r\n<code>/inbound [remark]</code>")
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
	var numericKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Server Usage", "get_usage"),
			tgbotapi.NewInlineKeyboardButtonData("Get DB Backup", "get_backup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Get Inbounds", "inbounds"),
			tgbotapi.NewInlineKeyboardButtonData("Deplete soon", "deplete_soon"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Commands", "commands"),
		),
	)
	var numericKeyboardClient = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Get Usage", "client_traffic"),
			tgbotapi.NewInlineKeyboardButtonData("Commands", "client_commands"),
		),
	)
	msgConfig := tgbotapi.NewMessage(chatId, msg)
	msgConfig.ParseMode = "HTML"
	if isAdmin {
		msgConfig.ReplyMarkup = numericKeyboard
	} else {
		msgConfig.ReplyMarkup = numericKeyboardClient
	}
	_, err := bot.Send(msgConfig)
	if err != nil {
		logger.Warning("Error sending telegram message :", err)
	}
}

func (t *Tgbot) SendMsgToTgbot(tgid int64, msg string, inlineKeyboard ...tgbotapi.InlineKeyboardMarkup) {
	if !isRunning {
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
		info := tgbotapi.NewMessage(tgid, message)
		info.ParseMode = "HTML"
		if len(inlineKeyboard) > 0 {
			info.ReplyMarkup = inlineKeyboard[0]
		}
		_, err := bot.Send(info)
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

func (t *Tgbot) getServerUsage() string {
	var info string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		name = ""
	}
	info = fmt.Sprintf("💻 Hostname: %s\r\n", name)
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
	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed,invalid info")
		return
	}
	var msg string
	// Get hostname
	name, err := os.Hostname()
	if err != nil {
		logger.Warning("get hostname error:", err)
		return
	}
	if status == LoginSuccess {
		msg = fmt.Sprintf("✅ Successfully logged-in to the panel\r\nHostname:%s\r\n", name)
	} else if status == LoginFail {
		msg = fmt.Sprintf("❗ Login to the panel was unsuccessful\r\nHostname:%s\r\n", name)
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
	var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "ips_refresh "+email),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Clear IPs", "clear_ips "+email),
		),
	)
	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
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
	var inlineKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "client_refresh "+email),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📈 Reset Traffic", "reset_traffic "+email),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Reset Expire Days", "reset_exp "+email),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔢 IP Log", "ip_log "+email),
			tgbotapi.NewInlineKeyboardButtonData("🔢 IP Limit", "ip_limit "+email),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔘 Enable / Disable", "toggle_enable "+email),
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
	sendingTime := time.Now().Format("2006-01-02 15:04:05")
	t.SendMsgToTgbot(chatId, "Backup time: "+sendingTime)
	file := tgbotapi.FilePath(config.GetDBPath())
	msg := tgbotapi.NewDocument(chatId, file)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Warning("Error in uploading backup: ", err)
	}
	file = tgbotapi.FilePath(xray.GetConfigPath())
	msg = tgbotapi.NewDocument(chatId, file)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Warning("Error in uploading config.json: ", err)
	}
}

func (t *Tgbot) sendCallbackAnswerTgBot(id string, message string) {
	callback := tgbotapi.NewCallback(id, message)
	if _, err := bot.Request(callback); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageCallbackTgBot(chatId int64, messageID int, inlineKeyboard tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatId, messageID, inlineKeyboard)
	if _, err := bot.Request(edit); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageTgBot(chatId int64, messageID int, text string, inlineKeyboard ...tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatId, messageID, text)
	edit.ParseMode = "HTML"
	if len(inlineKeyboard) > 0 {
		edit.ReplyMarkup = &inlineKeyboard[0]
	}
	if _, err := bot.Request(edit); err != nil {
		logger.Warning(err)
	}
}
