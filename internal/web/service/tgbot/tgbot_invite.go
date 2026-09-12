package tgbot

import (
	"html"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type inviteOutcome int

const (
	inviteInvalid inviteOutcome = iota
	inviteTaken
	inviteAlreadyOwned
	inviteBindable
	inviteHasClient
)

// A client's SubID doubles as its invite token. Unknown and already-claimed
// tokens share one reply so a prober cannot tell valid SubIDs from invalid ones.
func (t *Tgbot) resolveInviteToken(token string, fromID int64) (inviteOutcome, []*model.ClientRecord) {
	token = strings.TrimSpace(token)
	if token == "" || fromID <= 0 {
		return inviteInvalid, nil
	}
	records, err := t.clientService.GetRecordsBySubID(token)
	if err != nil || len(records) == 0 {
		return inviteInvalid, nil
	}

	// One subscription can span several clients, so a token is only claimable
	// when no part of it belongs to a third party.
	owned := false
	for _, record := range records {
		switch record.TgID {
		case 0:
		case fromID:
			owned = true
		default:
			return inviteTaken, records
		}
	}
	for _, record := range records {
		if record.TgID == 0 {
			return inviteBindable, records
		}
	}
	if owned {
		return inviteAlreadyOwned, records
	}
	return inviteInvalid, records
}

func (t *Tgbot) claimInvite(chatId int64, from *telego.User, token string, isAdmin bool) {
	fromID := from.ID
	outcome, records := t.resolveInviteToken(token, fromID)

	// One account may hold several subscriptions — a customer often manages a
	// relative's config — but only up to the ceiling an admin set.
	if outcome == inviteBindable && !isAdmin {
		if allowed, limit := t.bindingHeadroom(fromID, records); !allowed {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBindLimit", "Limit=="+bindLimitLabel(limit)))
			return
		}
	}

	switch outcome {
	case inviteAlreadyOwned:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+recordEmails(records)))
	case inviteBindable:
		// Read before binding: afterwards every record names this account, so
		// a first arrival and a re-tap would look the same.
		fresh := freshBindings(records)
		bound, err := t.bindRecordsToUser(records, fromID)
		if err != nil {
			logger.Warning("tgbot: invite bind failed:", err)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+strings.Join(bound, ", ")))
		if len(fresh) > 0 {
			t.notifyNewClient(from, fresh, t.heldSubscriptions(fromID))
		}
	default:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteInvalid"))
		// The deliberately vague reply above leaves an admin with nothing to go
		// on, so they alone also get the reason the token was refused.
		if isAdmin {
			t.SendMsgToTgbot(chatId, t.inviteDiagnosis(token, records))
		}
	}
}

// Names each client behind the token that is already spoken for, so the admin
// can see which Telegram account to chase rather than guessing.
func inviteHolders(records []*model.ClientRecord) string {
	held := make([]string, 0, len(records))
	for _, record := range records {
		if record.TgID != 0 {
			held = append(held, record.Email+" → "+strconv.FormatInt(record.TgID, 10))
		}
	}
	return strings.Join(held, ", ")
}

func (t *Tgbot) inviteDiagnosis(token string, records []*model.ClientRecord) string {
	holders := inviteHolders(records)
	if holders == "" {
		return t.I18nBot("tgbot.messages.inviteDiagUnknown", "Token=="+token)
	}
	return t.I18nBot("tgbot.messages.inviteDiagTaken", "Detail=="+holders)
}

func recordEmails(records []*model.ClientRecord) string {
	emails := make([]string, 0, len(records))
	for _, record := range records {
		emails = append(emails, record.Email)
	}
	return strings.Join(emails, ", ")
}

// Every unbound client behind the token is bound, so a subscription spanning
// several inbounds does not leave the customer holding only one of its configs.
func (t *Tgbot) bindRecordsToUser(records []*model.ClientRecord, tgID int64) ([]string, error) {
	bound := make([]string, 0, len(records))
	for _, record := range records {
		if record.TgID == tgID {
			bound = append(bound, record.Email)
			continue
		}
		if record.TgID != 0 {
			continue
		}
		if err := t.bindRecordToUser(record, tgID); err != nil {
			return bound, err
		}
		bound = append(bound, record.Email)
	}
	return bound, nil
}

func (t *Tgbot) bindRecordToUser(record *model.ClientRecord, tgID int64) error {
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
	return err
}

func (t *Tgbot) inviteLinkFor(email string) (string, error) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return "", err
	}
	if record.SubID == "" {
		return "", common.NewError("client has no subId:", email)
	}
	username := botUsername()
	if username == "" {
		return "", common.NewError("bot username unavailable")
	}
	return "https://t.me/" + username + "?start=" + record.SubID, nil
}

func (t *Tgbot) sendInviteLink(chatId int64, email string) {
	link, err := t.inviteLinkFor(email)
	if err != nil {
		logger.Warning("tgbot: invite link failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteLink", "Email=="+email, "Link=="+link))
}

// Records with no Telegram account are the ones a claim actually binds, so they
// alone mark a customer arriving rather than returning.
func freshBindings(records []*model.ClientRecord) []string {
	var fresh []string
	for _, record := range records {
		if record.TgID == 0 {
			fresh = append(fresh, record.Email)
		}
	}
	return fresh
}

// Carries a tappable mention so an admin can open the chat, and the numeric id
// so they can still find the account if the display name is unusable.
func (t *Tgbot) newClientNotice(tgID int64, firstName, username string, emails []string, held int) string {
	id := strconv.FormatInt(tgID, 10)
	name := html.EscapeString(firstName)
	if name == "" {
		name = id
	}
	mention := `<a href="tg://user?id=` + id + `">` + name + `</a>`
	if username != "" {
		mention += " (@" + html.EscapeString(username) + ")"
	}

	notice := t.I18nBot("tgbot.messages.newClientHeader") + "\r\n" +
		mention + " · <code>" + id + "</code>\r\n" +
		html.EscapeString(strings.Join(emails, ", "))

	// Named only once the account holds more than the config that just bound,
	// so an ordinary first arrival reads exactly as it always did.
	if held > 1 {
		notice += "\r\n" + t.I18nBot("tgbot.messages.newClientHolding", "Count=="+strconv.Itoa(held))
	}
	return notice
}

// A lookup failure notifies anyway: an admin ignoring a notice is cheaper than
// silently missing every customer who arrives.
func (t *Tgbot) notifyNewClient(from *telego.User, emails []string, held int) {
	enabled, err := t.settingService.GetTgBotNotifyNewClient()
	if err != nil {
		logger.Warning("tgbot: new-client notification setting lookup failed:", err)
		enabled = true
	}
	if !enabled {
		return
	}

	replyKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.replyToClient")).
				WithCallbackData(t.encodeQuery("pm_reply " + strconv.FormatInt(from.ID, 10))),
		),
	)
	for _, adminId := range adminIds {
		scoped := t.forUser(adminId)
		scoped.SendMsgToTgbot(adminId, scoped.newClientNotice(from.ID, from.FirstName, from.Username, emails, held), replyKeyboard)
	}
}
