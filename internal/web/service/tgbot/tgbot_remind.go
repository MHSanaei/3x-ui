package tgbot

import (
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	tu "github.com/mymmrac/telego/telegoutil"
)

// reminderPlan is who the pass would reach, counted before anything leaves the
// panel so the admin confirms against a number rather than a guess.
type reminderPlan struct {
	renew int
	quota int
	total int
}

func (p reminderPlan) empty() bool { return p.total == 0 }

// Mirrors the two customer ladders the pass walks: expiry and traffic. A client
// due both is one recipient, so total is the union rather than the sum.
func planReminders(records []bulkClient, now time.Time, quotaOn bool, muted func(string, int64) bool, marks map[string]int64) reminderPlan {
	plan := reminderPlan{}
	for i := range records {
		record := &records[i]
		// An unlinked client has nowhere to be reminded; the daily pass counts
		// them in its admin summary, but nothing is sent, so nothing is offered.
		if record.TgID == 0 {
			continue
		}

		due := false
		if _, notifies := rungMessageKey[expiryRung(record.ExpiryTime, now)]; notifies && !muted(record.Email, record.ExpiryTime) {
			plan.renew++
			due = true
		}
		if quotaOn {
			used := int64(0)
			if record.Traffic != nil {
				used = record.Traffic.Up + record.Traffic.Down
			}
			warned, marked := marks[record.Email]
			if quotaNoticeIsDue(quotaRungOf(used, record.TotalGB), warned, marked) {
				plan.quota++
				due = true
			}
		}
		if due {
			plan.total++
		}
	}
	return plan
}

// A quota lookup failure counts the notice as on, matching notifyQuota's own
// fail-open read: the preview must not promise fewer messages than are sent.
func (t *Tgbot) reminderPlan() reminderPlan {
	records, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: reminder preview client list failed:", err)
		return reminderPlan{}
	}
	quotaOn, err := t.settingService.GetTgBotNotifyQuota()
	if err != nil {
		quotaOn = true
	}
	return planReminders(records, time.Now(), quotaOn, t.isRenewMuted, quotaWarned.all(t))
}

// remindPreview names the count before sending, because the button chases every
// due customer at once and there is no way to call a reminder back.
func (t *Tgbot) remindPreview(chatId int64) {
	plan := t.reminderPlan()
	if plan.empty() {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.remindNothing"), t.adminMessagingKeyboard())
		return
	}

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("admin_messaging")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmRemind", "Count=="+strconv.Itoa(plan.total))).
				WithCallbackData(t.encodeQuery("remind_send")),
		),
	)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.remindPreview",
		"Renew=="+strconv.Itoa(plan.renew),
		"Quota=="+strconv.Itoa(plan.quota)), keyboard)
}

// Runs the same pass the scheduler runs, hour gate and all skipped, then claims
// the day so tonight's scheduled run cannot chase the same customers twice.
func (t *Tgbot) runManualReminders(chatId int64) {
	t.reminderPass()
	if err := t.settingService.SetTgBotLadderRun(time.Now().Format("2006-01-02")); err != nil {
		logger.Warning("tgbot: daily pass state save failed:", err)
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.remindDone"), t.adminMessagingKeyboard())
}
