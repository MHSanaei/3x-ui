package tgbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// The bot's lists are browsed in inline mode, which must be enabled by hand in
// @BotFather; when off, screens show a hint rather than a silent dead button.

// The launcher carries its own intent as the query text (empty = all inbounds,
// "cl" = every client, "cl <id>" = one inbound's), so nothing is kept server-side.
const (
	inlineScopeInbounds = ""
	inlineScopeClients  = "cl"
)

// scopeQuery is the inline-query text the launcher opens with.
func scopeQuery(kind, data string) string {
	if kind == inlineScopeClients && data != "" {
		return inlineScopeClients + " " + data
	}
	if kind == inlineScopeClients {
		return inlineScopeClients
	}
	return ""
}

// inlineSupport caches getMe().supports_inline_queries. A permanent cache would
// freeze a pre-/setinline False forever, so it refreshes once stale.
type inlineSupport struct {
	mu    sync.Mutex
	ok    bool
	known bool
	at    time.Time
}

var inlineCapability = &inlineSupport{}

const inlineCapabilityTTL = 10 * time.Minute

// recordInlineCapability stores what getMe() reported.
func recordInlineCapability(supported bool) {
	inlineCapability.mu.Lock()
	inlineCapability.ok = supported
	inlineCapability.known = true
	inlineCapability.at = time.Now()
	inlineCapability.mu.Unlock()
}

// inlineSupported reports whether the launcher may be shown. A stale value is
// served and refreshed off-path; an unknown empty cache must not warn (post-restart bug).
func (t *Tgbot) inlineSupported() bool {
	inlineCapability.mu.Lock()
	ok, known, at := inlineCapability.ok, inlineCapability.known, inlineCapability.at
	inlineCapability.mu.Unlock()

	if known && time.Since(at) <= inlineCapabilityTTL {
		return ok
	}
	if !known {
		t.refreshInlineCapability()
		inlineCapability.mu.Lock()
		defer inlineCapability.mu.Unlock()
		return inlineCapability.ok
	}
	go t.refreshInlineCapability()
	return ok
}

func (t *Tgbot) refreshInlineCapability() {
	if liveBot() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	me, err := liveBot().GetMe(ctx)
	if err != nil {
		logger.Debug("Cannot read the bot capability for inline mode:", err)
		return
	}
	recordInlineCapability(me.SupportsInlineQueries)
}

// launchRow is the one way into a list: a launcher that opens inline mode in
// this chat, or the hint that says why there is none.
func (t *Tgbot) launchRow(query string, labelKey string) []telego.InlineKeyboardButton {
	if !t.inlineSupported() {
		return tu.InlineKeyboardRow(t.btn("tgbot.buttons.inlineDisabled", "inline_help"))
	}
	// Telegram prefills and returns this token as the query, so it is both intent
	// and filter prefix; not localized because a translated marker could not parse.
	btn := telego.InlineKeyboardButton{
		Text:                         t.I18nBot(labelKey),
		SwitchInlineQueryCurrentChat: &query,
	}
	return tu.InlineKeyboardRow(btn)
}

// Capabilities is the bot's own answer to "can you show lists?": inline mode is
// a BotFather switch, so the panel can only warn, never enable it.
type Capabilities struct {
	InlineEnabled bool
	// GroupPrivacy means the bot CANNOT read every group message, so a /start
	// typed in a group never reaches it unless the message mentions the liveBot().
	GroupPrivacy bool
	Username     string
	Running      bool
}

// Capabilities probes the live bot for what it can do.
func (t *Tgbot) Capabilities() (Capabilities, error) {
	caps := Capabilities{Running: t.IsRunning(), Username: botUsername()}
	if liveBot() == nil {
		return caps, errors.New("bot is not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	me, err := liveBot().GetMe(ctx)
	if err != nil {
		return caps, err
	}
	caps.InlineEnabled = me.SupportsInlineQueries
	// can_read_all_group_messages is the inverse of BotFather's Group Privacy.
	caps.GroupPrivacy = !me.CanReadAllGroupMessages
	caps.Username = me.Username
	recordInlineCapability(me.SupportsInlineQueries)
	return caps, nil
}

// screenInlineHelp explains a bot that cannot open lists: inline mode is a
// BotFather switch, so the screen tells the admin exactly which one to flip.
func (t *Tgbot) screenInlineHelp(chatID int64) {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.inline.recheck", "inline_recheck")),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("commands",
		t.I18nBot("tgbot.inline.offTitle")+"\r\n\r\n"+t.I18nBot("tgbot.inline.offBody"), rows...))
}

// handleInlineQuery answers one query with the items of its scope.
func (t *Tgbot) handleInlineQuery(query *telego.InlineQuery) {
	if liveBot() == nil {
		return
	}
	level := t.levelOf(query.From.ID)
	if level == levelStranger {
		t.answerInline(query.ID, nil, "")
		return
	}
	// Telegram prefills the typed text with the launcher's query, so the intent
	// arrives before anything the user adds; a bare query means inbounds.
	action, data, search := parseInlineQuery(query.Query)
	results := t.inlineResults(action, data, search, level == levelAdmin)
	t.answerInline(query.ID, results, query.Offset)
}

// maxInlineResults is the Bot API's hard cap on one answer. Exceeding it gets
// the whole answer rejected, which on a big panel means an empty list forever.
const maxInlineResults = 50

// pageInlineResults returns one page and the cursor for the next. The full list
// is built first because the cap is a property of the transport, not of a list.
func pageInlineResults(results []telego.InlineQueryResult, offset string) (page []telego.InlineQueryResult, next string) {
	start, err := strconv.Atoi(offset)
	if err != nil || start < 0 {
		start = 0
	}
	if start >= len(results) {
		return nil, ""
	}
	end := start + maxInlineResults
	if end < len(results) {
		return results[start:end], strconv.Itoa(end)
	}
	return results[start:], ""
}

// markerResultID keeps a result id inside the 64-byte cap Telegram enforces: one
// long email would otherwise sink the entire answer.
func markerResultID(marker string) string {
	if len(marker) <= 63 {
		return marker
	}
	sum := sha256.Sum256([]byte(marker))
	return "h:" + hex.EncodeToString(sum[:16])
}

// answerInline answers with page, or with the "nothing here" card: an
// unanswered query leaves Telegram spinning.
func (t *Tgbot) answerInline(queryID string, results []telego.InlineQueryResult, offset string) {
	page, next := pageInlineResults(results, offset)
	if len(results) == 0 {
		hint := t.I18nBot("tgbot.inline.noneTitle")
		page = []telego.InlineQueryResult{&telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          "none",
			Title:       hint,
			Description: t.I18nBot("tgbot.inline.noneDesc"),
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: "none:" + hint,
			},
		}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := liveBot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
		InlineQueryID: queryID,
		Results:       page,
		NextOffset:    next,
		CacheTime:     0,
		IsPersonal:    true,
	})
	if err != nil {
		// A stale query id is normal (the user kept typing); never a failure.
		logger.Debug("answerInlineQuery failed:", err)
	}
}

// inlineResults builds the items for one inline query.
func (t *Tgbot) inlineResults(action, data, search string, isAdmin bool) []telego.InlineQueryResult {
	switch action {
	case inlineScopeClients:
		return t.clientResults(search, isAdmin, data)
	case "add":
		return t.inboundResults(search, "add")
	}
	return t.inboundResults(search, "inb")
}

// parseInlineQuery splits "cl 3 user@x" into its action, inbound and the text to
// filter by. Nothing found means the inbounds list.
func parseInlineQuery(text string) (action, data, search string) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{inlineScopeClients, "add"} {
		if !strings.HasPrefix(lower, marker) {
			continue
		}
		rest := strings.TrimSpace(trimmed[len(marker):])
		if marker == inlineScopeClients {
			// "cl 3 name" — the first token is the inbound when it is a number.
			fields := strings.SplitN(rest, " ", 2)
			if len(fields) > 0 && fields[0] != "" {
				if _, err := strconv.Atoi(fields[0]); err == nil {
					if len(fields) == 2 {
						return marker, fields[0], fields[1]
					}
					return marker, fields[0], ""
				}
			}
			return marker, "", rest
		}
		return marker, "", rest
	}
	return inlineScopeInbounds, "", trimmed
}

// inboundResults lists inbounds as markers. action is what a chosen item means:
// "inb" opens the inbound, "add" starts the add-client wizard on it.
func (t *Tgbot) inboundResults(search, action string) []telego.InlineQueryResult {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Cannot list inbounds for inline mode:", err)
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(search))
	results := make([]telego.InlineQueryResult, 0, len(inbounds))
	for _, ib := range inbounds {
		remark := ib.Remark
		if remark == "" {
			remark = "#" + strconv.Itoa(ib.Id)
		}
		status := "❌"
		if ib.Enable {
			status = "✅"
		}
		description := fmt.Sprintf("#%d · %s · %s · 🚦 %s\n👥 %d", ib.Id, ib.Protocol, status,
			common.FormatTraffic(ib.Up+ib.Down), len(ib.ClientStats))
		if needle != "" && !strings.Contains(strings.ToLower(remark+" "+string(ib.Protocol)+" "+strconv.Itoa(ib.Port)), needle) {
			continue
		}
		marker := action + ":" + strconv.Itoa(ib.Id)
		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          markerResultID(marker),
			Title:       remark,
			Description: description,
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: marker,
			},
		})
	}
	return results
}

// clientResults lists clients of one inbound, or of all of them when inboundID
// is empty: an admin browses the panel, a client only ever sees its own.
func (t *Tgbot) clientResults(search string, isAdmin bool, inboundID string) []telego.InlineQueryResult {
	needle := strings.ToLower(strings.TrimSpace(search))
	results := make([]telego.InlineQueryResult, 0, 32)

	add := func(email, remark string, used, limit, expiry int64, enabled, online bool) {
		if needle != "" && !strings.Contains(strings.ToLower(email+" "+remark), needle) {
			return
		}
		marker := "cl:" + email
		results = append(results, &telego.InlineQueryResultArticle{
			Type:  "article",
			ID:    markerResultID(marker),
			Title: email, // the title is the one thing every client shows
			// Telegram renders about three lines here, so the three facts a
			// client screen answers first get one line each.
			Description: t.clientCardLines(remark, used, limit, expiry, enabled, online),
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: marker,
			},
		})
	}

	if !isAdmin {
		return results
	}

	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Cannot list clients for inline mode:", err)
		return results
	}
	seen := map[string]bool{}
	for _, ib := range inbounds {
		if inboundID != "" && strconv.Itoa(ib.Id) != inboundID {
			continue
		}
		clients, err := t.inboundService.GetClients(ib)
		if err != nil {
			continue
		}
		for _, client := range clients {
			if seen[client.Email] {
				continue
			}
			seen[client.Email] = true
			used := int64(0)
			if traffic, err := t.inboundService.GetClientTrafficByEmail(client.Email); err == nil && traffic != nil {
				used = traffic.Up + traffic.Down
			}
			add(client.Email, ib.Remark, used, client.TotalGB, client.ExpiryTime, client.Enable, false)
		}
	}
	return results
}

// clientCardLines is an inline result's description: three lines, because that
// is what Telegram renders under the title.
func (t *Tgbot) clientCardLines(remark string, used, limit, expiry int64, enabled, online bool) string {
	head := "❌"
	if enabled {
		head = "✅"
	}
	if online {
		head += " 🟢"
	}
	if remark != "" {
		head += " · " + remark
	}

	traffic := common.FormatTraffic(used)
	if limit > 0 {
		// The percentage is what people actually compare between clients.
		traffic += " / " + common.FormatTraffic(limit) +
			fmt.Sprintf(" · %d%%", used*100/limit)
	} else {
		traffic += " / " + t.I18nBot("tgbot.unlimited")
	}

	expiryText := t.I18nBot("tgbot.unlimited")
	switch {
	case expiry < 0:
		// A negative term is days counted from first use, not a date.
		expiryText = t.durationText(-expiry*86400000, true)
	case expiry > 0:
		expiryText = t.durationText(expiry-time.Now().UnixMilli(), false)
	}
	return head + "\n🚦 " + traffic + "\n📅 " + expiryText
}

// durationText is a compact, localized span: "12 Minutes", "5 Hours", "30 Days".
// fromFirstUse names the not-yet-started case, whose clock has not begun.
func (t *Tgbot) durationText(ms int64, fromFirstUse bool) string {
	if ms <= 0 {
		return t.I18nBot("tgbot.inline.expired")
	}
	if fromFirstUse {
		return fmt.Sprintf("%d %s (%s)", ms/86400000, t.I18nBot("tgbot.days"),
			t.I18nBot("tgbot.inline.fromFirstUse"))
	}
	switch {
	case ms >= 30*86400000:
		return fmt.Sprintf("%d %s", ms/(30*86400000), t.I18nBot("tgbot.months"))
	case ms >= 86400000:
		return fmt.Sprintf("%d %s", ms/86400000, t.I18nBot("tgbot.days"))
	case ms >= 3600000:
		return fmt.Sprintf("%d %s", ms/3600000, t.I18nBot("tgbot.hours"))
	default:
		return fmt.Sprintf("%d %s", ms/60000, t.I18nBot("tgbot.minutes"))
	}
}

// handleListMarker catches an item picked in inline mode. The marker arrives as
// the user's own message, so it is deleted first, then the real card is drawn.
func (t *Tgbot) handleListMarker(message *telego.Message) bool {
	if message == nil || message.Text == "" {
		return false
	}
	action, value, ok := splitListMarker(message.Text)
	if !ok {
		return false
	}

	level := t.levelOf(message.From.ID)
	if level == levelStranger {
		return true
	}
	if !t.markerAllowed(action, value, message.From.ID, level == levelAdmin) {
		return true
	}
	// Only a marker that actually opens something is litter worth removing.
	t.deleteIncoming(message)

	switch action {
	case "inb":
		id, err := strconv.Atoi(value)
		if err != nil {
			return true
		}
		t.screenInboundClients(message.Chat.ID, id)
	case "add":
		id, err := strconv.Atoi(value)
		if err != nil {
			return true
		}
		t.startAddClientForInbound(message.Chat.ID, message.From.ID, id)
	case "cl":
		if t.clientOwnedByTgUser(message.From.ID, value) || level == levelAdmin {
			t.searchClientScreen(message.Chat.ID, value)
		}
	}
	return true
}

// markerAllowed checks the marker against the sender: an admin marker needs the
// admin level, a client marker must name a client that account owns.
func (t *Tgbot) markerAllowed(action, value string, tgUserID int64, isAdmin bool) bool {
	switch action {
	case "inb", "add":
		return isAdmin
	case "cl":
		if isAdmin {
			return true
		}
		return t.clientOwnedByTgUser(tgUserID, value)
	}
	return false
}

// messageIsListMarker matches only a picked inline item; a "text contains a
// colon" predicate once swallowed broadcasts and wizard messages (URL, time, comment).
func messageIsListMarker(_ context.Context, update telego.Update) bool {
	if update.Message == nil {
		return false
	}
	_, _, ok := splitListMarker(update.Message.Text)
	return ok
}

// splitListMarker parses "inb:3", "add:3" or "cl:user@x".
func splitListMarker(text string) (action, value string, ok bool) {
	for _, candidate := range []string{"inb:", "add:", "cl:"} {
		if rest, found := strings.CutPrefix(text, candidate); found && rest != "" {
			return strings.TrimSuffix(candidate, ":"), rest, true
		}
	}
	return "", "", false
}

// screenInboundClients draws one inbound's clients, listed by the inline query.
func (t *Tgbot) screenInboundClients(chatID int64, inboundID int) {
	inbound, err := t.inboundService.GetInbound(inboundID)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.getInboundsFailed"))
		return
	}
	// The clients are browsed in inline mode, never as a callback-button grid:
	// a big inbound would otherwise fill the screen with one button per client.
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow(scopeQuery(inlineScopeClients, strconv.Itoa(inboundID)), "tgbot.buttons.searchClients"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("clients", t.inboundSummaryText(inbound), rows...))
}

// screenAllClients is the panel-wide client browser. There is no inbound step:
// the list is every client, and a tap opens the client's own screen.
func (t *Tgbot) screenAllClients(chatID int64) {
	body := t.I18nBot("tgbot.messages.clientsCount", "Count=="+strconv.Itoa(t.clientCount()))
	body += t.I18nBot("tgbot.messages.inlineHint")
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow(scopeQuery(inlineScopeClients, ""), "tgbot.buttons.searchClients"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("clients", body, rows...))
}

// clientCount is how many clients the panel holds, for the browser's header.
func (t *Tgbot) clientCount() int {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Cannot count clients for the browser:", err)
		return 0
	}
	seen := map[string]bool{}
	for _, ib := range inbounds {
		clients, err := t.inboundService.GetClients(ib)
		if err != nil {
			continue
		}
		for _, client := range clients {
			seen[client.Email] = true
		}
	}
	return len(seen)
}

// inboundSummaryText is one inbound's header line: remark, protocol, port.
func (t *Tgbot) inboundSummaryText(inbound *model.Inbound) string {
	body := t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
	body += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
	body += t.I18nBot("tgbot.messages.traffic",
		"Total=="+common.FormatTraffic(inbound.Up+inbound.Down),
		"Upload=="+common.FormatTraffic(inbound.Up),
		"Download=="+common.FormatTraffic(inbound.Down))
	if inbound.ExpiryTime == 0 {
		body += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
	} else {
		body += t.I18nBot("tgbot.messages.expire",
			"Time=="+time.Unix(inbound.ExpiryTime/1000, 0).Format("2006-01-02 15:04:05"))
	}
	body += t.I18nBot("tgbot.messages.clientsCount", "Count=="+strconv.Itoa(len(inbound.ClientStats)))
	return body
}

// startAddClientForInbound opens the add-client wizard on a chosen inbound.
func (t *Tgbot) startAddClientForInbound(chatID, userID int64, inboundID int) {
	if _, err := t.inboundService.GetInbound(inboundID); err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.getInboundsFailed"))
		return
	}
	draft := addClientDrafts.forActor(chatUser{chatID: chatID, userID: userID})
	draft.Lock()
	defer draft.Unlock()
	t.resetDraft(draft)
	draft.receiverInboundID = inboundID
	draft.receiverInboundIDs = []int{inboundID}
	t.addClient(chatID, draft, t.BuildClientDraftMessage(draft))
}

// resetDraft clears the wizard back to its defaults for a fresh client.
func (t *Tgbot) resetDraft(draft *clientDraft) {
	draft.email = t.randomLowerAndNum(8)
	draft.limitIP = 0
	draft.totalGB = 0
	draft.expiryTime = 0
	draft.enable = true
	draft.tgID = ""
	draft.subID = t.randomLowerAndNum(16)
	draft.comment = ""
	draft.reset = 0
}
