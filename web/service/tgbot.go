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
		msg = "این ربات داده های خاصی را از سرور در اختیار شما قرار می دهد.\n\n لطفا انتخاب کنید:"
	case "start":
		msg = "سلام <i>" + message.From.FirstName + "</i> 👋🇮🇷"
		if isAdmin {
			hostname, _ := os.Hostname()
			msg += "\n" + hostname + "\nبه ربات مدیریت پنل x-ui خوش آمدید"
		}
		msg += "\n\nکارهایی که از دستم بر میاد فعلا اینان یکی رو انتخاب کن: :"
	case "status":
		msg = "ربات در حال اجرا است: ✅"
	case "usage":
		if len(message.CommandArguments()) > 1 {
			if isAdmin {
				t.searchClient(chatId, message.CommandArguments())
			} else {
				t.searchForClient(chatId, message.CommandArguments())
			}
		} else {
			msg = "لطفا یکی از گزینه های زیر را انتخاب کنید:"
		}
	case "inbound":
		if isAdmin {
			t.searchInbound(chatId, message.CommandArguments())
		} else {
			msg = "❗ دستور نامفهوم"
		}
	default:
		msg = "❗ دستور نامفهوم"
	}
	t.SendAnswer(chatId, msg, isAdmin)
}

func (t *Tgbot) asnwerCallback(callbackQuery *tgbotapi.CallbackQuery, isAdmin bool) {
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
		t.getClientUsage(callbackQuery.From.ID, callbackQuery.From.UserName)
	case "client_commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, "برای اینکه ببینی چه قد مصرف داشتی و اینکه کی تموم میشه مقدار uuid کانفیگ رو بعد از \n/usage\nبزار :\n👇👇👇👇👇\n\n/usage [UID|Passowrd]\n\nمقدار UUID رو برای کانفیگهای vmess و vless بزار و مقدار پسورد هم برای کانفیگ تروجان بزار  . کانفیگ های تروجان فقط پسورد- کانفیگ های vmess و vless فقط uuid بزار.\n\nمثال برای vmess و vless به صورت زیر:\n\n/usage d0cadb19-390c-4041-a3bd-def4b969cadf\n\nبرای تروجان به صورت زیر:\n\n/usage u*ug5wix65sv")
	case "commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, "جستجو برای مصرف کاربر توسط ایمیل:\r\n<code>/usage email</code>\r\n<code>example :  /usage P_Tech </code>\r\nجستجو برای مصرف کاربر با اسمی که برای کانفیگ گذاشتی:\r\n<code>/inbound [remark]</code>\r\n<code>example:  /inbound Peyman </code>")
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
			tgbotapi.NewInlineKeyboardButtonData("وضعیت سرور", "get_usage"),
			tgbotapi.NewInlineKeyboardButtonData("دریافت بک آپ", "get_backup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("وضعیت کاربران", "inbounds"),
			tgbotapi.NewInlineKeyboardButtonData("کاربران رو به اتمام", "deplete_soon"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("دستورات", "commands"),
		),
	)
	var numericKeyboardClient = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("وضعیت من 1", "client_traffic"),
			tgbotapi.NewInlineKeyboardButtonData("وضعیت من 2", "client_commands"),
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

func (t *Tgbot) SendMsgToTgbot(tgid int64, msg string) {
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
		t.SendMsgToTgbotAdmins("🕰 گزارش های برنامه ریزی شده: " + runTime + "\r\nDate-Time: " + time.Now().Format("2006-01-02 15:04:05"))
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
	info = fmt.Sprintf("💻 نام سرور: %s\r\n", name)
	info += fmt.Sprintf("🚀ورژن پنل: %s\r\n", config.GetVersion())
	//get ip address
	var ip string
	var ipv6 string
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err:", err.Error())
		info += "🌐 آی پی: ناشناخته\r\n \r\n"
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
		info += fmt.Sprintf("🌐آی پی ورژن 4 شما: %s\r\n🌐آی پی ورژن6 شما: %s\r\n", ip, ipv6)
	}

	// get latest status of server
	t.lastStatus = t.serverService.GetStatus(t.lastStatus)
	info += fmt.Sprintf("🔌زمان فعالیت سرور : %d روز\r\n", int(t.lastStatus.Uptime/86400))
	info += fmt.Sprintf("📈لود سرور: %.1f, %.1f, %.1f\r\n", t.lastStatus.Loads[0], t.lastStatus.Loads[1], t.lastStatus.Loads[2])
	info += fmt.Sprintf("📋حافظه سرور: %s/%s\r\n", common.FormatTraffic(int64(t.lastStatus.Mem.Current)), common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += fmt.Sprintf("🔹TcpCount: %d\r\n", t.lastStatus.TcpCount)
	info += fmt.Sprintf("🔸UdpCount: %d\r\n", t.lastStatus.UdpCount)
	info += fmt.Sprintf("🚦مصرف: %s (↑%s,↓%s)\r\n", common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += fmt.Sprintf("ℹXray وضعیت: %s", t.lastStatus.Xray.State)

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
		msg = fmt.Sprintf("✅ ورود موفقیت آمیز به پنل\r\nنام سرور:%s\r\n", name)
	} else if status == LoginFail {
		msg = fmt.Sprintf("❗ ورود به پنل ناموفق بود\r\nنام سرور:%s\r\n", name)
	}
	msg += fmt.Sprintf("⏰ زمان:%s\r\n", time)
	msg += fmt.Sprintf("🆔 نام کاربری:%s\r\n", username)
	msg += fmt.Sprintf("🌐 آی پی:%s\r\n", ip)
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
			info += fmt.Sprintf("📍کاربر:%s\r\nپورت:%d\r\n", inbound.Remark, inbound.Port)
			info += fmt.Sprintf("مصرف: %s (↑%s,↓%s)\r\n", common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				info += "انقضا: ♾ نامحدود\r\n \r\n"
			} else {
				info += fmt.Sprintf("انقضا:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}
	return info
}

func (t *Tgbot) getClientUsage(chatId int64, tgUserName string) {
	if len(tgUserName) == 0 {
		msg := "کانفیگ شما یافت نشد!\n به ادمین بگید که یوزرنیم تلگرامتون رو به کانفیگ اضافه کنه."
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserName)
	if err != nil {
		logger.Warning(err)
		msg := "❌ Something went wrong!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(traffics) == 0 {
		msg := "اک که هی 😐  تنظیمات پیکر بندی شما پیدا نشد!\nلطفا از ادمین ربات بخواه که از نام کاربری تلگرامتون در ساخت کانفیگ مربوط به شما در قسمت ایمیل  استفاده کند.\n\nنام کاربری تلگرام شما: <b>@" + tgUserName + "</b>"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	for _, traffic := range traffics {
		expiryTime := ""
		if traffic.ExpiryTime == 0 {
			expiryTime = "♾نامحدود"
		} else if traffic.ExpiryTime < 0 {
			expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
		} else {
			expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
		}
		total := ""
		if traffic.Total == 0 {
			total = "♾نامحدود"
		} else {
			total = common.FormatTraffic((traffic.Total))
		}
		output := fmt.Sprintf("💡 فعال است؟: %t\r\n📧 ایمیل: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 مجموع دانلود و آپلود: %s / %s\r\n📅 تاریخ انقضا: %s\r\n",
			traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
			total, expiryTime)
		t.SendMsgToTgbot(chatId, output)
	}
	t.SendAnswer(chatId, "لطفا انتخاب کنید:", false)
}

func (t *Tgbot) searchClient(chatId int64, email string) {
	traffics, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := "❌ یه اشتباهی پیش اومده!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(traffics) == 0 {
		msg := "بدون نتیجه!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	for _, traffic := range traffics {
		expiryTime := ""
		if traffic.ExpiryTime == 0 {
			expiryTime = "♾نامحدود"
		} else if traffic.ExpiryTime < 0 {
			expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
		} else {
			expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
		}
		total := ""
		if traffic.Total == 0 {
			total = "♾نامحدود "
		} else {
			total = common.FormatTraffic((traffic.Total))
		}
		output := fmt.Sprintf("💡 فعال است؟: %t\r\n📧 ایمیل: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 مجموع دانلود و آپلود: %s / %s\r\n📅 تاریخ انقضا: %s\r\n",
			traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
			total, expiryTime)
		t.SendMsgToTgbot(chatId, output)
	}
}

func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbouds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := "❌ یه اشتباهی پیش اومده!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	for _, inbound := range inbouds {
		info := ""
		info += fmt.Sprintf("📍کاربر:%s\r\nپورت:%d\r\n", inbound.Remark, inbound.Port)
		info += fmt.Sprintf("مصرف: %s (↑%s,↓%s)\r\n", common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
		if inbound.ExpiryTime == 0 {
			info += "انقضا: ♾ نامحدود\r\n \r\n"
		} else {
			info += fmt.Sprintf("انقضا:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)
		for _, traffic := range inbound.ClientStats {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = "♾نامحدود"
			} else if traffic.ExpiryTime < 0 {
				expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}
			total := ""
			if traffic.Total == 0 {
				total = "♾نامحدود"
			} else {
				total = common.FormatTraffic((traffic.Total))
			}
			output := fmt.Sprintf("💡 فعال است؟: %t\r\n📧 ایمیل: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 مجموع دانلود و آپلود: %s / %s\r\n📅 تاریخ انقضا: %s\r\n",
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
		msg := "❌ یه اشتباهی پیش اومده!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := "بدون نتیجه!"
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = "♾نامحدود"
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
	} else {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	}
	total := ""
	if traffic.Total == 0 {
		total = "♾نامحدود"
	} else {
		total = common.FormatTraffic((traffic.Total))
	}
	output := fmt.Sprintf("💡 فعال است؟: %t\r\n📧 ایمیل: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 مجموع دانلود و آپلود: %s / %s\r\n📅 تاریخ انقضا: %s\r\n",
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
	output += fmt.Sprintf("تعداد کل کاربران غیر فعال:\r\n🛑 غیرفعال شده: %d\r\n🔜 در آستانه ی غیر فعال شدن: %d\r\n \r\n", len(disabledInbounds), len(exhaustedInbounds))
	if len(exhaustedInbounds) > 0 {
		output += "کاربران در آستانه غیر فعال شدن:\r\n"
		for _, inbound := range exhaustedInbounds {
			output += fmt.Sprintf("📍کاربر:%s\r\nپورت:%d\r\nمصرف: %s (↑%s,↓%s)\r\n", inbound.Remark, inbound.Port, common.FormatTraffic((inbound.Up + inbound.Down)), common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				output += "تاریخ انقضا: ♾نامحدود\r\n \r\n"
			} else {
				output += fmt.Sprintf("تاریخ انقضا:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}
	output += fmt.Sprintf("تعداد کل کلاینتهای غیرفعال:\r\n🛑 غیرفعال شده: %d\r\n🔜 در آستانه ی غیر فعال شدن: %d\r\n \r\n", len(disabledClients), len(exhaustedClients))
	if len(exhaustedClients) > 0 {
		output += "کلاینت های در آستانه ی غیر فعال شدن:\r\n"
		for _, traffic := range exhaustedClients {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = "♾نامحدود"
			} else if traffic.ExpiryTime < 0 {
				expiryTime += fmt.Sprintf("%d days", traffic.ExpiryTime/-86400000)
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}
			total := ""
			if traffic.Total == 0 {
				total = "♾نامحدود"
			} else {
				total = common.FormatTraffic((traffic.Total))
			}
			output += fmt.Sprintf("💡 فعال است؟: %t\r\n📧 ایمیل: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 مجموع دانلود و آپلود: %s / %s\r\n📅 تاریخ انقضا: %s\r\n \r\n",
				traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
				total, expiryTime)
		}
	}

	return output
}

func (t *Tgbot) sendBackup(chatId int64) {
	sendingTime := time.Now().Format("2006-01-02 15:04:05")
	t.SendMsgToTgbot(chatId, "زمان گرفتن فایل بک آپ(پشتیبان): "+sendingTime)
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
