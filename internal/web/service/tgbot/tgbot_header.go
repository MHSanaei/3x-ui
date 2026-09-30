package tgbot

import (
	"html"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// A config can be unlimited, dated, or not started yet, and the three read very
// differently to a customer, so the link pages name which one they are looking at.
func clientExpiryLabel(t *Tgbot, expiryTime int64) string {
	switch {
	case expiryTime == 0:
		return t.I18nBot("tgbot.messages.unlimited")
	case expiryTime < 0:
		return t.I18nBot("tgbot.messages.notStarted")
	default:
		return time.Unix(expiryTime/1000, 0).Format("2006-01-02")
	}
}

// No quota means unlimited, not zero remaining: reporting the latter would tell
// a customer with an uncapped config that they are cut off.
func clientRemainingLabel(t *Tgbot, used, total int64) string {
	if total <= 0 {
		return t.I18nBot("tgbot.messages.unlimited")
	}
	return common.FormatTraffic(max(total-used, 0))
}

// clientHeader names the config a link page is showing and how much of it is
// left, so a customer holding several does not have to guess from the links.
func (t *Tgbot) clientHeader(email string) string {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		logger.Warning("tgbot: link page header lookup failed for", email, ":", err)
		return ""
	}
	return t.I18nBot("tgbot.messages.clientHeader",
		"Email=="+html.EscapeString(email),
		"Expiry=="+clientExpiryLabel(t, traffic.ExpiryTime),
		"Remaining=="+clientRemainingLabel(t, traffic.Up+traffic.Down, traffic.Total)) + "\r\n\r\n"
}
