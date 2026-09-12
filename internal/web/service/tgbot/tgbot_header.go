package tgbot

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// expiryState is the three cases the roster already distinguishes, kept apart
// from formatting so a link page and the roster cannot disagree about them.
type expiryState int

const (
	expiryUnlimited expiryState = iota
	expiryNotStarted
	expiryDated
)

func expiryStateOf(expiryTime int64) expiryState {
	switch {
	case expiryTime == 0:
		return expiryUnlimited
	case expiryTime < 0:
		return expiryNotStarted
	default:
		return expiryDated
	}
}

// A config with no quota reports unlimited rather than zero left, which a
// customer would read as being cut off.
func remainingBytes(used, total int64) (int64, bool) {
	if total <= 0 {
		return 0, false
	}
	return max(total-used, 0), true
}

// clientHeader says which config a link page is showing and how much of it is
// left, so a customer holding several does not have to guess from the links.
func (t *Tgbot) clientHeader(email string) string {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		logger.Warning("tgbot: link page header lookup failed for", email, ":", err)
		return ""
	}

	expiry := t.I18nBot("tgbot.unlimited")
	switch expiryStateOf(traffic.ExpiryTime) {
	case expiryNotStarted:
		expiry = t.I18nBot("tgbot.messages.rosterNotStarted")
	case expiryDated:
		expiry = time.Unix(traffic.ExpiryTime/1000, 0).Format("2006-01-02")
	}

	remaining := t.I18nBot("tgbot.unlimited")
	if left, limited := remainingBytes(traffic.Up+traffic.Down, traffic.Total); limited {
		remaining = common.FormatTraffic(left)
	}

	return t.I18nBot("tgbot.messages.clientHeader",
		"Email=="+email,
		"Expiry=="+expiry,
		"Remaining=="+remaining) + "\r\n\r\n"
}
