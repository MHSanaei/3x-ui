package tgbot

import (
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	bindLimitCallbackPrefix = "set_bindmax "
	defaultMaxBindings      = 5
	maxBindingsCeiling      = 50
)

// What the picker offers. 1 restores the original one-config-per-account rule,
// 0 removes the ceiling, and the rest are the household sizes in between.
var bindLimitChoices = []int{1, 2, 3, 4, 5, 6, 8, 10, 15, 20, 0}

// The limit arrives as callback data, so a value outside the stored range is
// refused before it can become a ceiling the menu never offered.
func parseBindLimitCallback(data string) (int, bool) {
	raw, found := strings.CutPrefix(data, bindLimitCallbackPrefix)
	if !found {
		return 0, false
	}
	return parseBindLimit(raw)
}

func parseBindLimit(raw string) (int, bool) {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 0 || limit > maxBindingsCeiling {
		return 0, false
	}
	return limit, true
}

// An unreadable or out-of-range setting falls back to the default rather than
// to unlimited: a damaged row must not be the thing that lifts the cap.
func (t *Tgbot) maxBindings() int {
	limit, err := t.settingService.GetTgBotMaxBindings()
	if err != nil || limit < 0 || limit > maxBindingsCeiling {
		return defaultMaxBindings
	}
	return limit
}

// 0 means "no ceiling", so it must never reach an admin as the number zero —
// that reads as "nobody may bind anything".
func bindLimitLabel(limit int) string {
	if limit <= 0 {
		return "∞"
	}
	return strconv.Itoa(limit)
}

// The unit is the subscription, not the client record: one subscription can
// span several inbounds, and the customer reads all of it as a single config.
func countSubscriptions(records []*model.ClientRecord) int {
	subs := make(map[string]bool, len(records))
	loose := 0
	for _, record := range records {
		if record.SubID == "" {
			loose++
			continue
		}
		subs[record.SubID] = true
	}
	return len(subs) + loose
}

// Records behind the token being claimed do not count against the holder, so
// a re-tap, or finishing a partly bound subscription, is not a second config.
func recordsOutsideToken(held []*model.ClientRecord, token []*model.ClientRecord) []*model.ClientRecord {
	inToken := make(map[int]bool, len(token))
	for _, record := range token {
		inToken[record.Id] = true
	}
	outside := make([]*model.ClientRecord, 0, len(held))
	for _, record := range held {
		if !inToken[record.Id] {
			outside = append(outside, record)
		}
	}
	return outside
}

// Whether this account may take on one more subscription, and the ceiling it
// was judged against. Fails closed: an unreadable roster must not read as zero.
func (t *Tgbot) bindingHeadroom(tgID int64, token []*model.ClientRecord) (bool, int) {
	limit := t.maxBindings()
	if limit <= 0 {
		return true, limit
	}
	held, err := t.clientService.GetRecordsByTgID(tgID)
	if err != nil {
		logger.Warning("tgbot: binding headroom lookup failed:", err)
		return false, limit
	}
	return countSubscriptions(recordsOutsideToken(held, token))+1 <= limit, limit
}

// What the account holds once a claim has landed, so the arrival notice can
// tell an admin how many configs one person has collected.
func (t *Tgbot) heldSubscriptions(tgID int64) int {
	held, err := t.clientService.GetRecordsByTgID(tgID)
	if err != nil {
		logger.Warning("tgbot: held subscription count failed:", err)
		return 0
	}
	return countSubscriptions(held)
}

// Two rows of six, matching the hour picker it sits beside in Bot Settings.
func (t *Tgbot) bindLimitKeyboard(current int) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(bindLimitChoices))
	for _, choice := range bindLimitChoices {
		label := bindLimitLabel(choice)
		if choice == current {
			label = languageMark + label
		}
		buttons = append(buttons, tu.InlineKeyboardButton(label).
			WithCallbackData(t.encodeQuery(bindLimitCallbackPrefix+strconv.Itoa(choice))))
	}
	rows := tu.InlineKeyboardCols(6, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_settings")),
	))
	return tu.InlineKeyboardGrid(rows)
}

func (t *Tgbot) bindLimitMenu(chatId int64, messageID int) {
	keyboard := t.bindLimitKeyboard(t.maxBindings())
	prompt := t.I18nBot("tgbot.messages.bindLimitPrompt")
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, prompt, keyboard)
		return
	}
	t.SendMsgToTgbot(chatId, prompt, keyboard)
}

func (t *Tgbot) applyBindLimit(chatId int64, limit int, messageID int) {
	if err := t.settingService.SetTgBotMaxBindings(limit); err != nil {
		logger.Warning("tgbot: binding limit save failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	saved := t.I18nBot("tgbot.messages.bindLimitSaved", "Limit=="+bindLimitLabel(limit))
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, saved, t.bindLimitKeyboard(limit))
		return
	}
	t.SendMsgToTgbot(chatId, saved, t.bindLimitKeyboard(limit))
}
