package tgbot

import (
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	hourCallbackPrefix = "set_hour "
	defaultDailyHour   = 8
)

// The hour arrives as callback data, so a value outside a clock is refused
// before it can be stored and silently disable the pass forever.
func parseHourCallback(data string) (int, bool) {
	raw, found := strings.CutPrefix(data, hourCallbackPrefix)
	if !found {
		return 0, false
	}
	return parseHour(raw)
}

func parseHour(raw string) (int, bool) {
	hour, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	return hour, true
}

// An unreadable or out-of-range setting falls back to 08:00 rather than to
// midnight, so a damaged row does not start waking customers at 3am.
func (t *Tgbot) dailyHour() int {
	hour, err := t.settingService.GetTgBotDailyHour()
	if err != nil || hour < 0 || hour > 23 {
		return defaultDailyHour
	}
	return hour
}

// RunDailyPass sends the customer-facing notices once a day at the hour an admin chose.
// It is scheduled hourly and gates itself, so a change needs no cron edit or restart.
func (t *Tgbot) RunDailyPass() {
	if !t.IsRunning() {
		return
	}
	if time.Now().Hour() != t.dailyHour() {
		return
	}

	today := time.Now().Format("2006-01-02")
	if t.ladderAlreadyRanToday(today) {
		return
	}

	t.reminderPass()

	if err := t.settingService.SetTgBotLadderRun(today); err != nil {
		logger.Warning("tgbot: daily pass state save failed:", err)
	}
}

// The notices themselves, with no gate of their own, so the Remind-now button
// sends exactly what the schedule would have sent.
func (t *Tgbot) reminderPass() {
	// Every notice in the pass goes out quietly, including the admin summary:
	// nothing here is urgent enough to alert at a fixed hour each day.
	quiet := t.quiet()
	quiet.notifyRenewals()
	quiet.notifyExhausted()
	quiet.notifyQuota()
}

// Four rows of six so a whole day fits without scrolling.
func (t *Tgbot) hourKeyboard(current int) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, 24)
	for hour := range 24 {
		label := strconv.Itoa(hour) + ":00"
		if hour == current {
			label = languageMark + label
		}
		buttons = append(buttons, tu.InlineKeyboardButton(label).
			WithCallbackData(t.encodeQuery(hourCallbackPrefix+strconv.Itoa(hour))))
	}
	rows := tu.InlineKeyboardCols(6, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.botFeatures")).WithCallbackData(t.encodeQuery("admin_features")),
	))
	return tu.InlineKeyboardGrid(rows)
}

func (t *Tgbot) hourMenu(chatId int64, messageID int) {
	keyboard := t.hourKeyboard(t.dailyHour())
	prompt := t.I18nBot("tgbot.messages.dailyHourPrompt")
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, prompt, keyboard)
		return
	}
	t.SendMsgToTgbot(chatId, prompt, keyboard)
}

func (t *Tgbot) applyDailyHour(chatId int64, hour int, messageID int) {
	if err := t.settingService.SetTgBotDailyHour(hour); err != nil {
		logger.Warning("tgbot: daily hour save failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	saved := t.I18nBot("tgbot.messages.dailyHourSaved", "Hour=="+strconv.Itoa(hour)+":00")
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, saved, t.hourKeyboard(hour))
		return
	}
	t.SendMsgToTgbot(chatId, saved, t.hourKeyboard(hour))
}
