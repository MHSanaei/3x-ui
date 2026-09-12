package tgbot

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// A roster longer than this pages out over a dozen Telegram messages, which is
// worse than useless; the sort puts the entries worth reading at the top.
const rosterPageSize = 30

// A zero expiry means the client never lapses and a negative one means its
// window has not started, so neither may sort ahead of a real deadline.
func expirySortKey(expiryTime int64) int64 {
	if expiryTime > 0 {
		return expiryTime
	}
	return math.MaxInt64
}

func sortClientsByExpiry(clients []service.ClientWithAttachments) {
	slices.SortStableFunc(clients, func(a, b service.ClientWithAttachments) int {
		if diff := cmp.Compare(expirySortKey(a.ExpiryTime), expirySortKey(b.ExpiryTime)); diff != 0 {
			return diff
		}
		return cmp.Compare(a.Email, b.Email)
	})
}

func rosterPage(clients []service.ClientWithAttachments, limit int) ([]service.ClientWithAttachments, int) {
	if limit <= 0 || len(clients) <= limit {
		return clients, 0
	}
	return clients[:limit], len(clients) - limit
}

func (t *Tgbot) rosterEntry(client *service.ClientWithAttachments) string {
	bound := "❌"
	if client.TgID != 0 {
		bound = "✅"
	}

	expiry := t.I18nBot("tgbot.unlimited")
	switch {
	case client.ExpiryTime > 0:
		expiry = time.Unix(client.ExpiryTime/1000, 0).Format("2006-01-02")
	case client.ExpiryTime < 0:
		expiry = t.I18nBot("tgbot.messages.rosterNotStarted")
	}

	used := int64(0)
	if client.Traffic != nil {
		used = client.Traffic.Up + client.Traffic.Down
	}
	total := t.I18nBot("tgbot.unlimited")
	if client.TotalGB > 0 {
		total = common.FormatTraffic(client.TotalGB)
	}

	return t.I18nBot("tgbot.messages.rosterEntry",
		"Bound=="+bound,
		"Email=="+client.Email,
		"Expiry=="+expiry,
		"Used=="+common.FormatTraffic(used),
		"Total=="+total,
	)
}

// Answers "who is on this panel and who lapses next?" in one message, the
// overview an admin otherwise has to open the web panel for.
func (t *Tgbot) clientRoster(chatId int64) {
	t.clientRosterFiltered(chatId, rosterFilterAll)
}

func (t *Tgbot) clientRosterFiltered(chatId int64, filter string) {
	clients, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: client roster failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	matched := filterClients(clients, filter, time.Now())
	t.sendRoster(chatId, matched, t.rosterFilterKeyboard(filter))
}

func (t *Tgbot) rosterSearchResults(chatId int64, query string) {
	clients, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: client roster search failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	t.sendRoster(chatId, searchClients(clients, query), t.rosterFilterKeyboard(""))
}

func (t *Tgbot) sendRoster(chatId int64, clients []service.ClientWithAttachments, keyboard *telego.InlineKeyboardMarkup) {
	if len(clients) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.rosterEmpty"), keyboard)
		return
	}

	sortClientsByExpiry(clients)
	page, omitted := rosterPage(clients, rosterPageSize)

	var report strings.Builder
	report.WriteString(t.I18nBot("tgbot.messages.rosterHeader", "Count=="+strconv.Itoa(len(clients))))
	for i := range page {
		report.WriteString("\r\n\r\n")
		report.WriteString(t.rosterEntry(&page[i]))
	}
	if omitted > 0 {
		report.WriteString("\r\n\r\n")
		report.WriteString(t.I18nBot("tgbot.messages.rosterTruncated", "Count=="+strconv.Itoa(omitted)))
	}
	t.SendMsgToTgbot(chatId, report.String(), keyboard)
}

// The roster is admin-only, so the typed query is re-checked against the
// sender rather than the chat the state was stored under.
func (t *Tgbot) startRosterSearch(chatId int64) {
	userStateMgr.set(chatId, stateRosterSearch)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.rosterSearchPrompt"))
}

// Handing a customer their invite link is the commonest admin errand, so the picker
// lists clients directly rather than making the operator recall an email first.
func (t *Tgbot) inviteLinkPicker(chatId int64, messageID int) {
	clients, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: invite picker failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if len(clients) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.rosterEmpty"))
		return
	}

	sortClientsByExpiry(clients)
	page, omitted := rosterPage(clients, rosterPageSize)

	buttons := make([]telego.InlineKeyboardButton, 0, len(page))
	for i := range page {
		mark := "❌"
		if page[i].TgID != 0 {
			mark = "✅"
		}
		buttons = append(buttons, tu.InlineKeyboardButton(mark+" "+page[i].Email).
			WithCallbackData(t.encodeQuery("client_invite_link "+page[i].Email)))
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(2, buttons...))

	header := t.I18nBot("tgbot.messages.invitePicker", "Count=="+strconv.Itoa(len(clients)))
	if omitted > 0 {
		header += "\r\n" + t.I18nBot("tgbot.messages.rosterTruncated", "Count=="+strconv.Itoa(omitted))
	}
	if messageID > 0 {
		t.editMessageTgBot(chatId, messageID, header, keyboard)
		return
	}
	t.SendMsgToTgbot(chatId, header, keyboard)
}
