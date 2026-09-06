package tgbot

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// renewalRung is how close a subscription is to its renewal date. The rungs are
// exact-day matches rather than a window, so each one fires once per client.
type renewalRung int

const (
	rungNone renewalRung = iota
	rungThreeDays
	rungTwoDays
	rungTomorrow
	rungToday
	rungOverdue
)

// A rung missing from this map is summarised for admins but never sent to the
// customer: rungTwoDays exists to widen the admin's view, not to add a reminder.
var rungMessageKey = map[renewalRung]string{
	rungThreeDays: "tgbot.messages.renewThreeDays",
	rungTomorrow:  "tgbot.messages.renewTomorrow",
	rungToday:     "tgbot.messages.renewToday",
	rungOverdue:   "tgbot.messages.renewOverdue",
}

// A zero expiry never lapses and a negative one has not started counting, so neither
// is on the ladder. Days are calendar days, so "tomorrow" is the next date, not +24h.
func expiryRung(expiryTime int64, now time.Time) renewalRung {
	if expiryTime <= 0 {
		return rungNone
	}
	expiry := time.UnixMilli(expiryTime)
	days := int(dayStart(expiry).Sub(dayStart(now)).Hours() / 24)
	switch {
	case days < 0:
		return rungOverdue
	case days == 0:
		return rungToday
	case days == 1:
		return rungTomorrow
	case days == 2:
		return rungTwoDays
	case days == 3:
		return rungThreeDays
	default:
		return rungNone
	}
}

func dayStart(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// Exact-day rungs assume one pass per day, but tgRunTime is an admin-editable
// cron: an hourly schedule would otherwise nag every customer 24 times.
func (t *Tgbot) ladderAlreadyRanToday(today string) bool {
	last, err := t.settingService.GetTgBotLadderRun()
	if err != nil {
		logger.Warning("tgbot: renewal ladder state lookup failed:", err)
		return false
	}
	return last == today
}

func (t *Tgbot) notifyRenewals() {
	records, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: renewal ladder client list failed:", err)
		return
	}

	now := time.Now()
	byRung := map[renewalRung][]renewalNotice{}
	for i := range records {
		record := &records[i].ClientRecord
		rung := expiryRung(record.ExpiryTime, now)
		if rung == rungNone {
			continue
		}
		byRung[rung] = append(byRung[rung], renewalNotice{
			email:   record.Email,
			outcome: t.remindCustomer(record.Email, record.TgID, rung, record.ExpiryTime),
		})
	}

	if summary := t.renewalSummary(byRung); summary != "" {
		t.SendMsgToTgbotAdmins(summary)
	}
}

// Groups the day's notices so an admin sees who was chased without reading the
// same message once per customer.
func (t *Tgbot) renewalSummary(byRung map[renewalRung][]renewalNotice) string {
	order := []renewalRung{rungOverdue, rungToday, rungTomorrow, rungTwoDays, rungThreeDays}
	sections := make([]string, 0, len(order))
	for _, rung := range order {
		notices := byRung[rung]
		if len(notices) == 0 {
			continue
		}
		sections = append(sections, t.I18nBot(rungSummaryKey[rung],
			"Count=="+strconv.Itoa(len(notices)),
			"Clients=="+summaryClientList(notices)))
	}
	if len(sections) == 0 {
		return ""
	}
	return t.I18nBot("tgbot.messages.renewSummaryHeader") + "\r\n\r\n" + strings.Join(sections, "\r\n")
}

var rungSummaryKey = map[renewalRung]string{
	rungThreeDays: "tgbot.messages.renewSummaryThreeDays",
	rungTwoDays:   "tgbot.messages.renewSummaryTwoDays",
	rungTomorrow:  "tgbot.messages.renewSummaryTomorrow",
	rungToday:     "tgbot.messages.renewSummaryToday",
	rungOverdue:   "tgbot.messages.renewSummaryOverdue",
}

// Sorted by email so the list reads the same each day rather than following
// whichever send finished first.
func summaryClientList(notices []renewalNotice) string {
	sort.Slice(notices, func(i, j int) bool { return notices[i].email < notices[j].email })
	lines := make([]string, 0, len(notices))
	for _, notice := range notices {
		lines = append(lines, notice.outcome.mark()+notice.email)
	}
	return strings.Join(lines, ", ")
}

// renewalNotice is one client's place on the ladder plus what became of the
// reminder, so the admin summary can say who was actually reached.
type renewalNotice struct {
	email   string
	outcome deliveryOutcome
}

type deliveryOutcome int

const (
	// The rung sends no customer reminder, so there is nothing to report.
	deliveryNone deliveryOutcome = iota
	deliveryUnlinked
	deliveryFailed
	deliveryDelivered
	deliveryMuted
)

// A cross means the bot tried and Telegram refused; the dash means there was no
// account to try, which is an admin's job to fix rather than the customer's.
func (o deliveryOutcome) mark() string {
	switch o {
	case deliveryDelivered:
		return "✅ "
	case deliveryFailed:
		return "❌ "
	case deliveryUnlinked:
		return "➖ "
	case deliveryMuted:
		return "🔕 "
	default:
		return ""
	}
}

// Sends through sendHTMLDirect rather than SendMsgToTgbot because the summary
// has to distinguish a delivered reminder from one Telegram refused.
func (t *Tgbot) remindCustomer(email string, tgID int64, rung renewalRung, expiry int64) deliveryOutcome {
	messageKey, notifies := rungMessageKey[rung]
	if !notifies {
		return deliveryNone
	}
	if tgID == 0 {
		return deliveryUnlinked
	}
	if t.isRenewMuted(email, expiry) {
		return deliveryMuted
	}

	scoped := t.forUser(tgID)
	if err := scoped.sendHTMLDirect(tgID, scoped.I18nBot(messageKey, "Email=="+email), scoped.renewKeyboard(email)); err != nil {
		logger.Warning("tgbot: renewal reminder refused for", email, ":", err)
		return deliveryFailed
	}
	return deliveryDelivered
}
