package tgbot

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	langCallbackPrefix = "settings_setlang "
	languageMark       = "✅ "
)

// parseLangCallback validates the tag before it reaches the language store: the
// data behind a button is whatever the user's client chose to send back.
func parseLangCallback(data string) (string, bool) {
	tag, found := strings.CutPrefix(data, langCallbackPrefix)
	if !found || !isSupportedLang(tag) {
		return "", false
	}
	return tag, true
}

func (t *Tgbot) settingsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.language")).WithCallbackData(t.encodeQuery("settings_lang")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
		),
	)
}

// Two columns keep 13 languages on a phone screen without scrolling past the
// back button.
func (t *Tgbot) languageKeyboard(current string) *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(botLanguages)/2+2)
	for i := 0; i < len(botLanguages); i += 2 {
		row := make([]telego.InlineKeyboardButton, 0, 2)
		for _, lang := range botLanguages[i:min(i+2, len(botLanguages))] {
			label := lang.label
			if lang.tag == current {
				label = languageMark + label
			}
			row = append(row, tu.InlineKeyboardButton(label).
				WithCallbackData(t.encodeQuery(langCallbackPrefix+lang.tag)))
		}
		rows = append(rows, row)
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToSettings")).WithCallbackData(t.encodeQuery("client_settings")),
	))
	return tu.InlineKeyboardGrid(rows)
}

func (t *Tgbot) settingsMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.settings"), t.settingsKeyboard())
}

func (t *Tgbot) languageMenu(chatId int64, tgUserID int64, messageID int) {
	keyboard := t.languageKeyboard(t.langOf(tgUserID))
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, t.I18nBot("tgbot.messages.languagePrompt"), keyboard)
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.languagePrompt"), keyboard)
}

// Re-renders in the language just chosen, so the confirmation is itself the
// proof that the choice took effect.
func (t *Tgbot) applyLanguage(chatId int64, tgUserID int64, tag string, messageID int) {
	if err := t.setUserLang(tgUserID, tag); err != nil {
		logger.Warning("tgbot: language save failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}

	scoped := t.forUser(tgUserID)
	if messageID > 0 {
		scoped.editMessageTgBot(chatId, messageID,
			scoped.I18nBot("tgbot.messages.languageSaved", "Language=="+languageLabel(tag)),
			scoped.languageKeyboard(tag))
		return
	}
	scoped.SendMsgToTgbot(chatId,
		scoped.I18nBot("tgbot.messages.languageSaved", "Language=="+languageLabel(tag)),
		scoped.languageKeyboard(tag))
}
