package tgbot

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// A reset drops every device this client has installed, so one a day is plenty
// and a misfire cannot be repeated into an outage.
const selfResetCooldown = 24 * time.Hour

// Fails closed, unlike the notification toggles: a lookup failure must not hand
// customers a destructive action an operator never turned on.
func (t *Tgbot) selfResetEnabled() bool {
	enabled, err := t.settingService.GetTgBotAllowSelfReset()
	if err != nil {
		logger.Warning("tgbot: self-reset setting lookup failed:", err)
		return false
	}
	return enabled
}

// selfResetGate is the whole authorisation for a customer resetting their own config.
// An empty key means refuse in silence: saying why would confirm the client exists.
func (t *Tgbot) selfResetGate(tgUserID int64, email string) (bool, string) {
	if !t.selfResetEnabled() {
		return false, "tgbot.messages.selfResetDisabled"
	}
	if !t.ownsClient(tgUserID, email) {
		return false, ""
	}
	last, _ := selfResetAt.get(t, email)
	if withinCooldown(last, time.Now(), selfResetCooldown) {
		return false, "tgbot.messages.selfResetThrottled"
	}
	return true, ""
}

func (t *Tgbot) selfResetKeyboard(email string) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmSelfReset")).WithCallbackData(t.encodeQuery("client_reset_self_c "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToMenu")).WithCallbackData(t.encodeQuery("client_menu")),
		),
	)
}

// The warning is the point of the confirm step: the customer must know their
// installed configs stop working before they tap, not after.
func (t *Tgbot) confirmSelfReset(chatId int64, tgUserID int64, email string) {
	scoped := t.forUser(tgUserID)
	if allowed, reason := t.selfResetGate(tgUserID, email); !allowed {
		if reason != "" {
			scoped.SendMsgToTgbot(chatId, scoped.I18nBot(reason))
		}
		return
	}
	scoped.SendMsgToTgbot(chatId,
		scoped.I18nBot("tgbot.messages.selfResetWarning", "Email=="+email),
		scoped.selfResetKeyboard(email))
}

// Rotates the protocol secret and the subscription token together: a leaked
// subscription URL has already handed over the individual configs behind it.
func (t *Tgbot) applySelfReset(chatId int64, tgUserID int64, email string) {
	scoped := t.forUser(tgUserID)
	if allowed, reason := t.selfResetGate(tgUserID, email); !allowed {
		if reason != "" {
			scoped.SendMsgToTgbot(chatId, scoped.I18nBot(reason))
		}
		return
	}

	needRestart, err := t.clientService.RotateClientCredentialsByEmail(&t.inboundService, email)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("tgbot: self-reset credential rotation failed for", email, ":", err)
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.answers.errorOperation"))
		return
	}

	subID := uuid.NewString()
	if err := t.editClientRecord(email, func(c *model.Client) { c.SubID = subID }); err != nil {
		logger.Warning("tgbot: self-reset subId regeneration failed for", email, ":", err)
		scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.answers.errorOperation"))
		return
	}

	// Recorded only after both halves succeeded, so a failed attempt does not
	// lock the customer out of retrying for a day.
	if err := selfResetAt.put(t, email, time.Now().Unix()); err != nil {
		logger.Warning("tgbot: self-reset state save failed:", err)
	}

	scoped.SendMsgToTgbot(chatId, scoped.I18nBot("tgbot.messages.selfResetDone", "Email=="+email))
	scoped.sendClientSubLinks(chatId, email)
}
