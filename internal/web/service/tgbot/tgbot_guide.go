package tgbot

import (
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const guideCallbackPrefix = "guide_"

// guidePlatform is one entry in the setup picker. The message key comes from this
// table rather than the tag, so callback data can never reach the localizer.
type guidePlatform struct {
	tag        string
	labelKey   string
	messageKey string
}

var guidePlatforms = []guidePlatform{
	{tag: "ios", labelKey: "tgbot.buttons.guideIos", messageKey: "tgbot.messages.guideIos"},
	{tag: "android", labelKey: "tgbot.buttons.guideAndroid", messageKey: "tgbot.messages.guideAndroid"},
	{tag: "windows", labelKey: "tgbot.buttons.guideWindows", messageKey: "tgbot.messages.guideWindows"},
	{tag: "macos", labelKey: "tgbot.buttons.guideMacos", messageKey: "tgbot.messages.guideMacos"},
	{tag: "linux", labelKey: "tgbot.buttons.guideLinux", messageKey: "tgbot.messages.guideLinux"},
}

func parseGuideCallback(data string) (string, bool) {
	tag, found := strings.CutPrefix(data, guideCallbackPrefix)
	if !found {
		return "", false
	}
	for _, platform := range guidePlatforms {
		if platform.tag == tag {
			return platform.messageKey, true
		}
	}
	return "", false
}

func (t *Tgbot) guideKeyboard() *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(guidePlatforms))
	for _, platform := range guidePlatforms {
		buttons = append(buttons, tu.InlineKeyboardButton(t.I18nBot(platform.labelKey)).
			WithCallbackData(t.encodeQuery(guideCallbackPrefix+platform.tag)))
	}
	rows := tu.InlineKeyboardCols(2, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_help")),
	))
	return tu.InlineKeyboardGrid(rows)
}

func (t *Tgbot) guideMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.guideChoose"), t.guideKeyboard())
}

// Re-renders the picker under the instructions so a customer can compare two
// platforms without walking back out to the main menu.
func (t *Tgbot) sendGuide(chatId int64, messageKey string) {
	t.SendMsgToTgbot(chatId, t.I18nBot(messageKey), t.guideKeyboard())
}
