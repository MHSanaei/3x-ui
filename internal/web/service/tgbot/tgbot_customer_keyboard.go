package tgbot

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// clientKeyboard is the keyboard a linked customer sees. It is extracted from
// SendAnswer so the entry points it carries can be asserted on directly rather
// than by reading a sent message.
func (t *Tgbot) clientKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientUsage")).WithCallbackData(t.encodeQuery("client_traffic")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("client_commands")),
			// The door to the customer menu. Without it the menu, the one-link
			// picker and the setup guides have no entry point at all.
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.customerMenu")).WithCallbackData(t.encodeQuery("client_menu_open")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("pages.settings.subSettings")).WithCallbackData(t.encodeQuery("client_sub_links")),
			tu.InlineKeyboardButton(t.I18nBot("subscription.individualLinks")).WithCallbackData(t.encodeQuery("client_individual_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("qrCode")).WithCallbackData(t.encodeQuery("client_qr_links")),
		),
	)
}
