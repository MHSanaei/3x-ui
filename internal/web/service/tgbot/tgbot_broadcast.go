package tgbot

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	telegoapi "github.com/mymmrac/telego/telegoapi"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	broadcastAwaitingText = "awaiting_broadcast_text"

	// Pause per recipient, scaled by the copied message count, keeps the run
	// around the bot-wide ~30 msg/s ceiling even for whole albums.
	broadcastSendDelay = 60 * time.Millisecond
	// Progress refreshes are throttled to keep the run under rate limits.
	broadcastProgressEvery    = 25
	broadcastProgressInterval = 3 * time.Second
	broadcastFloodRetries     = 5
	// A long retry_after is slept in slices so cancel and bot state stay checked.
	broadcastFloodWaitSlice = 5 * time.Second
)

// broadcastDraft references the admin's original message: copyMessage relays
// any message type 1:1 on behalf of the bot. An album lists its messages.
type broadcastDraft struct {
	FromChatID int64
	MessageIDs []int
}

// broadcastResult is the end-of-run statistics shown to the admin.
type broadcastResult struct {
	Total       int
	Delivered   int
	Failed      int
	Skipped     int
	Unreachable int
	Canceled    bool
	Elapsed     time.Duration
}

// broadcastRunner tracks the single in-flight broadcast: where to report
// progress and whether the admin asked to stop it.
type broadcastRunner struct {
	actor     chatUser
	chatID    int64
	messageID int
	cancel    atomic.Bool

	mu     sync.Mutex
	result broadcastResult
}

func (r *broadcastRunner) setResult(res broadcastResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.result = res
}

func (r *broadcastRunner) getResult() broadcastResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.result
}

// broadcastCompose is one admin's composition: collected message ids, the
// album group still arriving, and the token binding the preview to its card.
type broadcastCompose struct {
	messageIDs []int
	// inputMessageIDs are the admin's own messages in the chat. They are the
	// SOURCE of the copy, not something a recipient ever sees, so they are
	// removed once the broadcast is confirmed or dropped.
	inputMessageIDs []int
	groupID         string
	token           string
	timer           *time.Timer
}

var (
	broadcastMu       sync.Mutex
	broadcastComposes = make(map[chatUser]*broadcastCompose)
	// broadcastPrompts is the "send me the message" card per admin, remembered
	// only so that leaving the flow takes it out of the chat.
	broadcastPrompts = make(map[chatUser]int)
	broadcastActive  *broadcastRunner
)

// broadcastAlbumDebounce waits out Telegram's stream of one media group: an
// album reaches the bot as separate messages sharing a media_group_id.
var broadcastAlbumDebounce = 900 * time.Millisecond

// errBroadcastAborted reports a recipient abandoned because the run was
// cancelled or the bot stopped, which is not a delivery failure.
var errBroadcastAborted = errors.New("broadcast aborted")

// broadcastResetAll drops every composition and cancels the active run; the
// bot calls it on stop so no timer, token or runner slot outlives the receiver.
func broadcastResetAll() {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	for _, c := range broadcastComposes {
		if c.timer != nil {
			c.timer.Stop()
		}
	}
	broadcastComposes = make(map[chatUser]*broadcastCompose)
	if broadcastActive != nil {
		broadcastActive.cancel.Store(true)
		broadcastActive = nil
	}
	broadcastPrompts = make(map[chatUser]int)
}

func broadcastDropCompose(actor chatUser) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if c := broadcastComposes[actor]; c != nil && c.timer != nil {
		c.timer.Stop()
	}
	delete(broadcastComposes, actor)
}

// broadcastComposeInputs returns the admin's own composing messages for the
// pending draft, empty while an album is still being collected.
func broadcastComposeInputs(actor chatUser) []int {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	c := broadcastComposes[actor]
	if c == nil {
		return nil
	}
	return append([]int(nil), c.inputMessageIDs...)
}

// broadcastPendingDraft reports the ids awaiting an admin's confirmation;
// ok is false while an album is still being collected.
func broadcastPendingDraft(actor chatUser) ([]int, string, bool) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	c := broadcastComposes[actor]
	if c == nil || c.groupID != "" || c.token == "" {
		return nil, "", false
	}
	return append([]int(nil), c.messageIDs...), c.token, true
}

// broadcastTakePending removes the pending draft only when its card token
// matches; ok is false for stale taps or admins with no pending draft.
func broadcastTakePending(actor chatUser, token string) ([]int, bool) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	c := broadcastComposes[actor]
	if c == nil || c.token == "" || c.token != token {
		return nil, false
	}
	delete(broadcastComposes, actor)
	return c.messageIDs, true
}

// broadcastRegisterRunner claims the single broadcast slot; nil means one is
// already running.
func broadcastRegisterRunner(actor chatUser) *broadcastRunner {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if broadcastActive != nil {
		return nil
	}
	broadcastActive = &broadcastRunner{actor: actor, chatID: actor.chatID}
	return broadcastActive
}

// broadcastUnregisterRunner releases the slot only if it is still ours, so a
// stale runner cannot cancel a newer one's registration.
func broadcastUnregisterRunner(runner *broadcastRunner) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if broadcastActive == runner {
		broadcastActive = nil
	}
}

func broadcastCurrentRunner() *broadcastRunner {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	return broadcastActive
}

// broadcastSender delivers one draft to one chat and reports the ids it made;
// swapped out in tests.
var broadcastSender = deliverBroadcastCopy

// broadcastPause stands in for time.Sleep so tests don't wait real seconds.
var broadcastPause = time.Sleep

// startBroadcast answers /broadcast: one broadcast at a time, so a second
// command while one is running is refused instead of queued.
func (t *Tgbot) startBroadcast(actor chatUser) {
	chatId := actor.chatID
	if broadcastCurrentRunner() != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastAlreadyRunning"))
		return
	}
	broadcastDropCompose(actor)
	// A second prompt supersedes the first: twin "send me the message" cards in
	// one chat is the sort of litter this flow is supposed to avoid.
	t.clearBroadcastPrompt(actor)
	userStateMgr.set(actor, broadcastAwaitingText)
	id := t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastAskText"), t.broadcastCancelKeyboard())
	broadcastSetPrompt(actor, id)
}

// broadcastSetPrompt remembers the "send me the message" card, so leaving the
// flow cannot strand it in the chat.
func broadcastSetPrompt(actor chatUser, messageID int) {
	if messageID == 0 {
		return
	}
	broadcastMu.Lock()
	broadcastPrompts[actor] = messageID
	broadcastMu.Unlock()
}

// broadcastTakePrompt returns the pending prompt and forgets it.
func broadcastTakePrompt(actor chatUser) int {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	id := broadcastPrompts[actor]
	delete(broadcastPrompts, actor)
	return id
}

// clearBroadcastPrompt removes the prompt from the chat, if one is pending.
func (t *Tgbot) clearBroadcastPrompt(actor chatUser) {
	if id := broadcastTakePrompt(actor); id != 0 {
		t.deleteScreenMessage(actor.chatID, id)
	}
}

// handleBroadcastInput references the message the admin sent and shows the
// confirmation preview. The router hands over only the admin /broadcast awaits.
func (t *Tgbot) handleBroadcastInput(message *telego.Message, actor chatUser) {
	logger.Debugf("broadcast: chat %d input (message_id=%d group=%q)", actor.chatID, message.MessageID, message.MediaGroupID)
	// The prompt has been answered: it is the admin's scratch space, not the
	// broadcast, so it does not stay behind the preview card.
	t.clearBroadcastPrompt(actor)
	if message.MediaGroupID == "" {
		broadcastDropCompose(actor)
		t.acceptBroadcastDraft(actor, []int{message.MessageID})
		return
	}
	t.bufferBroadcastMedia(actor, message.MediaGroupID, message.MessageID)
}

// acceptBroadcastDraft validates the draft with a self-copy — the admin sees
// exactly what recipients will get — and shows the confirmation preview.
func (t *Tgbot) acceptBroadcastDraft(actor chatUser, ids []int) {
	chatId := actor.chatID
	previewIDs, err := broadcastSender(chatId, broadcastDraft{FromChatID: chatId, MessageIDs: ids})
	if err != nil {
		broadcastDropCompose(actor)
		userStateMgr.clear(actor)
		logger.Warningf("broadcast: chat %d message %v cannot be copied: %v", chatId, ids, err)
		t.clearBroadcastDraft(actor, ids, nil)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastNotCopyable"))
		return
	}
	recipients := t.collectBroadcastRecipients()
	token := t.randomLowerAndNum(12)
	broadcastMu.Lock()
	broadcastComposes[actor] = &broadcastCompose{
		messageIDs:      ids,
		inputMessageIDs: ids,
		token:           token,
	}
	broadcastMu.Unlock()
	userStateMgr.clear(actor)
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.broadcastSend")).WithCallbackData("broadcast_confirm "+token),
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("broadcast_cancel"),
	))
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastPreview", "Count=="+strconv.Itoa(len(recipients))), keyboard)
	// The preview is the copy itself, so it is what the recipients receive. It
	// has served its purpose the moment the admin has looked at it: leaving it
	// in the chat is how the broadcast ended up with three copies of the same
	// message on screen.
	for _, id := range previewIDs {
		t.deleteOwnMessage(chatId, id)
	}
}

// clearBroadcastDraft removes the admin's own composing messages: the input the
// broadcast copies FROM and, when it was already made, the preview copy of it.
// Both are the operator's scratch space and none of it is the broadcast.
func (t *Tgbot) clearBroadcastDraft(actor chatUser, inputIDs, previewIDs []int) {
	for _, id := range inputIDs {
		t.deleteIncomingID(actor.chatID, id)
	}
	for _, id := range previewIDs {
		t.deleteOwnMessage(actor.chatID, id)
	}
}

// recallBroadcast undoes a cancelled broadcast: the copies are removed from
// every recipient that already got one. Best-effort — Telegram refuses a
// deleteMessage older than 48 hours, and a recipient may have read it already.
func (t *Tgbot) recallBroadcast(chatID int64, ids []int) {
	for _, id := range ids {
		t.deleteOwnMessage(chatID, id)
	}
}

// bufferBroadcastMedia appends an album item; the debounce timer fires the
// preview once the group stops growing.
func (t *Tgbot) bufferBroadcastMedia(actor chatUser, groupID string, messageID int) {
	broadcastMu.Lock()
	c := broadcastComposes[actor]
	if c == nil || c.groupID != groupID {
		if c != nil && c.timer != nil {
			c.timer.Stop()
		}
		c = &broadcastCompose{groupID: groupID}
		c.timer = time.AfterFunc(broadcastAlbumDebounce, func() {
			t.finalizeBroadcastAlbum(actor, groupID)
		})
		broadcastComposes[actor] = c
	}
	c.messageIDs = append(c.messageIDs, messageID)
	c.timer.Reset(broadcastAlbumDebounce)
	broadcastMu.Unlock()
}

func (t *Tgbot) finalizeBroadcastAlbum(actor chatUser, groupID string) {
	broadcastMu.Lock()
	c := broadcastComposes[actor]
	if c == nil || c.groupID != groupID {
		broadcastMu.Unlock()
		return
	}
	// Album updates can arrive out of order, and copyMessages requires
	// strictly increasing ids.
	ids := append([]int(nil), c.messageIDs...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	delete(broadcastComposes, actor)
	broadcastMu.Unlock()

	t.acceptBroadcastDraft(actor, ids)
}

// The broadcast flow stays message-scoped rather than screen-scoped: two admins
// in one group compose and confirm their own drafts, and a single per-chat
// screen would make the second preview overwrite the first admin's card (and
// its confirm token). Each card carries its own confirm/cancel, and cancel
// deletes it, so the chat still ends up clean.
func (t *Tgbot) broadcastCancelKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("broadcast_cancel"),
	))
}

// confirmBroadcast turns the pending draft into a run: it claims the single
// runner slot, replaces the preview with a progress card, and starts delivery.
func (t *Tgbot) confirmBroadcast(actor chatUser, token string, messageID int, queryID string) {
	chatId := actor.chatID
	runner := broadcastRegisterRunner(actor)
	if runner == nil {
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.messages.broadcastAlreadyRunning"))
		return
	}
	ids, ok := broadcastTakePending(actor, token)
	if !ok {
		broadcastUnregisterRunner(runner)
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.wentWrong"))
		return
	}
	draft := broadcastDraft{FromChatID: chatId, MessageIDs: ids}
	recipients := t.collectBroadcastRecipients()
	if len(recipients) == 0 {
		broadcastUnregisterRunner(runner)
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.messages.broadcastNoRecipients"))
		t.deleteMessageTgBot(chatId, messageID)
		return
	}
	runner.messageID = messageID
	t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.answers.broadcastStarted"))
	t.editMessageTgBot(chatId, messageID, t.broadcastProgressText(0, len(recipients), 0, 0), t.broadcastCancelKeyboard())
	common.GoRecover("tgbot-broadcast", func() {
		t.runBroadcast(runner, draft, recipients)
	})
}

// runBroadcast walks the recipients sequentially, honoring rate limits, and
// reports the final summary on the runner's card.
func (t *Tgbot) runBroadcast(runner *broadcastRunner, draft broadcastDraft, recipients []int64) {
	defer broadcastUnregisterRunner(runner)
	start := time.Now()
	// One copyMessages call carries a whole album, so the pause scales with
	// the batch size to stay under the same per-second ceiling.
	pause := broadcastSendDelay * time.Duration(max(1, len(draft.MessageIDs)))
	aborted := func() bool { return runner.cancel.Load() || !t.IsRunning() }
	sent, failed, unreachable := 0, 0, 0
	lastProgress := time.Now()
	canceled := false
	// What each recipient actually received, so a cancel can take it back.
	delivered := map[int64][]int{}

	for i, chatID := range recipients {
		if aborted() {
			canceled = runner.cancel.Load()
			break
		}
		ids, err := broadcastDeliverOne(chatID, draft, aborted)
		if errors.Is(err, errBroadcastAborted) {
			canceled = runner.cancel.Load()
			break
		}
		switch {
		case err == nil:
			sent++
			delivered[chatID] = ids
		case broadcastChatUnreachable(err):
			// 403 means the chat never started the bot or blocked it; a long
			// recipient list would turn these into log spam at warning level.
			unreachable++
			logger.Debugf("broadcast: chat %d cannot receive bot messages: %v", chatID, err)
		default:
			failed++
			logger.Warningf("broadcast: chat %d not delivered: %v", chatID, err)
		}
		done := i + 1
		if done%broadcastProgressEvery == 0 || time.Since(lastProgress) >= broadcastProgressInterval {
			t.editMessageTgBot(runner.chatID, runner.messageID, t.broadcastProgressText(done, len(recipients), sent, failed), t.broadcastCancelKeyboard())
			lastProgress = time.Now()
		}
		if i < len(recipients)-1 {
			broadcastPause(pause)
		}
	}

	result := broadcastResult{
		Total:       len(recipients),
		Delivered:   sent,
		Failed:      failed,
		Skipped:     len(recipients) - sent - failed,
		Unreachable: unreachable,
		Canceled:    canceled,
		Elapsed:     time.Since(start).Round(time.Second),
	}
	// A cancelled broadcast that says "this is what went out" and leaves it in
	// every chat is a lie: the copies are withdrawn before that summary is drawn.
	if canceled {
		for chatID, ids := range delivered {
			t.recallBroadcast(chatID, ids)
		}
	}
	summary := t.broadcastSummaryText(result)
	if !t.finalizeBroadcastCard(runner, summary) {
		t.SendMsgToTgbot(runner.chatID, summary)
	}
	// The card is now the only message the run leaves behind: the admin's input
	// and its preview copy are composing artefacts, not the broadcast.
	t.clearBroadcastDraft(runner.actor, draft.MessageIDs, nil)
	logger.Info("broadcast finished: delivered", sent, "failed", failed, "skipped", result.Skipped, "elapsed", result.Elapsed)
	runner.setResult(result)
}

func (t *Tgbot) broadcastProgressText(done, total, sent, failed int) string {
	return t.I18nBot("tgbot.messages.broadcastProgress",
		"Done=="+strconv.Itoa(done),
		"Total=="+strconv.Itoa(total),
		"Sent=="+strconv.Itoa(sent),
		"Failed=="+strconv.Itoa(failed))
}

func (t *Tgbot) broadcastSummaryText(result broadcastResult) string {
	params := []string{
		"Total==" + strconv.Itoa(result.Total),
		"Sent==" + strconv.Itoa(result.Delivered),
		"Failed==" + strconv.Itoa(result.Failed),
		"Skipped==" + strconv.Itoa(result.Skipped),
		"Time==" + result.Elapsed.String(),
	}
	summary := t.I18nBot("tgbot.messages.broadcastFinished", params...)
	if result.Canceled {
		summary = t.I18nBot("tgbot.messages.broadcastCanceled", params...)
	}
	if result.Unreachable > 0 {
		summary += t.I18nBot("tgbot.messages.broadcastUnreachable", "Count=="+strconv.Itoa(result.Unreachable))
	}
	return summary
}

// finalizeBroadcastCard turns the progress card into the summary; false means
// the card is gone and the summary needs its own message to be seen at all.
func (t *Tgbot) finalizeBroadcastCard(runner *broadcastRunner, summary string) bool {
	params := telego.EditMessageTextParams{
		ChatID:    tu.ID(runner.chatID),
		MessageID: runner.messageID,
		Text:      summary,
		ParseMode: "HTML",
		// The run is over: its own controls go, and what remains is the same
		// "🙈 Скрыть" every other finished message offers.
		ReplyMarkup: t.hideButton(),
	}
	_, err := bot.EditMessageText(context.Background(), &params)
	if err == nil || isTelegramNotModifiedError(err) {
		return true
	}
	logger.Warning("broadcast: progress card edit failed:", err)
	return false
}

// cancelBroadcast handles the inline cancel button: while composing it drops
// the draft, while running it stops the loop after the current recipient.
func (t *Tgbot) cancelBroadcast(actor chatUser, messageID int, queryID string) {
	chatId := actor.chatID
	if runner := broadcastCurrentRunner(); runner != nil && runner.chatID == chatId {
		runner.cancel.Store(true)
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.answers.broadcastCanceling"))
		return
	}
	// Dropping a broadcast before it ran: the admin's input, its preview copy,
	// the "send me the message" card and the confirmation card all go, so the
	// chat is as it was before /broadcast.
	inputIDs := broadcastComposeInputs(actor)
	broadcastDropCompose(actor)
	userStateMgr.clear(actor)
	t.clearBroadcastPrompt(actor)
	t.clearBroadcastDraft(actor, inputIDs, nil)
	t.deleteMessageTgBot(chatId, messageID)
	t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.answers.broadcastCanceled"))
}

// collectBroadcastRecipients returns the distinct client Telegram IDs to
// deliver to: clients with a linked tg_id, admins excluded.
func (t *Tgbot) collectBroadcastRecipients() []int64 {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("broadcast: unable to load inbounds:", err)
		return nil
	}
	seen := make(map[int64]bool)
	var recipients []int64
	for _, inbound := range inbounds {
		clients, err := t.inboundService.GetClients(inbound)
		if err != nil {
			continue
		}
		for _, client := range clients {
			if client.TgID == 0 || seen[client.TgID] || checkAdmin(client.TgID) {
				continue
			}
			seen[client.TgID] = true
			recipients = append(recipients, client.TgID)
		}
	}
	return recipients
}

// broadcastDeliverOne retries a recipient through flood-control waits so a 429
// never drops them; after broadcastFloodRetries waits it gives up on them.
func broadcastDeliverOne(chatID int64, draft broadcastDraft, aborted func() bool) ([]int, error) {
	for attempt := 0; ; attempt++ {
		ids, err := broadcastSender(chatID, draft)
		if err == nil {
			return ids, nil
		}
		wait, flood := broadcastRetryAfter(err)
		if !flood || attempt >= broadcastFloodRetries {
			return nil, err
		}
		logger.Warningf("broadcast: chat %d is flood-limited, retrying in %s", chatID, wait)
		if !broadcastFloodWait(wait, aborted) {
			return nil, errBroadcastAborted
		}
	}
}

// broadcastFloodWait sleeps out a flood-control delay in slices so a cancel or
// a bot stop ends the wait instead of parking the runner slot for minutes.
func broadcastFloodWait(wait time.Duration, aborted func() bool) bool {
	for remaining := wait; remaining > 0; remaining -= broadcastFloodWaitSlice {
		if aborted != nil && aborted() {
			return false
		}
		broadcastPause(min(broadcastFloodWaitSlice, remaining))
	}
	return aborted == nil || !aborted()
}

// broadcastChatUnreachable reports a Telegram 403: the chat never started the
// bot or has blocked it, which no retry can fix.
func broadcastChatUnreachable(err error) bool {
	var apiErr *telegoapi.Error
	return errors.As(err, &apiErr) && apiErr.ErrorCode == 403
}

// broadcastRetryAfter reports the flood-control wait a 429 response asks for.
func broadcastRetryAfter(err error) (time.Duration, bool) {
	var apiErr *telegoapi.Error
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != 429 {
		return 0, false
	}
	if apiErr.Parameters == nil || apiErr.Parameters.RetryAfter <= 0 {
		return time.Second, true
	}
	return time.Duration(apiErr.Parameters.RetryAfter) * time.Second, true
}

// deliverBroadcastCopy copies the admin's message into one chat; an album rides
// one copyMessages call and arrives with no forward header. The sender returns
// the ids it created so the caller can take them away again.
func deliverBroadcastCopy(chatID int64, draft broadcastDraft) ([]int, error) {
	from := tu.ID(draft.FromChatID)
	var ids []int
	err := callTelegramAPI(func(ctx context.Context) error {
		var err error
		switch {
		case len(draft.MessageIDs) > 1:
			var copied []telego.MessageID
			copied, err = bot.CopyMessages(ctx, &telego.CopyMessagesParams{ChatID: tu.ID(chatID), FromChatID: from, MessageIDs: draft.MessageIDs})
			for _, m := range copied {
				ids = append(ids, m.MessageID)
			}
		case len(draft.MessageIDs) == 1:
			var copied *telego.MessageID
			copied, err = bot.CopyMessage(ctx, &telego.CopyMessageParams{ChatID: tu.ID(chatID), FromChatID: from, MessageID: draft.MessageIDs[0]})
			if copied != nil {
				ids = append(ids, copied.MessageID)
			}
		}
		return err
	})
	// Every recipient can put the message away: a client has no panel menu to
	// clear it from, and the admin who composed it had no button either. The copy
	// is the broadcast, so its content is untouched — only the keyboard is added.
	if err == nil {
		for _, id := range ids {
			markupBroadcastCopy(chatID, id)
		}
	}
	return ids, err
}

// markupBroadcastCopy puts the hide button on a delivered copy. Best-effort: a
// failure here leaves the broadcast intact and is not worth retrying the send.
func markupBroadcastCopy(chatID int64, messageID int) {
	_ = callTelegramAPI(func(ctx context.Context) error {
		_, err := bot.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
			ChatID:      tu.ID(chatID),
			MessageID:   messageID,
			ReplyMarkup: hideButtonMarkup(),
		})
		return err
	})
}

func callTelegramAPI(call func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return call(ctx)
}
