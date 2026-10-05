package tgbot

import (
	"context"
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

// The bot's lists are browsed in inline mode, one marker per item. Inline mode
// has to be enabled by hand in @BotFather; when it is off the screens show a
// hint instead of a launcher, because the buttons the old bot used no longer
// exist and a silent dead button is worse than an instruction.

// inlineScope names what the user's inline query should list. It is remembered
// per chat when a list screen is drawn: an inline query carries no chat, so the
// launcher has to say what it is looking for.
type inlineScope struct {
	kind string // "inb" — inbounds, "cl" — clients, "add" — inbounds for a new client
	// data is what a client scope browses: an inbound id, empty for all clients.
	data string
	at   time.Time
}

type inlineScopeStore struct {
	mu     sync.Mutex
	scopes map[int64]inlineScope
}

var inlineScopes = &inlineScopeStore{scopes: map[int64]inlineScope{}}

func (s *inlineScopeStore) set(tgUserID int64, kind, data string) {
	s.mu.Lock()
	s.scopes[tgUserID] = inlineScope{kind: kind, data: data, at: time.Now()}
	s.mu.Unlock()
}

// get is keyed by the inline query's sender, not by the chat the list was
// opened in: in a group those are different numbers, and a scope written under
// the chat id would leave the query falling back to inbounds forever.
func (s *inlineScopeStore) get(tgUserID int64) inlineScope {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.scopes[tgUserID]
	if !ok || time.Since(entry.at) > 30*time.Minute {
		return inlineScope{}
	}
	return entry
}

func (s *inlineScopeStore) reset() {
	s.mu.Lock()
	s.scopes = map[int64]inlineScope{}
	s.mu.Unlock()
}

// inlineSupport caches getMe().supports_inline_queries. A permanent cache would
// freeze a False recorded before /setinline and hide the launcher forever, so
// the value is refreshed in the background once it goes stale.
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
// served as-is and refreshed off the render path, so a screen never waits on a
// network round trip.
func (t *Tgbot) inlineSupported() bool {
	inlineCapability.mu.Lock()
	ok, known, at := inlineCapability.ok, inlineCapability.known, inlineCapability.at
	inlineCapability.mu.Unlock()

	if !known || time.Since(at) > inlineCapabilityTTL {
		go t.refreshInlineCapability()
	}
	return ok
}

func (t *Tgbot) refreshInlineCapability() {
	if bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	me, err := bot.GetMe(ctx)
	if err != nil {
		logger.Debug("Cannot read the bot capability for inline mode:", err)
		return
	}
	recordInlineCapability(me.SupportsInlineQueries)
}

// launchRow is the one way into a list: a launcher that opens inline mode in
// this chat, or the hint that says why there is none.
func (t *Tgbot) launchRow(scope, labelKey string) []telego.InlineKeyboardButton {
	if !t.inlineSupported() {
		return tu.InlineKeyboardRow(t.btn("tgbot.buttons.inlineDisabled", "inline_help"))
	}
	empty := ""
	btn := telego.InlineKeyboardButton{
		Text:                         t.I18nBot(labelKey),
		SwitchInlineQueryCurrentChat: &empty,
	}
	return tu.InlineKeyboardRow(btn)
}

// Capabilities is the bot's own answer to "can you show lists?": inline mode is
// a BotFather switch, so the panel can only warn, never enable it.
type Capabilities struct {
	InlineEnabled bool
	Username      string
	Running       bool
}

// Capabilities probes the live bot for what it can do.
func (t *Tgbot) Capabilities() (Capabilities, error) {
	caps := Capabilities{Running: t.IsRunning(), Username: botUsername()}
	if bot == nil {
		return caps, errors.New("bot is not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	me, err := bot.GetMe(ctx)
	if err != nil {
		return caps, err
	}
	caps.InlineEnabled = me.SupportsInlineQueries
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
	if bot == nil {
		return
	}
	level := t.levelOf(query.From.ID)
	if level == levelStranger {
		t.answerInline(query.ID, nil)
		return
	}
	scope := inlineScopes.get(query.From.ID)
	results := t.inlineResults(scope, query.Query, level == levelAdmin)
	t.answerInline(query.ID, results)
}

// answerInline answers with results, or with the "nothing here" card: an
// unanswered query leaves Telegram spinning.
func (t *Tgbot) answerInline(queryID string, results []telego.InlineQueryResult) {
	if len(results) == 0 {
		hint := t.I18nBot("tgbot.inline.noneTitle")
		results = []telego.InlineQueryResult{&telego.InlineQueryResultArticle{
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
	err := bot.AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
		InlineQueryID: queryID,
		Results:       results,
		CacheTime:     0,
		IsPersonal:    true,
	})
	if err != nil {
		// A stale query id is normal (the user kept typing); never a failure.
		logger.Debug("answerInlineQuery failed:", err)
	}
}

// inlineResults builds the items for a scope, filtered by the typed text. The
// scope is resolved from the user's id, so the query itself needs no context.
func (t *Tgbot) inlineResults(scope inlineScope, search string, isAdmin bool) []telego.InlineQueryResult {
	switch scope.kind {
	case "cl":
		return t.clientResults(search, isAdmin, scope.data)
	case "add":
		return t.inboundResults(search, "add")
	case "inb":
		return t.inboundResults(search, "inb")
	}
	// No scope: the query was typed without a list screen behind it. Listing
	// every client would leak the panel's names to anyone who can type, and
	// listing inbounds is what the caller most likely meant to search.
	return t.inboundResults(search, "inb")
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
		description := fmt.Sprintf("#%d · %s · %s · %d %s", ib.Id, ib.Protocol, status, len(ib.ClientStats), "👥")
		if needle != "" && !strings.Contains(strings.ToLower(remark+" "+string(ib.Protocol)+" "+strconv.Itoa(ib.Port)), needle) {
			continue
		}
		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          action + ":" + strconv.Itoa(ib.Id),
			Title:       remark,
			Description: description,
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: action + ":" + strconv.Itoa(ib.Id),
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

	add := func(email, remark string, used, limit int64, enabled, online bool) {
		if needle != "" && !strings.Contains(strings.ToLower(email+" "+remark), needle) {
			return
		}
		status := "❌"
		if enabled {
			status = "✅"
		}
		traffic := common.FormatTraffic(used)
		if limit > 0 {
			traffic += " / " + common.FormatTraffic(limit)
		}
		description := strings.TrimSpace(strings.Join([]string{remark, traffic, status, boolMark(online)}, " · "))
		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          "cl:" + email,
			Title:       email,
			Description: description,
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: "cl:" + email,
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
			add(client.Email, ib.Remark, used, client.TotalGB, client.Enable, false)
		}
	}
	return results
}

func boolMark(v bool) string {
	if v {
		return "🟢"
	}
	return ""
}

// handleListMarker catches an item the user picked in inline mode. The marker
// arrives as the user's own message, so it is deleted first and only then is
// the real card drawn.
func (t *Tgbot) handleListMarker(message *telego.Message) bool {
	if message == nil || message.Text == "" {
		return false
	}
	action, value, ok := splitListMarker(message.Text)
	if !ok {
		return false
	}
	t.deleteIncoming(message)

	level := t.levelOf(message.From.ID)
	if level == levelStranger {
		return true
	}
	if !t.markerAllowed(action, value, message.From.ID, level == levelAdmin) {
		return true
	}

	switch action {
	case "inb":
		id, err := strconv.Atoi(value)
		if err != nil {
			return true
		}
		t.screenInboundClients(message.Chat.ID, message.From.ID, id)
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
func (t *Tgbot) screenInboundClients(chatID, userID int64, inboundID int) {
	inbound, err := t.inboundService.GetInbound(inboundID)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.getInboundsFailed"))
		return
	}
	// The clients are browsed in inline mode, never as a callback-button grid:
	// a big inbound would otherwise fill the screen with one button per client.
	inlineScopes.set(userID, "cl", strconv.Itoa(inboundID))
	rows := [][]telego.InlineKeyboardButton{
		t.launchRow("cl", "tgbot.buttons.searchClients"),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("clients", t.inboundSummaryText(inbound), rows...))
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
