package tgbot

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	stateHelpText = "awaiting_help_text"
	helpTextReset = "-"
)

// An empty stored value means the operator never customised the text, so the
// translated built-in help is served instead of a blank message.
func (t *Tgbot) helpText() string {
	stored, err := t.settingService.GetTgBotHelpText()
	if err != nil {
		logger.Warning("tgbot: help text lookup failed:", err)
		return t.I18nBot("tgbot.commands.help")
	}
	if strings.TrimSpace(stored) == "" {
		return t.I18nBot("tgbot.commands.help")
	}
	return stored
}

// Help is the one place a customer looks when stuck, so the setup guide and the
// command sheet hang off it rather than competing with it on the main keyboard.
func (t *Tgbot) helpKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.botCommands")).WithCallbackData(t.encodeQuery("client_commands")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.setupGuide")).WithCallbackData(t.encodeQuery("client_guide")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
		),
	)
}

// The text arrives with the hub rather than behind it: one tap still answers
// the common case, and the buttons are there when it does not.
func (t *Tgbot) helpMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.helpText(), t.helpKeyboard())
}

// The admin command sheet is built in one place so /help and the Commands
// button cannot drift apart as commands are added.
func (t *Tgbot) adminCommandHelp() string {
	return strings.Join([]string{
		t.I18nBot("tgbot.commands.helpAdminCommands"),
		t.I18nBot("tgbot.commands.helpAdminExtraCommands"),
		t.I18nBot("tgbot.commands.whoisUsage"),
		t.I18nBot("tgbot.commands.serverMenuUsage"),
		t.I18nBot("tgbot.commands.clientsUsage"),
	}, "\r\n\r\n")
}

func (t *Tgbot) startSetHelp(chatId int64) {
	userStateMgr.set(chatId, stateHelpText)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextPrompt", "Reset=="+helpTextReset))
}

// The text is proved renderable before it is stored: the bot sends every
// message with HTML parse mode, so stray markup would silently break /help.
func (t *Tgbot) saveHelpText(chatId int64, text string) {
	if strings.TrimSpace(text) == helpTextReset {
		if err := t.settingService.SetTgBotHelpText(""); err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextFailed", "Error=="+err.Error()))
			return
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextReset"))
		return
	}
	if strings.TrimSpace(text) == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextEmpty"))
		return
	}
	if err := t.sendHTMLDirect(chatId, text); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextInvalid", "Error=="+err.Error()))
		return
	}
	if err := t.settingService.SetTgBotHelpText(text); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextSaved"))
}
