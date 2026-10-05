package tgbot

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Bridges for the screens still served by the legacy views. Each one is a
// screen now, so the router's callback cases only decide which view to draw.

// clearScreen drops the tracked screen and the user's own message: the pilot
// tap that closes the reply keyboard must leave nothing behind.
func (t *Tgbot) clearScreen(message telego.Message) {
	t.deleteIncoming(&message)
	if sc, ok := t.screens().get(message.Chat.ID); ok && sc.msgID != 0 {
		t.deleteOwnMessage(message.Chat.ID, sc.msgID)
	}
	t.screens().clear(message.Chat.ID)
}

// hideMessage deletes the very message whose button was tapped. The id comes
// from the callback, so nothing has to be remembered to make it work.
func (t *Tgbot) hideMessage(query *telego.CallbackQuery) {
	if query.Message == nil {
		return
	}
	chat := query.Message.GetChat()
	t.deleteOwnMessage(chat.ID, query.Message.GetMessageID())
	// A hidden notice must not stay the chat's screen either, and the next event
	// of its kind has to start a fresh card rather than edit a deleted one.
	if sc, ok := t.screens().get(chat.ID); ok && sc.msgID == query.Message.GetMessageID() {
		t.screens().clear(chat.ID)
	}
	t.dropNoticeFor(chat.ID, query.Message.GetMessageID())
	t.answerSilent(query.ID)
}

// turnPage moves the stored screen one body-page back or forward.
func (t *Tgbot) turnPage(query *telego.CallbackQuery, delta int) {
	chatID := query.Message.GetChat().ID
	sc, ok := t.screens().get(chatID)
	if !ok {
		t.answerSilent(query.ID)
		return
	}
	t.showPage(chatID, sc.page+delta)
	t.answerSilent(query.ID)
}

// resetAllConfirm is the confirmation screen for the panel-wide traffic reset.
func (t *Tgbot) resetAllConfirm() *screen {
	return t.newScreen("main", t.I18nBot("tgbot.messages.AreYouSure"),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.confirmResetTraffic", "reset_all_traffics_c")),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.cancel", cbHome)),
	)
}

// screenInbounds summarises the inbounds; the browsable, searchable list is the
// inline query, so the screen carries only the summary and the way back.
func (t *Tgbot) screenInbounds(chatID int64) {
	inlineScopes.set(chatID, "inb")
	body := t.getInboundUsages()
	body += t.I18nBot("tgbot.messages.inlineHint")
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow("inb", "tgbot.buttons.searchInbounds"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("inbounds", body, rows...))
}

// browseInboundsScreen is the pick-an-inbound step: the list itself is the
// inline browser, so the screen carries the hint and the search launcher.
func (t *Tgbot) browseInboundsScreen(chatID int64) {
	inlineScopes.set(chatID, "inb")
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow("inb", "tgbot.buttons.searchInbounds"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("inbounds", t.I18nBot("tgbot.answers.chooseInbound"), rows...))
}

// screenAddClientStart renders the add-client wizard's first step: which
// inbound the client lands on.
func (t *Tgbot) screenAddClientStart(chatID int64) {
	inlineScopes.set(chatID, "add")
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow("add", "tgbot.buttons.searchInbounds"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("wizard", t.I18nBot("tgbot.messages.pickInbound"), rows...))
}

// restartXrayFromScreen restarts Xray and reports the outcome as a toast, so
// the menu screen itself stays put.
func (t *Tgbot) restartXrayFromScreen(query *telego.CallbackQuery) {
	if !t.xrayService.IsXrayRunning() {
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.commands.xrayNotRunning"))
		return
	}
	if err := t.xrayService.RestartXray(true); err != nil {
		logger.Warning("Xray restart failed:", err)
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.commands.restartFailed", "Error=="+err.Error()))
		return
	}
	t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.commands.restartSuccess"))
}

// searchClientScreen renders the client card as a screen; every per-email
// callback that used to post a message lands here.
func (t *Tgbot) searchClientScreen(chatID int64, email string) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	body := t.clientInfoMsg(traffic, true, true, true, true, true, true)
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	t.renderScreen(chatID, t.newScreen("client", body, t.clientRows(email)...))
}

// clientRows is the client card's keyboard, in one place so every entry point
// draws an identical card.
func (t *Tgbot) clientRows(email string) [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", t.encodeQuery("client_refresh "+email))),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.resetTraffic", t.encodeQuery("reset_traffic "+email)),
			t.btn("tgbot.buttons.limitTraffic", t.encodeQuery("limit_traffic "+email)),
		),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.resetExpire", t.encodeQuery("reset_exp "+email))),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.ipLog", t.encodeQuery("ip_log "+email)),
			t.btn("tgbot.buttons.ipLimit", t.encodeQuery("ip_limit "+email)),
		),
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.setTGUser", t.encodeQuery("tg_user "+email)),
			t.btn("tgbot.buttons.inviteLink", t.encodeQuery("client_invite_link "+email)),
		),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.toggle", t.encodeQuery("toggle_enable "+email))),
		t.backRow(),
	}
}
