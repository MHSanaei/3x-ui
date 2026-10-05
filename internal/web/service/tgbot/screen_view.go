package tgbot

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Every screen's keyboard is built here, one builder per screen: the old code
// assembled keyboards inside the router's callback cases, where a row could be
// edited without the screen it belongs to noticing.

const (
	cbHome       = "home"
	cbActions    = "act"
	cbHide       = "hide"
	cbPageNext   = "pg:next"
	cbPagePrev   = "pg:prev"
	cbPageNone   = "pg:none"
	cbServer     = "srv"
	cbInbounds   = "inb"
	cbClients    = "cli"
	cbOnlines    = "onl"
	cbDeplete    = "dep"
	cbReport     = "rep"
	cbBackup     = "bkp"
	cbBanLogs    = "ban"
	cbAddClient  = "addc"
	cbResetAll   = "rst_all"
	cbRestartXry = "xray_restart"
)

// homeRows is the admin's main menu: the panels you actually open, then the
// maintenance actions people kept reaching for slash commands to get.
func (t *Tgbot) homeRows() [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.serverUsage", cbServer),
			t.btn("tgbot.buttons.getInbounds", cbInbounds),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.allClients", cbClients),
			t.btn("tgbot.buttons.onlines", cbOnlines),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.depleteSoon", cbDeplete),
			t.btn("tgbot.buttons.SortedTrafficUsageReport", cbReport),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.dbBackup", cbBackup),
			t.btn("tgbot.buttons.getBanLogs", cbBanLogs),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.addClient", cbAddClient),
			t.btn("tgbot.buttons.actions", cbActions),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.refresh", cbHome),
		),
	}
}

// actionsRows holds the heavier, rarer operations, kept one tap away from the
// menu so a misfire cannot reach them.
func (t *Tgbot) actionsRows() [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.restartXray", cbRestartXry),
			t.btn("tgbot.buttons.ResetAllTraffics", cbResetAll),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.back", cbHome),
		),
	}
}

// btn is the one place a localized label becomes a callback button.
func (t *Tgbot) btn(labelKey, data string) telego.InlineKeyboardButton {
	return tu.InlineKeyboardButton(t.I18nBot(labelKey)).WithCallbackData(data)
}

// hideRow is appended to every message that is not a screen: it gives the user
// a way to clear a notification without hunting the bot's own menu.
func (t *Tgbot) hideRow() []telego.InlineKeyboardButton {
	return tu.InlineKeyboardRow(t.btn("tgbot.buttons.hide", cbHide))
}

// backRow is the single "up" control a non-root screen carries.
func (t *Tgbot) backRow() []telego.InlineKeyboardButton {
	return tu.InlineKeyboardRow(t.btn("tgbot.buttons.back", cbHome))
}
