package tgbot

import (
	"strconv"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// updateNumericInput applies one number-pad key: -2 clears, -1 backspaces, and 0..9 append.
// Callers retain their own validation and keyboard labels.
func updateNumericInput(value, key int) int {
	switch key {
	case -2:
		return 0
	case -1:
		if value > 0 {
			return value / 10
		}
		return value
	default:
		return value*10 + key
	}
}

// numericKeypadSpec describes one number-pad flow. dataBase is the callback
// prefix without its "_in"/"_c" suffix, and dataArgs carries any leading
// argument (an email plus its separating space) the flow threads through.
type numericKeypadSpec struct {
	dataBase        string
	dataArgs        string
	cancelData      string
	confirmLabelKey string
}

// numericKeypad builds the shared digit pad: cancel, confirm, 1-9, clear, 0 and
// backspace. Every numeric callback flow renders the same grid, so the layout
// and the callback wording live here rather than once per flow.
func (t *Tgbot) numericKeypad(spec numericKeypadSpec, inputNumber int) *telego.InlineKeyboardMarkup {
	value := strconv.Itoa(inputNumber)
	key := func(label, k string) telego.InlineKeyboardButton {
		return tu.InlineKeyboardButton(label).
			WithCallbackData(t.encodeQuery(spec.dataBase + "_in " + spec.dataArgs + value + " " + k))
	}
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery(spec.cancelData)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot(spec.confirmLabelKey, "Num=="+value)).
				WithCallbackData(t.encodeQuery(spec.dataBase+"_c "+spec.dataArgs+value)),
		),
		tu.InlineKeyboardRow(key("1", "1"), key("2", "2"), key("3", "3")),
		tu.InlineKeyboardRow(key("4", "4"), key("5", "5"), key("6", "6")),
		tu.InlineKeyboardRow(key("7", "7"), key("8", "8"), key("9", "9")),
		tu.InlineKeyboardRow(key("🔄", "-2"), key("0", "0"), key("⬅️", "-1")),
	)
}
