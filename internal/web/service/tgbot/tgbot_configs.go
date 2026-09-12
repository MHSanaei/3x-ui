package tgbot

import (
	"fmt"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// One door for everything that hands out or rotates a config, so a customer
// never has to guess which of three top-level buttons means "give me my link".
func (t *Tgbot) configsKeyboard() *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getAllConfigs")).WithCallbackData(t.encodeQuery("client_individual_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getOneConfig")).WithCallbackData(t.encodeQuery("client_one_link")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.subLink")).WithCallbackData(t.encodeQuery("client_sub_links")),
		),
	}
	// Offered only where an operator opted in, so a customer is never shown a
	// destructive button that would refuse them.
	if t.selfResetEnabled() {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.selfReset")).WithCallbackData(t.encodeQuery("client_reset_self")),
		))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
	))
	return tu.InlineKeyboardGrid(rows)
}

func (t *Tgbot) configsMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.configsChoose"), t.configsKeyboard())
}

// Asks which config before sending anything, so "Get one" on a client attached
// to six inbounds does not turn into the same wall of links as "Get all".
func (t *Tgbot) oneLinkPicker(chatId int64, email string) {
	cleaned, err := t.fetchIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if len(cleaned) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}
	if len(cleaned) == 1 {
		t.sendOneLink(chatId, email, 0)
		return
	}

	buttons := make([]telego.InlineKeyboardButton, 0, len(cleaned))
	for i, link := range cleaned {
		buttons = append(buttons, tu.InlineKeyboardButton(linkLabel(link, i)).
			WithCallbackData(t.encodeQuery(fmt.Sprintf("link_one %s %d", email, i))))
	}
	rows := tu.InlineKeyboardCols(1, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_configs")),
	))
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.chooseOneConfig"), tu.InlineKeyboardGrid(rows))
}

// The list is re-fetched rather than cached, so a stale button from an older
// message cannot send a link that has since changed or gone.
func (t *Tgbot) sendOneLink(chatId int64, email string, index int) {
	cleaned, err := t.fetchIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if index < 0 || index >= len(cleaned) {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	link := cleaned[index]
	msg := t.clientHeader(email) + linkLabel(link, index) + ":\r\n<code>" + link + "</code>"
	t.SendMsgToTgbot(chatId, msg, t.oneLinkKeyboard(email, index))
}

// The QR rides under the link it encodes and carries that link's own index, so
// tapping it cannot ask which config a second time.
func (t *Tgbot) oneLinkKeyboard(email string, index int) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrCode")).
				WithCallbackData(t.encodeQuery(fmt.Sprintf("qr_one %s %d", email, index))),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_configs")),
		),
	)
}
