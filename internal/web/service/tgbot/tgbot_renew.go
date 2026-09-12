package tgbot

import (
	"html"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// One request per client per day: the card goes to every admin, so an impatient
// customer must not be able to turn a button into a flood.
const renewRequestCooldown = 24 * time.Hour

// A mute is stored against the expiry it was made for, so an extension makes it
// stale by itself and reminders resume with no cleanup job to forget to run.
func muteCovers(stored int64, found bool, expiry int64) bool {
	return found && stored == expiry
}

// A clock that has gone backwards reads as "asked just now" rather than
// reopening the gate, since the stored time is the one we cannot trust.
func withinCooldown(last int64, now time.Time, window time.Duration) bool {
	if last <= 0 {
		return false
	}
	return now.Unix()-last < int64(window.Seconds())
}

func (t *Tgbot) renewKeyboard(email string) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.renewRequest")).WithCallbackData(t.encodeQuery("renew_req "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.renewMute")).WithCallbackData(t.encodeQuery("renew_mute "+email)),
		),
	)
}

func (t *Tgbot) isRenewMuted(email string, expiry int64) bool {
	stored, found := renewOptOut.get(t, email)
	return muteCovers(stored, found, expiry)
}

// Muting is recorded against the client's expiry as it stands now, which is
// what lets a later extension undo it.
func (t *Tgbot) muteRenewal(chatId int64, tgUserID int64, email string) {
	scoped := t.forUser(tgUserID)
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		logger.Warning("tgbot: renewal mute lookup failed for", email, ":", err)
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if err := renewOptOut.put(t, email, traffic.ExpiryTime); err != nil {
		logger.Warning("tgbot: renewal mute save failed for", email, ":", err)
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.messages.renewMuted", "Email=="+email))
}

func (t *Tgbot) requestRenewal(chatId int64, from *telego.User, email string) {
	scoped := t.forUser(from.ID)
	last, _ := renewReqAt.get(t, email)
	if withinCooldown(last, time.Now(), renewRequestCooldown) {
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.messages.renewRequestThrottled"))
		return
	}

	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		logger.Warning("tgbot: renewal request lookup failed for", email, ":", err)
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.answers.errorOperation"))
		return
	}

	if err := renewReqAt.put(t, email, time.Now().Unix()); err != nil {
		logger.Warning("tgbot: renewal request state save failed:", err)
	}
	t.SendMsgToTgbotAdmins(t.renewRequestCard(from, traffic), tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData(t.encodeQuery("reset_exp "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.replyToClient")).WithCallbackData(t.encodeQuery("pm_reply "+strconv.FormatInt(from.ID, 10))),
		),
	))

	// Clearing the mute is what makes Renew the opposite of Stop reminders: a
	// customer who muted and then asked to renew expects to be chased again.
	if err := renewOptOut.drop(t, email); err != nil {
		logger.Warning("tgbot: renewal mute clear failed:", err)
	}
	scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.messages.renewRequested"))
}

func (t *Tgbot) renewRequestCard(from *telego.User, traffic *xray.ClientTraffic) string {
	name := html.EscapeString(from.FirstName)
	if name == "" {
		name = strconv.FormatInt(from.ID, 10)
	}
	mention := `<a href="tg://user?id=` + strconv.FormatInt(from.ID, 10) + `">` + name + `</a>`
	if from.Username != "" {
		mention += " (@" + html.EscapeString(from.Username) + ")"
	}

	expiry := t.I18nBot("tgbot.unlimited")
	if traffic.ExpiryTime > 0 {
		expiry = time.Unix(traffic.ExpiryTime/1000, 0).Format("2006-01-02")
	}
	return t.I18nBot("tgbot.messages.renewRequestCard",
		"Client=="+mention,
		"Email=="+html.EscapeString(traffic.Email),
		"Expiry=="+expiry,
		"Used=="+common.FormatTraffic(traffic.Up+traffic.Down))
}
