package tgbot

import (
	"fmt"
	"html"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// A config with no quota reports unlimited rather than zero left, which a
// customer would read as being cut off.
func remainingBytes(used, total int64) (int64, bool) {
	if total <= 0 {
		return 0, false
	}
	if used >= total {
		return 0, true
	}
	return total - used, true
}

// headerExpiry renders the expiry the way the client card does: a negative
// value counts days from first use rather than naming a date.
func (t *Tgbot) headerExpiry(expiryTime int64) string {
	switch {
	case expiryTime == 0:
		return t.I18nBot("tgbot.unlimited")
	case expiryTime < 0:
		return fmt.Sprintf("%d %s", expiryTime/-86400000, t.I18nBot("tgbot.days"))
	default:
		return time.UnixMilli(expiryTime).Format("2006-01-02")
	}
}

// clientHeader says which config a link page shows and how much of it is left,
// so a customer holding several does not have to guess from the links.
func (t *Tgbot) clientHeader(email string) string {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		logger.Warning("tgbot: link page header lookup failed for", email, ":", err)
		return t.clientScreenBody(email, "")
	}
	remaining := t.I18nBot("tgbot.unlimited")
	if left, limited := remainingBytes(traffic.Up+traffic.Down, traffic.Total); limited {
		remaining = common.FormatTraffic(left)
	}
	return t.I18nBot("tgbot.messages.clientHeader",
		"Email=="+html.EscapeString(email),
		"Expiry=="+t.headerExpiry(traffic.ExpiryTime),
		"Remaining=="+remaining) + "\r\n\r\n"
}
