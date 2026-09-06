package tgbot

import (
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	tu "github.com/mymmrac/telego/telegoutil"
)

const stateBroadcast = "awaiting_broadcast"

// Telegram throttles bulk sending at roughly 30 messages per second; pacing
// the fan-out keeps a large customer base from tripping 429 rate limits.
const broadcastSendGap = 50 * time.Millisecond

var pendingBroadcasts = struct {
	sync.Mutex
	byAdmin map[int64]string
}{byAdmin: map[int64]string{}}

func setPendingBroadcast(adminChatId int64, text string) {
	pendingBroadcasts.Lock()
	defer pendingBroadcasts.Unlock()
	pendingBroadcasts.byAdmin[adminChatId] = text
}

func takePendingBroadcast(adminChatId int64) (string, bool) {
	pendingBroadcasts.Lock()
	defer pendingBroadcasts.Unlock()
	text, ok := pendingBroadcasts.byAdmin[adminChatId]
	delete(pendingBroadcasts.byAdmin, adminChatId)
	return text, ok
}

// Admins are excluded from their own broadcast, matching how expiry
// notifications already skip admin chats.
func (t *Tgbot) broadcastRecipients() []int64 {
	ids, err := t.clientService.DistinctTelegramUserIDs()
	if err != nil {
		logger.Warning("tgbot: broadcast recipients lookup failed:", err)
		return nil
	}
	recipients := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !checkAdmin(id) {
			recipients = append(recipients, id)
		}
	}
	return recipients
}

func (t *Tgbot) startBroadcast(chatId int64) {
	userStateMgr.set(chatId, stateBroadcast)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastPrompt"))
}

func (t *Tgbot) previewBroadcast(chatId int64, text string) {
	if text == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastEmpty"))
		return
	}
	recipients := t.broadcastRecipients()
	if len(recipients) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastNoRecipients"))
		return
	}
	setPendingBroadcast(chatId, text)

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("broadcast_cancel")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmBroadcast", "Count=="+strconv.Itoa(len(recipients)))).
				WithCallbackData(t.encodeQuery("broadcast_send")),
		),
	)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastPreview"), keyboard)
	if err := t.sendDirect(chatId, text); err != nil {
		logger.Warning("tgbot: broadcast preview failed:", err)
	}
}

func (t *Tgbot) cancelBroadcast(chatId int64) {
	takePendingBroadcast(chatId)
	userStateMgr.clear(chatId)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastCancelled"))
}

// One unreachable recipient (a customer who blocked the bot) must not abort
// the run, so failures are counted and reported rather than returned.
func (t *Tgbot) runBroadcast(chatId int64) {
	text, ok := takePendingBroadcast(chatId)
	if !ok || text == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastEmpty"))
		return
	}
	recipients := t.broadcastRecipients()
	sent, failed := 0, 0
	for i, id := range recipients {
		if err := t.sendDirect(id, text); err != nil {
			logger.Warningf("tgbot: broadcast to %d failed: %v", id, err)
			failed++
		} else {
			sent++
		}
		if i < len(recipients)-1 {
			time.Sleep(broadcastSendGap)
		}
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.broadcastDone",
		"Sent=="+strconv.Itoa(sent), "Failed=="+strconv.Itoa(failed)))
}
