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

	// Pause per recipient, scaled by the number of copied messages, keeps the
	// run around the bot-wide ~30 msg/s ceiling even for whole albums.
	broadcastSendDelay = 60 * time.Millisecond
	// Progress refreshes are throttled to keep the run under rate limits.
	broadcastProgressEvery    = 25
	broadcastProgressInterval = 3 * time.Second
	broadcastFloodRetries     = 5
)

// broadcastDraft references the admin's original message instead of parsing
// its content: copyMessage delivers any message type 1:1 on behalf of the
// bot, formatting and media included. An album is the list of its messages.
type broadcastDraft struct {
	FromChatID int64
	MessageIDs []int
}

// broadcastResult is the end-of-run statistics shown to the admin.
type broadcastResult struct {
	Total     int
	Delivered int
	Failed    int
	Skipped   int
	Canceled  bool
	Elapsed   time.Duration
}

// broadcastRunner tracks the single in-flight broadcast: where to report
// progress and whether the admin asked to stop it.
type broadcastRunner struct {
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

// broadcastCompose is one admin chat's composition: the collected message
// ids, the media group still arriving (if any), and the card token tying the
// pending preview to its own Send button.
type broadcastCompose struct {
	messageIDs []int
	groupID    string
	token      string
	timer      *time.Timer
}

var (
	broadcastMu       sync.Mutex
	broadcastComposes = make(map[int64]*broadcastCompose)
	broadcastActive   *broadcastRunner
)

// broadcastAlbumDebounce waits out Telegram's stream of one media group: an
// album reaches the bot as separate messages sharing a media_group_id.
var broadcastAlbumDebounce = 900 * time.Millisecond

func broadcastDropCompose(chatID int64) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if c := broadcastComposes[chatID]; c != nil && c.timer != nil {
		c.timer.Stop()
	}
	delete(broadcastComposes, chatID)
}

// broadcastPendingDraft reports the ids awaiting confirmation for a chat;
// ok is false while an album is still being collected.
func broadcastPendingDraft(chatID int64) ([]int, string, bool) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	c := broadcastComposes[chatID]
	if c == nil || c.groupID != "" || c.token == "" {
		return nil, "", false
	}
	return append([]int(nil), c.messageIDs...), c.token, true
}

// broadcastTakePending removes the pending draft only when its card token
// matches; ok is false for stale taps or chats with no pending draft.
func broadcastTakePending(chatID int64, token string) ([]int, bool) {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	c := broadcastComposes[chatID]
	if c == nil || c.token == "" || c.token != token {
		return nil, false
	}
	delete(broadcastComposes, chatID)
	return c.messageIDs, true
}

// broadcastRegisterRunner claims the single broadcast slot; nil means one is
// already running.
func broadcastRegisterRunner(chatID int64) *broadcastRunner {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if broadcastActive != nil {
		return nil
	}
	broadcastActive = &broadcastRunner{chatID: chatID}
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

// broadcastSender delivers one draft to one chat; swapped out in tests.
var broadcastSender = deliverBroadcastCopy

// broadcastPause stands in for time.Sleep so tests don't wait real seconds.
var broadcastPause = time.Sleep

// startBroadcast answers /broadcast: one broadcast at a time, so a second
// command while one is running is refused instead of queued.
func (t *Tgbot) startBroadcast(chatId int64) {
	if broadcastCurrentRunner() != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastAlreadyRunning"))
		return
	}
	broadcastDropCompose(chatId)
	userStateMgr.set(chatId, broadcastAwaitingText)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastAskText"), t.broadcastCancelKeyboard())
}

// handleBroadcastInput references the message the admin sent for the
// broadcast and shows the confirmation preview. Only the chat's admin may
// fill it: the state lives under a chat id, which in a group is shared.
func (t *Tgbot) handleBroadcastInput(message *telego.Message) {
	chatId := message.Chat.ID
	if message.From == nil || !checkAdmin(message.From.ID) {
		return
	}
	logger.Debugf("broadcast: chat %d input (message_id=%d group=%q)", chatId, message.MessageID, message.MediaGroupID)
	if message.MediaGroupID == "" {
		broadcastDropCompose(chatId)
		t.acceptBroadcastDraft(chatId, []int{message.MessageID})
		return
	}
	t.bufferBroadcastMedia(chatId, message.MediaGroupID, message.MessageID)
}

// acceptBroadcastDraft validates the draft with a self-copy — the admin sees
// exactly what recipients will get — and shows the confirmation preview.
func (t *Tgbot) acceptBroadcastDraft(chatId int64, ids []int) {
	if err := broadcastSender(chatId, broadcastDraft{FromChatID: chatId, MessageIDs: ids}); err != nil {
		broadcastDropCompose(chatId)
		userStateMgr.clear(chatId)
		logger.Warningf("broadcast: chat %d message %v cannot be copied: %v", chatId, ids, err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastNotCopyable"))
		return
	}
	recipients := t.collectBroadcastRecipients()
	token := t.randomLowerAndNum(12)
	broadcastMu.Lock()
	broadcastComposes[chatId] = &broadcastCompose{messageIDs: ids, token: token}
	broadcastMu.Unlock()
	userStateMgr.clear(chatId)
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.broadcastSend")).WithCallbackData("broadcast_confirm "+token),
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("broadcast_cancel"),
	))
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastPreview", "Count=="+strconv.Itoa(len(recipients))), keyboard)
}

// bufferBroadcastMedia appends an album item; the debounce timer fires the
// preview once the group stops growing.
func (t *Tgbot) bufferBroadcastMedia(chatId int64, groupID string, messageID int) {
	broadcastMu.Lock()
	c := broadcastComposes[chatId]
	if c == nil || c.groupID != groupID {
		if c != nil && c.timer != nil {
			c.timer.Stop()
		}
		c = &broadcastCompose{groupID: groupID}
		c.timer = time.AfterFunc(broadcastAlbumDebounce, func() {
			t.finalizeBroadcastAlbum(chatId, groupID)
		})
		broadcastComposes[chatId] = c
	}
	c.messageIDs = append(c.messageIDs, messageID)
	c.timer.Reset(broadcastAlbumDebounce)
	broadcastMu.Unlock()
}

func (t *Tgbot) finalizeBroadcastAlbum(chatId int64, groupID string) {
	broadcastMu.Lock()
	c := broadcastComposes[chatId]
	if c == nil || c.groupID != groupID {
		broadcastMu.Unlock()
		return
	}
	// Album updates can arrive out of order, and copyMessages requires
	// strictly increasing ids.
	ids := append([]int(nil), c.messageIDs...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	delete(broadcastComposes, chatId)
	broadcastMu.Unlock()

	t.acceptBroadcastDraft(chatId, ids)
}

func (t *Tgbot) broadcastCancelKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("broadcast_cancel"),
	))
}

// confirmBroadcast turns the pending draft into a running broadcast: it claims
// the single runner slot, replaces the preview with a progress card and hands
// the delivery loop to a background goroutine.
func (t *Tgbot) confirmBroadcast(chatId int64, token string, messageID int, queryID string) {
	runner := broadcastRegisterRunner(chatId)
	if runner == nil {
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.messages.broadcastAlreadyRunning"))
		return
	}
	ids, ok := broadcastTakePending(chatId, token)
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
// reports progress on the runner's message until done, canceled or the bot
// stops. The result is published last so observers of the released slot see
// the final stats.
func (t *Tgbot) runBroadcast(runner *broadcastRunner, draft broadcastDraft, recipients []int64) {
	defer broadcastUnregisterRunner(runner)
	start := time.Now()
	// One copyMessages call carries a whole album, so the pause scales with
	// the batch size to stay under the same per-second ceiling.
	pause := broadcastSendDelay * time.Duration(max(1, len(draft.MessageIDs)))
	sent, failed := 0, 0
	lastProgress := time.Now()
	canceled := false

	for i, chatID := range recipients {
		if runner.cancel.Load() {
			canceled = true
			break
		}
		if !t.IsRunning() {
			break
		}
		if err := broadcastDeliverOne(chatID, draft, runner.cancel.Load); err != nil {
			failed++
			logger.Warningf("broadcast: chat %d not delivered: %v", chatID, err)
		} else {
			sent++
		}
		done := sent + failed
		if done%broadcastProgressEvery == 0 || time.Since(lastProgress) >= broadcastProgressInterval {
			t.editMessageTgBot(runner.chatID, runner.messageID, t.broadcastProgressText(done, len(recipients), sent, failed), t.broadcastCancelKeyboard())
			lastProgress = time.Now()
		}
		if i < len(recipients)-1 {
			broadcastPause(pause)
		}
	}

	result := broadcastResult{
		Total:     len(recipients),
		Delivered: sent,
		Failed:    failed,
		Skipped:   len(recipients) - sent - failed,
		Canceled:  canceled,
		Elapsed:   time.Since(start).Round(time.Second),
	}
	summary := t.broadcastSummaryText(result)
	if !t.finalizeBroadcastCard(runner, summary) {
		t.SendMsgToTgbot(runner.chatID, summary)
	}
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
	if result.Canceled {
		return t.I18nBot("tgbot.messages.broadcastCanceled", params...)
	}
	return t.I18nBot("tgbot.messages.broadcastFinished", params...)
}

// finalizeBroadcastCard turns the progress card into the summary and drops its
// cancel button; false means the card is gone and the summary needs its own
// message to be seen at all.
func (t *Tgbot) finalizeBroadcastCard(runner *broadcastRunner, summary string) bool {
	params := telego.EditMessageTextParams{
		ChatID:      tu.ID(runner.chatID),
		MessageID:   runner.messageID,
		Text:        summary,
		ParseMode:   "HTML",
		ReplyMarkup: &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{}},
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
func (t *Tgbot) cancelBroadcast(chatId int64, messageID int, queryID string) {
	if runner := broadcastCurrentRunner(); runner != nil && runner.chatID == chatId {
		runner.cancel.Store(true)
		t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.answers.broadcastCanceling"))
		return
	}
	broadcastDropCompose(chatId)
	userStateMgr.clear(chatId)
	t.deleteMessageTgBot(chatId, messageID)
	t.sendCallbackAnswerTgBot(queryID, t.I18nBot("tgbot.answers.broadcastCanceled"))
}

// collectBroadcastRecipients returns the distinct client Telegram IDs to
// deliver to: clients with a linked tg_id, admins excluded — they already
// receive the reports.
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
// A cancel during a wait abandons the recipient immediately.
func broadcastDeliverOne(chatID int64, draft broadcastDraft, canceled func() bool) error {
	for attempt := 0; ; attempt++ {
		err := broadcastSender(chatID, draft)
		if err == nil {
			return nil
		}
		wait, flood := broadcastRetryAfter(err)
		if !flood || attempt >= broadcastFloodRetries {
			return err
		}
		if canceled != nil && canceled() {
			return err
		}
		logger.Warningf("broadcast: chat %d is flood-limited, retrying in %s", chatID, wait)
		broadcastPause(wait)
	}
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

// deliverBroadcastCopy copies the admin's original message to one recipient
// chat; an album rides one copyMessages call. Content arrives 1:1 on behalf
// of the bot, with no forward header.
func deliverBroadcastCopy(chatID int64, draft broadcastDraft) error {
	from := tu.ID(draft.FromChatID)
	return callTelegramAPI(func(ctx context.Context) error {
		var err error
		switch {
		case len(draft.MessageIDs) > 1:
			_, err = bot.CopyMessages(ctx, &telego.CopyMessagesParams{ChatID: tu.ID(chatID), FromChatID: from, MessageIDs: draft.MessageIDs})
		case len(draft.MessageIDs) == 1:
			_, err = bot.CopyMessage(ctx, &telego.CopyMessageParams{ChatID: tu.ID(chatID), FromChatID: from, MessageID: draft.MessageIDs[0]})
		}
		return err
	})
}

func callTelegramAPI(call func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return call(ctx)
}
