package tgbot

import (
	"html"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	statePmText       = "awaiting_pm_text"
	stateReplyPrefix  = "awaiting_reply:"
	stateRosterSearch = "awaiting_roster_search"
)

// The reply target rides in the state string so a single map entry survives the
// admin typing an unrelated message in between.
func parseReplyTarget(state string) (int64, bool) {
	if !strings.HasPrefix(state, stateReplyPrefix) {
		return 0, false
	}
	target, err := strconv.ParseInt(strings.TrimPrefix(state, stateReplyPrefix), 10, 64)
	if err != nil || target == 0 {
		return 0, false
	}
	return target, true
}

// Conversational flows added on top of the add-client wizard's states. Returns
// true when the state was consumed so the caller leaves its own switch alone.
func (t *Tgbot) handleConversationState(message *telego.Message, state string) bool {
	chatId := message.Chat.ID
	text := strings.TrimSpace(message.Text)

	switch {
	case state == stateBroadcast:
		userStateMgr.clear(chatId)
		if !checkAdmin(message.From.ID) {
			return true
		}
		t.previewBroadcast(chatId, text)
		return true
	case state == stateRosterSearch:
		userStateMgr.clear(chatId)
		if !checkAdmin(message.From.ID) {
			return true
		}
		t.rosterSearchResults(chatId, text)
		return true
	case state == stateHelpText:
		userStateMgr.clear(chatId)
		if !checkAdmin(message.From.ID) {
			return true
		}
		t.saveHelpText(chatId, text)
		return true
	case strings.HasPrefix(state, stateEditEmailPrefix):
		userStateMgr.clear(chatId)
		target, ok := stateTarget(state, stateEditEmailPrefix)
		if !ok || !checkAdmin(message.From.ID) {
			return true
		}
		t.applyEmailEdit(chatId, target, text)
		return true
	case strings.HasPrefix(state, stateEditCommentPrefix):
		userStateMgr.clear(chatId)
		target, ok := stateTarget(state, stateEditCommentPrefix)
		if !ok || !checkAdmin(message.From.ID) {
			return true
		}
		t.applyCommentEdit(chatId, target, text)
		return true
	case state == statePmText:
		userStateMgr.clear(chatId)
		t.forwardClientMessage(message, text)
		return true
	case strings.HasPrefix(state, stateReplyPrefix):
		userStateMgr.clear(chatId)
		target, ok := parseReplyTarget(state)
		if !ok || !checkAdmin(message.From.ID) {
			return true
		}
		t.deliverAdminReply(chatId, target, text)
		return true
	}
	return false
}

// The router already took the state, so /cancel only names what was
// interrupted: the sender's real question is which side was spared the message.
func cancelledKey(pending string) string {
	switch {
	case pending == statePmText:
		return "tgbot.messages.pmCancelled"
	case strings.HasPrefix(pending, stateReplyPrefix):
		return "tgbot.messages.replyCancelled"
	case pending == "":
		return "tgbot.messages.cancelNothing"
	}
	return "tgbot.messages.cancelled"
}

// Both prompts have to advertise the way out, so the hint is appended rather
// than baked into each prompt's translation.
func (t *Tgbot) promptWithCancel(key string) string {
	return t.I18nBot(key) + t.I18nBot("tgbot.messages.cancelHint")
}

// Only clients already bound to a Telegram account may message admins, which
// keeps the admin inbox free of traffic from arbitrary strangers.
func (t *Tgbot) clientEmailsFor(tgUserID int64) []string {
	records, err := t.clientService.GetRecordsByTgID(tgUserID)
	if err != nil || len(records) == 0 {
		return nil
	}
	emails := make([]string, 0, len(records))
	for _, r := range records {
		emails = append(emails, r.Email)
	}
	return emails
}

func (t *Tgbot) startClientMessage(message *telego.Message, text string) {
	chatId := message.Chat.ID
	if len(t.clientEmailsFor(message.From.ID)) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmNotLinked"))
		return
	}
	if text == "" {
		userStateMgr.set(chatId, statePmText)
		t.SendMsgToTgbot(chatId, t.promptWithCancel("tgbot.messages.pmPrompt"))
		return
	}
	t.forwardClientMessage(message, text)
}

// The button has no text to carry, so it always opens the prompt rather than
// sending anything.
func (t *Tgbot) promptClientMessage(chatId int64, tgUserID int64) {
	if len(t.clientEmailsFor(tgUserID)) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmNotLinked"))
		return
	}
	userStateMgr.set(chatId, statePmText)
	t.SendMsgToTgbot(chatId, t.promptWithCancel("tgbot.messages.pmPrompt"))
}

func (t *Tgbot) forwardClientMessage(message *telego.Message, text string) {
	chatId := message.Chat.ID
	emails := t.clientEmailsFor(message.From.ID)
	if len(emails) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmNotLinked"))
		return
	}
	if text == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pmUsage"))
		return
	}

	name := html.EscapeString(message.From.FirstName)
	if message.From.Username != "" {
		name += " (@" + html.EscapeString(message.From.Username) + ")"
	}
	header := t.I18nBot("tgbot.messages.pmHeader", "Name=="+name, "Clients=="+html.EscapeString(strings.Join(emails, ", ")))

	replyKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.replyToClient")).
				WithCallbackData(t.encodeQuery("pm_reply " + strconv.FormatInt(chatId, 10))),
		),
	)
	for _, adminId := range adminIds {
		t.SendMsgToTgbot(adminId, header+"\r\n"+html.EscapeString(text), replyKeyboard)
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmSent"))
}

func (t *Tgbot) promptAdminReply(chatId int64, target string) {
	if _, err := strconv.ParseInt(target, 10, 64); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.sendUsage"))
		return
	}
	userStateMgr.set(chatId, stateReplyPrefix+target)
	t.SendMsgToTgbot(chatId, t.promptWithCancel("tgbot.messages.replyPrompt"))
}

func (t *Tgbot) deliverAdminReply(adminChatId int64, target int64, text string) {
	if text == "" {
		t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.commands.sendUsage"))
		return
	}
	if err := t.sendDirect(target, text); err != nil {
		t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.messages.replyFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.messages.replyDelivered"))
}
