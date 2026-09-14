package tgbot

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type inviteOutcome int

const (
	inviteInvalid inviteOutcome = iota
	inviteTaken
	inviteAlreadyOwned
	inviteBindable
)

// A client's SubID doubles as its invite token: whoever holds it can already
// fetch the subscription, so binding grants no access the token did not.
func (t *Tgbot) resolveInviteToken(token string, fromID int64) (inviteOutcome, []*model.ClientRecord) {
	token = strings.TrimSpace(token)
	if token == "" || fromID <= 0 {
		return inviteInvalid, nil
	}
	records, err := t.clientService.GetRecordsBySubID(token)
	if err != nil || len(records) == 0 {
		return inviteInvalid, nil
	}
	return classifyInvite(records, fromID), records
}

// One subscription can span several clients, so a token is claimable only when
// no part of it belongs to someone else.
func classifyInvite(records []*model.ClientRecord, fromID int64) inviteOutcome {
	unbound := false
	for _, record := range records {
		switch record.TgID {
		case 0:
			unbound = true
		case fromID:
		default:
			return inviteTaken
		}
	}
	if unbound {
		return inviteBindable
	}
	return inviteAlreadyOwned
}

func (t *Tgbot) claimInvite(chatId int64, fromID int64, token string) {
	outcome, records := t.resolveInviteToken(token, fromID)
	switch outcome {
	case inviteAlreadyOwned:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+recordEmails(records)))
	case inviteBindable:
		if err := t.bindRecordsToUser(records, fromID); err != nil {
			logger.Warning("tgbot: invite bind failed:", err)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+recordEmails(records)))
	default:
		// Unknown and already-claimed tokens share one reply, so a prober cannot
		// tell a valid SubID from an invalid one.
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteInvalid"))
	}
}

func recordEmails(records []*model.ClientRecord) string {
	emails := make([]string, 0, len(records))
	for _, record := range records {
		emails = append(emails, record.Email)
	}
	return strings.Join(emails, ", ")
}

// Every unbound client behind the token is bound, so a subscription spanning
// several inbounds does not leave the customer holding only part of it.
func (t *Tgbot) bindRecordsToUser(records []*model.ClientRecord, tgID int64) error {
	for _, record := range records {
		if record.TgID != 0 {
			continue
		}
		traffic, err := t.inboundService.GetClientTrafficByEmail(record.Email)
		if err != nil {
			return err
		}
		if traffic == nil {
			return common.NewError("no traffic record for client:", record.Email)
		}
		needRestart, err := t.clientService.SetClientTelegramUserID(&t.inboundService, traffic.Id, tgID)
		if needRestart {
			t.xrayService.SetToNeedRestart()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tgbot) sendInviteLink(chatId int64, email string) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	username := botUsername()
	if err != nil || record.SubID == "" || username == "" {
		logger.Warning("tgbot: invite link unavailable for", email, err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	link := "https://t.me/" + username + "?start=" + record.SubID
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteLink", "Email=="+email, "Link=="+link))
}
