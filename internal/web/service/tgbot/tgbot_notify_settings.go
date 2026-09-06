package tgbot

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Who a notice reaches. Every row is marked so an admin can tell which switch
// silences their own inbox from the one that silences the customer's.
type notifyAudience string

const (
	audienceNone     notifyAudience = ""
	audienceAdmin    notifyAudience = "👑"
	audienceCustomer notifyAudience = "👤"
)

// One notice the admin can switch off. Read and write stay together so a new
// notice cannot be listed in the menu without being wired to its setting.
type botNotification struct {
	callback string
	labelKey string
	audience notifyAudience
	get      func(*Tgbot) (bool, error)
	set      func(*Tgbot, bool) error
}

// Listed admin-first so the two audiences read as blocks rather than as marks
// the eye has to pick out row by row.
var botNotifications = []botNotification{
	{
		callback: "notify_usage",
		labelKey: "tgbot.buttons.notifyServerUsage",
		audience: audienceAdmin,
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyServerUsage() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyServerUsage(v) },
	},
	{
		callback: "notify_deplete",
		labelKey: "tgbot.buttons.notifyDepleteSoon",
		audience: audienceAdmin,
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyDepleteSoon() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyDepleteSoon(v) },
	},
	{
		callback: "notify_new_client",
		labelKey: "tgbot.buttons.notifyNewClient",
		audience: audienceAdmin,
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyNewClient() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyNewClient(v) },
	},
	{
		callback: "notify_backup",
		labelKey: "tgbot.buttons.notifyBackup",
		audience: audienceAdmin,
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotBackup() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotBackup(v) },
	},
	{
		callback: "notify_quota",
		labelKey: "tgbot.buttons.notifyQuota",
		audience: audienceCustomer,
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyQuota() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyQuota(v) },
	},
}

// Feature switches share the toggle machinery but not the fail-open default:
// selfResetEnabled reads its own setting and fails closed.
var botFeatures = []botNotification{
	{
		callback: "feature_self_reset",
		labelKey: "tgbot.buttons.allowSelfReset",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotAllowSelfReset() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotAllowSelfReset(v) },
	},
	{
		callback: "feature_silent",
		labelKey: "tgbot.buttons.silentNotices",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotSilentNotices() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotSilentNotices(v) },
	},
}

func toggleByCallback(toggles []botNotification, callback string) (botNotification, bool) {
	for _, toggle := range toggles {
		if toggle.callback == callback {
			return toggle, true
		}
	}
	return botNotification{}, false
}

// A lookup failure shows the notice as on, matching SendReport's fail-open read:
// the menu must not claim a notice is off while the job still sends it.
func (t *Tgbot) notificationToggleLabel(notification botNotification) string {
	enabled, err := notification.get(t)
	if err != nil {
		logger.Warning("tgbot: notification setting lookup failed:", err)
		enabled = true
	}
	mark := "❌"
	if enabled {
		mark = "✅"
	}
	if notification.audience == audienceNone {
		return mark + " " + t.I18nBot(notification.labelKey)
	}
	return mark + " " + string(notification.audience) + " " + t.I18nBot(notification.labelKey)
}

func (t *Tgbot) toggleKeyboard(toggles []botNotification, prefix string) *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(toggles)+1)
	for _, toggle := range toggles {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.notificationToggleLabel(toggle)).
				WithCallbackData(t.encodeQuery(prefix+toggle.callback)),
		))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_settings")),
	))
	return tu.InlineKeyboard(rows...)
}

func (t *Tgbot) notificationsKeyboard() *telego.InlineKeyboardMarkup {
	return t.toggleKeyboard(botNotifications, "notify_toggle ")
}

func (t *Tgbot) featuresKeyboard() *telego.InlineKeyboardMarkup {
	return t.toggleKeyboard(botFeatures, "feature_toggle ")
}

func (t *Tgbot) notificationsMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.notifications"), t.notificationsKeyboard())
}

func (t *Tgbot) featuresMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.botFeatures"), t.featuresKeyboard())
}

func (t *Tgbot) toggleNotification(chatId int64, callback string, messageID int) {
	t.applyToggle(chatId, botNotifications, callback, messageID, t.notificationsKeyboard, t.notificationsMenu)
}

func (t *Tgbot) toggleFeature(chatId int64, callback string, messageID int) {
	t.applyToggle(chatId, botFeatures, callback, messageID, t.featuresKeyboard, t.featuresMenu)
}

func (t *Tgbot) applyToggle(
	chatId int64,
	toggles []botNotification,
	callback string,
	messageID int,
	keyboard func() *telego.InlineKeyboardMarkup,
	menu func(int64),
) {
	toggle, found := toggleByCallback(toggles, callback)
	if !found {
		return
	}
	enabled, err := toggle.get(t)
	if err != nil {
		logger.Warning("tgbot: toggle lookup failed:", err)
		enabled = true
	}
	if err := toggle.set(t, !enabled); err != nil {
		logger.Warning("tgbot: toggle save failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if messageID > 0 {
		t.editMessageCallbackTgBot(chatId, messageID, keyboard())
		return
	}
	menu(chatId)
}
