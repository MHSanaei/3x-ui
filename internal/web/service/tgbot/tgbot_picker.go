package tgbot

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// pickerPlan is what a picker should do with the caller's own configs: resolve
// the only one, or offer the choice.
type pickerPlan struct {
	auto  string
	offer []string
}

func planPicker(emails []string) pickerPlan {
	if len(emails) == 1 {
		return pickerPlan{auto: emails[0]}
	}
	return pickerPlan{offer: emails}
}

// Two columns once a single one would scroll past the reply on a phone.
func pickerColumns(count int) int {
	if count >= 6 {
		return 2
	}
	return 1
}

func (t *Tgbot) pickerKeyboard(verb string, emails []string) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(emails))
	for _, email := range emails {
		buttons = append(buttons, tu.InlineKeyboardButton(email).
			WithCallbackData(t.encodeQuery(verb+" "+email)))
	}
	return tu.InlineKeyboardGrid(tu.InlineKeyboardCols(pickerColumns(len(buttons)), buttons...))
}

// clientPicker resolves which of the caller's own configs a verb applies to. The
// emails come from the caller's Telegram id, so an auto-selected config is theirs.
func (t *Tgbot) clientPicker(chatId int64, from *telego.User, verb string, level userLevel) {
	traffics, err := t.inboundService.GetClientTrafficTgBot(from.ID)
	if err != nil {
		logger.Warning("tgbot: client picker lookup failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if len(traffics) == 0 {
		t.SendMsgToTgbot(chatId, t.noBoundClientMsg(level))
		return
	}

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}

	plan := planPicker(emails)
	if plan.auto != "" {
		t.runClientSelfAction(chatId, from, verb, plan.auto)
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), t.pickerKeyboard(verb, plan.offer))
}

// runClientSelfAction is the one place a per-client customer verb is turned
// into work, shared by the callback router and the picker's auto-selection.
func (t *Tgbot) runClientSelfAction(chatId int64, from *telego.User, verb string, arg string) {
	switch verb {
	case "client_sub_links":
		t.sendClientSubLinks(chatId, arg)
	case "client_individual_links":
		t.sendClientIndividualLinks(chatId, arg)
	case "client_qr_links":
		t.sendClientQRLinks(chatId, arg)
	case "client_one_link":
		t.oneLinkPicker(chatId, arg)
	case "link_one":
		if target, index, ok := splitEmailIndexTarget(arg); ok {
			t.sendOneLink(chatId, target, index)
		}
	case "qr_sub":
		t.sendSubscriptionQR(chatId, arg, false)
	case "qr_subjson":
		t.sendSubscriptionQR(chatId, arg, true)
	case "qr_pick":
		t.qrLinkPicker(chatId, arg)
	case "qr_one":
		if target, index, ok := splitEmailIndexTarget(arg); ok {
			t.sendIndividualLinkQR(chatId, target, index)
		}
	case "renew_req":
		t.requestRenewal(chatId, from, arg)
	case "renew_mute":
		t.muteRenewal(chatId, from.ID, arg)
	case "client_reset_self":
		t.confirmSelfReset(chatId, from.ID, arg)
	case "client_reset_self_c":
		t.applySelfReset(chatId, from.ID, arg)
	}
}
