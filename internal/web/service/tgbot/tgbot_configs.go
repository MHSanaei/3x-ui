package tgbot

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// A subscription link carries the client's remark after "#", so the picker can
// name each one instead of listing eight identical "vless://" strings.
func clientLinkLabel(link string, index int) string {
	fallback := fmt.Sprintf("%d. %s", index+1, strings.ToUpper(strings.SplitN(link, "://", 2)[0]))
	hash := strings.LastIndex(link, "#")
	if hash < 0 || hash+1 >= len(link) {
		return fallback
	}
	remark, err := url.QueryUnescape(strings.TrimSpace(link[hash+1:]))
	if err != nil || remark == "" {
		return fallback
	}
	return fmt.Sprintf("%d. %s", index+1, remark)
}

// configsMenu is the "how do I get my config" step, asked once per client so a
// customer is never handed a wall of links when they only wanted the URL.
func (t *Tgbot) configsMenu(chatId int64, email string) {
	t.SendMsgToTgbot(chatId, t.clientHeader(email)+t.I18nBot("tgbot.messages.configsChoose"), t.configsKeyboard(email))
}

// One door for everything that hands out a config, so a customer never has to
// guess which of three buttons means "give me my link".
func (t *Tgbot) configsKeyboard(email string) *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getAllLinks")).WithCallbackData(t.encodeQuery("client_individual_links " + email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getOneConfig")).WithCallbackData(t.encodeQuery("client_one_link " + email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrCodes")).WithCallbackData(t.encodeQuery("client_qr_links " + email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
		),
	}
	return tu.InlineKeyboardGrid(rows)
}

// The links are built in-process rather than fetched from the subscription URL,
// which need not resolve on this host. Telegram only accepts http, https and
// tg:// in link buttons, so a guide is plain text and never a tappable link.
func (t *Tgbot) clientIndividualLinks(email string) ([]string, error) {
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		return nil, err
	}
	return t.clientSubLinks(email, subURL)
}

// Asking first keeps "Get one" on a client spanning six inbounds from dumping
// the same wall of links that "Get all" would.
func (t *Tgbot) oneLinkPicker(chatId int64, email string) {
	links, err := t.clientIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if len(links) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}
	if len(links) == 1 {
		t.sendOneLink(chatId, email, 0)
		return
	}

	buttons := make([]telego.InlineKeyboardButton, 0, len(links))
	for i, link := range links {
		buttons = append(buttons, tu.InlineKeyboardButton(clientLinkLabel(link, i)).
			WithCallbackData(t.encodeQuery(fmt.Sprintf("link_one %s %d", email, i))))
	}
	rows := tu.InlineKeyboardCols(1, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
	))
	t.SendMsgToTgbot(chatId, t.clientHeader(email)+t.I18nBot("tgbot.messages.chooseOneConfig"), tu.InlineKeyboardGrid(rows))
}

// The list is re-read on every tap, so a button from an older message cannot
// return an index that now points at a different link.
func (t *Tgbot) sendOneLink(chatId int64, email string, index int) {
	links, err := t.clientIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if index < 0 || index >= len(links) {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}
	link := links[index]
	msg := t.clientHeader(email) + clientLinkLabel(link, index) + ":\r\n<code>" + html.EscapeString(link) + "</code>"
	t.SendMsgToTgbot(chatId, msg, t.configsKeyboard(email))
}
