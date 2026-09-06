package tgbot

import (
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	stateEditEmailPrefix   = "awaiting_edit_email:"
	stateEditCommentPrefix = "awaiting_edit_comment:"
	// Telegram cannot deliver an empty message body, so a lone dash is the way
	// an admin says "clear this field".
	fieldClearToken = "-"
)

// The client being edited rides in the state string, so an admin who starts an
// edit and then types something unrelated cannot have it applied to the client.
func stateTarget(state, prefix string) (string, bool) {
	if !strings.HasPrefix(state, prefix) {
		return "", false
	}
	target := strings.TrimSpace(strings.TrimPrefix(state, prefix))
	return target, target != ""
}

func normalizeFieldValue(text string) string {
	if text == fieldClearToken {
		return ""
	}
	return text
}

// Edits reuse the panel's record→Update path so duplicate-email detection, subId
// validation and the multi-inbound fan-out keep living in exactly one place.
func (t *Tgbot) editClientRecord(email string, mutate func(*model.Client)) error {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	updated := record.ToClient()
	mutate(updated)
	needRestart, err := t.clientService.Update(&t.inboundService, record.Id, *updated, record.LimitHwid)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	return err
}

func (t *Tgbot) clientEditMenu(chatId int64, email string, messageID int) {
	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData(t.encodeQuery("client_edit_email "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_comment")).WithCallbackData(t.encodeQuery("client_edit_comment "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.newSubID")).WithCallbackData(t.encodeQuery("client_new_subid "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
		),
	)
	t.editMessageCallbackTgBot(chatId, messageID, inlineKeyboard)
}

func (t *Tgbot) promptClientEdit(chatId int64, email, statePrefix, promptKey string) {
	userStateMgr.set(chatId, statePrefix+email)
	t.SendMsgToTgbot(chatId, t.I18nBot(promptKey, "Email=="+email))
}

func (t *Tgbot) applyEmailEdit(chatId int64, oldEmail, newEmail string) {
	newEmail = strings.TrimSpace(newEmail)
	if newEmail == "" || newEmail == oldEmail {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.incorrect_input"))
		return
	}
	if err := t.editClientRecord(oldEmail, func(c *model.Client) { c.Email = newEmail }); err != nil {
		logger.Warning("tgbot: client rename failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientEditFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientRenamed", "Old=="+oldEmail, "New=="+newEmail))
	t.searchClient(chatId, newEmail)
}

func (t *Tgbot) applyCommentEdit(chatId int64, email, comment string) {
	comment = normalizeFieldValue(strings.TrimSpace(comment))
	if err := t.editClientRecord(email, func(c *model.Client) { c.Comment = comment }); err != nil {
		logger.Warning("tgbot: client comment edit failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientEditFailed", "Error=="+err.Error()))
		return
	}
	t.searchClient(chatId, email)
}

// A leaked subscription URL can only be revoked by issuing a new subId, which
// also invalidates the client's invite link because the two share the value.
func (t *Tgbot) regenerateSubID(chatId int64, email string, messageID ...int) {
	subID := uuid.NewString()
	if err := t.editClientRecord(email, func(c *model.Client) { c.SubID = subID }); err != nil {
		logger.Warning("tgbot: subId regeneration failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientEditFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.subIDRegenerated", "Email=="+email))
	t.searchClient(chatId, email, messageID...)
}

func (t *Tgbot) deleteClient(chatId int64, email string) {
	needRestart, err := t.clientService.DeleteByEmail(&t.inboundService, email, false)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("tgbot: client deletion failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientEditFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientDeleted", "Email=="+email))
}

func (t *Tgbot) deleteDepletedClients(chatId int64) {
	deleted, needRestart, err := t.clientService.DelDepleted(&t.inboundService)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("tgbot: depleted cleanup failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.clientEditFailed", "Error=="+err.Error()))
		return
	}
	if deleted == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.depletedNone"))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.depletedDeleted", "Count=="+strconv.Itoa(deleted)))
}

// Answers "which configs does this Telegram account hold?", the lookup an admin
// needs when a customer writes in without naming their client.
func (t *Tgbot) whoIs(chatId int64, arg string) {
	tgID, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(arg, "@")), 10, 64)
	if err != nil || tgID == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.whoisUsage"))
		return
	}
	records, err := t.clientService.GetRecordsByTgID(tgID)
	if err != nil {
		logger.Warning("tgbot: whois lookup failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if len(records) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.whoisEmpty", "ID=="+strconv.FormatInt(tgID, 10)))
		return
	}

	header := t.I18nBot("tgbot.messages.whoisHeader", "ID=="+strconv.FormatInt(tgID, 10), "Count=="+strconv.Itoa(len(records)))
	t.SendMsgToTgbot(chatId, header, clientPickerKeyboard(t, records))
}

func clientPickerKeyboard(t *Tgbot, records []*model.ClientRecord) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(records))
	for _, record := range records {
		buttons = append(buttons, tu.InlineKeyboardButton(record.Email).WithCallbackData(t.encodeQuery("client_get_usage "+record.Email)))
	}
	return tu.InlineKeyboardGrid(tu.InlineKeyboardCols(2, buttons...))
}
