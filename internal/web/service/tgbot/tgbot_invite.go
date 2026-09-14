package tgbot

import (
	"encoding/base64"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mymmrac/telego"
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

// Claims run on concurrent handlers, so resolving and binding happen under one
// lock: a second claimant must see the first one's binding, not the rows it read.
var inviteClaimMu sync.Mutex

// claimInvite reports the outcome it told the user, bindErr aside, so a caller
// can tell a bind that landed from a refusal without reading the reply.
func (t *Tgbot) claimInvite(chatId int64, fromID int64, payload string) inviteOutcome {
	token, ok := decodeInvitePayload(payload)
	if !ok {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteInvalid"))
		return inviteInvalid
	}

	inviteClaimMu.Lock()
	outcome, records := t.resolveInviteToken(token, fromID)
	var bindErr error
	if outcome == inviteBindable {
		bindErr = t.bindRecordsToUser(records, fromID)
	}
	inviteClaimMu.Unlock()

	switch outcome {
	case inviteAlreadyOwned:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+recordEmails(records)))
	case inviteBindable:
		if bindErr != nil {
			logger.Warning("tgbot: invite bind failed:", bindErr)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
			return inviteInvalid
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+recordEmails(records)))
	default:
		// Unknown and already-claimed tokens share one reply, so a prober cannot
		// tell a valid SubID from an invalid one.
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteInvalid"))
	}
	return outcome
}

func recordEmails(records []*model.ClientRecord) string {
	emails := make([]string, 0, len(records))
	for _, record := range records {
		emails = append(emails, record.Email)
	}
	return strings.Join(emails, ", ")
}

// Every unbound client behind the token is bound, so a subscription spanning
// several inbounds does not leave the customer holding only part of it. A failure
// part-way undoes this claim's bindings, so the reply never hides a half-bind.
func (t *Tgbot) bindRecordsToUser(records []*model.ClientRecord, tgID int64) error {
	var bound []int
	for _, record := range records {
		if record.TgID != 0 {
			continue
		}
		traffic, err := t.inboundService.GetClientTrafficByEmail(record.Email)
		if err == nil && traffic == nil {
			err = common.NewError("no traffic record for client:", record.Email)
		}
		if err == nil {
			err = t.setClientTgID(traffic.Id, tgID)
		}
		if err != nil {
			for _, trafficID := range bound {
				if undoErr := t.setClientTgID(trafficID, EmptyTelegramUserID); undoErr != nil {
					logger.Warning("tgbot: undoing partial invite bind failed:", undoErr)
				}
			}
			return err
		}
		bound = append(bound, traffic.Id)
	}
	return nil
}

func (t *Tgbot) setClientTgID(trafficID int, tgID int64) error {
	needRestart, err := t.clientService.SetClientTelegramUserID(&t.inboundService, trafficID, tgID)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	return err
}

// Telegram accepts only A-Za-z0-9_- in a start payload, at most 64 characters,
// while a subId may hold '#', '&' or non-ASCII; base64url carries any subId
// that fits intact instead of letting the link truncate it into another one.
const maxInvitePayload = 64

func encodeInvitePayload(subID string) (string, bool) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(subID))
	return payload, len(payload) <= maxInvitePayload
}

func decodeInvitePayload(payload string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil || len(raw) == 0 {
		return "", false
	}
	return string(raw), true
}

func (t *Tgbot) sendInviteLink(chatId int64, email string) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	username := botUsername()
	if err != nil || record.SubID == "" || username == "" {
		logger.Warning("tgbot: invite link unavailable for", email, err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	payload, ok := encodeInvitePayload(record.SubID)
	if !ok {
		logger.Warning("tgbot: subId of", email, "is too long for a Telegram invite link")
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	link := "https://t.me/" + username + "?start=" + payload
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteLink", "Email=="+email, "Link=="+link))
}

// A subId can be short or human-readable, so claim attempts are capped per
// Telegram account: guessing stays slow, and admins hear about whoever tries.
const (
	inviteAttemptLimit  = 5
	inviteAttemptWindow = time.Hour
)

type inviteAttempts struct {
	windowStart time.Time
	count       int
}

var (
	inviteAttemptsMu  sync.Mutex
	inviteAttemptsBy  = map[int64]*inviteAttempts{}
	inviteAttemptsNow = time.Now
)

// allowInviteAttempt counts one claim attempt and reports whether it may run.
// Admins are told once per window, on the first attempt past the limit.
func (t *Tgbot) allowInviteAttempt(from *telego.User) bool {
	now := inviteAttemptsNow()
	inviteAttemptsMu.Lock()
	for id, a := range inviteAttemptsBy {
		if now.Sub(a.windowStart) >= inviteAttemptWindow {
			delete(inviteAttemptsBy, id)
		}
	}
	a, ok := inviteAttemptsBy[from.ID]
	if !ok {
		a = &inviteAttempts{windowStart: now}
		inviteAttemptsBy[from.ID] = a
	}
	a.count++
	count := a.count
	inviteAttemptsMu.Unlock()

	if count == inviteAttemptLimit+1 {
		t.SendMsgToTgbotAdmins(t.I18nBot("tgbot.messages.inviteRateLimitedAdmin",
			"User=="+tgUserMention(from),
			"ID=="+strconv.FormatInt(from.ID, 10),
			"Limit=="+strconv.Itoa(inviteAttemptLimit)))
	}
	return count <= inviteAttemptLimit
}

func tgUserMention(from *telego.User) string {
	id := strconv.FormatInt(from.ID, 10)
	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	if name == "" {
		name = id
	}
	mention := `<a href="tg://user?id=` + id + `">` + html.EscapeString(name) + `</a>`
	if from.Username != "" {
		mention += " @" + html.EscapeString(from.Username)
	}
	return mention
}
