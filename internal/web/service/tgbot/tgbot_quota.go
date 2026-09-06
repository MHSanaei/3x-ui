package tgbot

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// quotaRung is how far through their traffic allowance a client is. The rungs
// mirror expiryRung: fixed thresholds, each announced once as it is crossed.
type quotaRung int

const (
	quotaNone quotaRung = iota
	quotaEighty
	quotaNinetyFive
)

var quotaRungMessageKey = map[quotaRung]string{
	quotaEighty:     "tgbot.messages.quotaEighty",
	quotaNinetyFive: "tgbot.messages.quotaNinetyFive",
}

// A zero or negative total is an unlimited config, which can never run out, and
// a negative usage is a counter we cannot read rather than a client at 0%.
func quotaRungOf(used, total int64) quotaRung {
	if total <= 0 || used < 0 {
		return quotaNone
	}
	switch percent := used * 100 / total; {
	case percent >= 95:
		return quotaNinetyFive
	case percent >= 80:
		return quotaEighty
	default:
		return quotaNone
	}
}

// Due only on the way up. A stored mark we do not recognise is treated as
// already spoken, so a damaged row cannot turn into a notification loop.
func quotaNoticeIsDue(rung quotaRung, warned int64, marked bool) bool {
	if rung == quotaNone {
		return false
	}
	if !marked {
		return true
	}
	if warned != int64(quotaEighty) && warned != int64(quotaNinetyFive) {
		return false
	}
	return rung > quotaRung(warned)
}

// Dropping below the first rung forgets the client, so a traffic reset re-arms
// both notices without a job that has to know a reset happened.
func (t *Tgbot) syncQuotaMark(email string, rung quotaRung) {
	var err error
	if rung == quotaNone {
		err = quotaWarned.drop(t, email)
	} else {
		err = quotaWarned.put(t, email, int64(rung))
	}
	if err != nil {
		logger.Warning("tgbot: quota mark save failed for", email, ":", err)
	}
}

// notifyQuota warns each linked customer as their own allowance runs down. The
// admin already has the deplete-soon report; this is the other half.
func (t *Tgbot) notifyQuota() {
	if enabled, err := t.settingService.GetTgBotNotifyQuota(); err == nil && !enabled {
		return
	}

	records, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: quota pass client list failed:", err)
		return
	}

	marks := quotaWarned.all(t)
	for i := range records {
		record := &records[i]
		// An unbound client is left unmarked rather than marked-and-skipped, so
		// binding an account later still earns the notice their usage has due.
		if record.TgID == 0 {
			continue
		}

		used := int64(0)
		if record.Traffic != nil {
			used = record.Traffic.Up + record.Traffic.Down
		}

		rung := quotaRungOf(used, record.TotalGB)
		warned, marked := marks[record.Email]
		due := quotaNoticeIsDue(rung, warned, marked)
		t.syncQuotaMark(record.Email, rung)
		if !due {
			continue
		}

		scoped := t.forUser(record.TgID)
		remaining := max(record.TotalGB-used, 0)
		msg := scoped.I18nBot(quotaRungMessageKey[rung],
			"Email=="+record.Email,
			"Percent=="+strconv.FormatInt(used*100/record.TotalGB, 10),
			"Remaining=="+common.FormatTraffic(remaining))
		if err := scoped.sendHTMLDirect(record.TgID, msg); err != nil {
			logger.Warning("tgbot: quota notice refused for", record.Email, ":", err)
		}
	}
}
