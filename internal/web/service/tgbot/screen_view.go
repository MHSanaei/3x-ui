package tgbot

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Every screen's keyboard is built here, one builder per screen: keyboards built
// inside the router's callback cases could be edited without their screen noticing.

const (
	cbHome           = "home"
	cbActions        = "act"
	cbHide           = "hide"
	cbPageNext       = "pg:next"
	cbPagePrev       = "pg:prev"
	cbPageNone       = "pg:none"
	cbServer         = "srv"
	cbInbounds       = "inb"
	cbClients        = "cli"
	cbOnlines        = "onl"
	cbDeplete        = "dep"
	cbReport         = "rep"
	cbBackup         = "bkp"
	cbBanLogs        = "ban"
	cbAddClient      = "addc"
	cbCatServer      = "cat:server"
	cbCatClients     = "cat:clients"
	cbCatTraffic     = "cat:traffic"
	cbBroadcast      = "bc"
	cbCatMaintenance = "cat:maintenance"
	cbResetAll       = "rst_all"
	cbRestartXry     = "xray_restart"
)

// homeRows is the admin's menu: four categories and a refresh, one row per
// subject, so every subject is one tap deeper and nothing is more than two away.
func (t *Tgbot) homeRows() [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.catServer", cbCatServer),
			t.btn("tgbot.buttons.catClients", cbCatClients),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.catTraffic", cbCatTraffic),
			t.btn("tgbot.buttons.catMaintenance", cbCatMaintenance),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.refresh", cbHome),
		),
	}
}

// categoryRows is one menu subject's own rows.
func (t *Tgbot) categoryRows(category string) [][]telego.InlineKeyboardButton {
	home := t.backRow()
	switch category {
	case "server":
		return [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.serverUsage", cbServer)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.getInbounds", cbInbounds)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.onlines", cbOnlines)),
			home,
		}
	case "clients":
		return [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.allClients", cbClients)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.addClient", cbAddClient)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.broadcast", cbBroadcast)),
			home,
		}
	case "traffic":
		return [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.SortedTrafficUsageReport", cbReport)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.depleteSoon", cbDeplete)),
			home,
		}
	case "maintenance":
		return [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.dbBackup", cbBackup)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.getBanLogs", cbBanLogs)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.restartXray", cbRestartXry)),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.ResetAllTraffics", cbResetAll)),
			home,
		}
	}
	return [][]telego.InlineKeyboardButton{home}
}

// screenCategory draws one menu subject. The hint line names the subject, so a
// user who arrived through a notification can see where they are.
func (t *Tgbot) screenCategory(chatID int64, category string) {
	body := t.I18nBot("tgbot.messages.category_" + category)
	rows := [][]telego.InlineKeyboardButton{}
	rows = append(rows, t.categoryRows(category)...)
	t.renderScreen(chatID, t.newScreen("main", body, rows...))
}

// btn is where a localized label becomes a callback button; btn2 is btn with a
// caller-supplied label, since a client's email names its subscription best.
func (t *Tgbot) btn2(label, data string) telego.InlineKeyboardButton {
	return tu.InlineKeyboardButton(label).WithCallbackData(data)
}

func (t *Tgbot) btn(labelKey, data string) telego.InlineKeyboardButton {
	return tu.InlineKeyboardButton(t.I18nBot(labelKey)).WithCallbackData(data)
}

// backRow is the single "up" control a non-root screen carries.
func (t *Tgbot) backRow() []telego.InlineKeyboardButton {
	return tu.InlineKeyboardRow(t.btn("tgbot.buttons.back", cbHome))
}
