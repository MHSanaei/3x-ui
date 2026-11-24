package tgbot

import (
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	cbClientGuide       = "client_guide"
	guideCallbackPrefix = "guide_"
)

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

// The steps send the customer to their links page, so it is one tap away; the
// bare verb resolves their own config, or asks which one.
func (t *Tgbot) guideRows() [][]telego.InlineKeyboardButton {
	buttons := make([]telego.InlineKeyboardButton, 0, len(guidePlatforms))
	for _, platform := range guidePlatforms {
		buttons = append(buttons, t.btn(platform.labelKey, guideCallbackPrefix+platform.tag))
	}
	rows := tu.InlineKeyboardCols(2, buttons...)
	return append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.links", "client_sub_links")))
}

// screenGuide draws the platform picker, under the chosen platform's steps when
// there is one, so a customer can compare two platforms without walking back.
func (t *Tgbot) screenGuide(chatID int64, messageKey string) {
	body := t.I18nBot("tgbot.messages.guideChoose")
	if messageKey != "" {
		body = t.I18nBot(messageKey)
	}
	t.renderScreen(chatID, t.newScreen("guide", body, t.guideRows()...))
}
