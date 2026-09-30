package tgbot

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// One door for a customer's own surface, so the buttons they are offered never
// depend on which message happened to reach them first.
func (t *Tgbot) clientMenuKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboardGrid([][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getAllLinks")).WithCallbackData(t.encodeQuery("client_one_link")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrCodes")).WithCallbackData(t.encodeQuery("client_qr_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.setupGuide")).WithCallbackData(t.encodeQuery("guide_menu")),
		),
	})
}

// A customer with nothing bound is told how to bind rather than shown an empty
// menu, since every button in it would refuse them.
func (t *Tgbot) clientMenu(chatId int64, tgUserID int64, isAdmin bool) {
	if len(t.clientEmailsFor(tgUserID)) == 0 {
		t.SendMsgToTgbot(chatId, t.noBoundClientMsg(isAdmin))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.customerMenu"), t.clientMenuKeyboard())
}

// clientConfigsStep answers "my configs" by naming the config first, so the
// buttons that follow carry an email this branch already resolved for them.
func (t *Tgbot) clientConfigsStep(chatId int64, tgUserID int64, isAdmin bool) {
	emails := t.clientEmailsFor(tgUserID)
	if len(emails) == 0 {
		t.SendMsgToTgbot(chatId, t.noBoundClientMsg(isAdmin))
		return
	}
	if len(emails) == 1 {
		t.configsMenu(chatId, emails[0])
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), t.pickerKeyboard("client_one_link", emails))
}

// One list of the customer's own configs, feeding every per-client action. The
// emails come from their Telegram id, so an auto-selected config is theirs.
func (t *Tgbot) clientEmailPicker(chatId int64, tgUserID int64, verb string, isAdmin bool) []string {
	emails := t.clientEmailsFor(tgUserID)
	if len(emails) == 0 {
		t.SendMsgToTgbot(chatId, t.noBoundClientMsg(isAdmin))
		return nil
	}
	// One config needs no question asked; a customer with several has to choose.
	if len(emails) == 1 {
		t.runClientSelfAction(chatId, verb, emails[0])
		return emails
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), t.pickerKeyboard(verb, emails))
	return emails
}

func (t *Tgbot) pickerKeyboard(verb string, emails []string) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(emails))
	for _, email := range emails {
		buttons = append(buttons, tu.InlineKeyboardButton(email).
			WithCallbackData(t.encodeQuery(verb+" "+email)))
	}
	// Two columns once a single one would scroll past the reply on a phone.
	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	return tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
}

func (t *Tgbot) clientEmailsFor(tgUserID int64) []string {
	records, err := t.clientService.GetRecordsByTgID(tgUserID)
	if err != nil || len(records) == 0 {
		return nil
	}
	emails := make([]string, 0, len(records))
	for _, record := range records {
		emails = append(emails, record.Email)
	}
	return emails
}
