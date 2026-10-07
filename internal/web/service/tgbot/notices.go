package tgbot

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Panel notifications are the bot's other voice: an outage, a failed login, a
// backup. Each one used to be its own message, so a single flapping outbound
// buried the chat. They are now ONE live message per (chat, kind) that repeats
// edit in place, plus the same hide button the rest of the bot uses.

type noticeKey struct {
	chatID int64
	kind   string
}

type noticeState struct {
	msgID int
	body  string
	count int
	at    time.Time
}

type noticeStore struct {
	mu      sync.Mutex
	notices map[noticeKey]noticeState
}

var notices = &noticeStore{notices: map[noticeKey]noticeState{}}

func (s *noticeStore) get(key noticeKey) (noticeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.notices[key]
	return st, ok
}

func (s *noticeStore) put(key noticeKey, st noticeState) {
	s.mu.Lock()
	s.notices[key] = st
	s.mu.Unlock()
}

func (s *noticeStore) reset() {
	s.mu.Lock()
	s.notices = map[noticeKey]noticeState{}
	s.mu.Unlock()
}

// noticeRepeatWindow is how long a repeated event of the same kind keeps
// updating the live card instead of starting a new one.
const noticeRepeatWindow = 30 * time.Minute

// liveNotice sends or updates the chat's card for one kind of event. A repeat
// inside the window edits the card and counts, so a storm of the same event
// reads as one line with a number instead of a wall of identical messages.
func (t *Tgbot) liveNotice(chatID int64, kind, body string) {
	if bot == nil {
		return
	}
	key := noticeKey{chatID: chatID, kind: kind}
	st, ok := notices.get(key)

	if ok && time.Since(st.at) < noticeRepeatWindow && st.msgID != 0 {
		st.count++
		st.at = time.Now()
		st.body = body
		text := t.noticeText(body, st.count)
		if t.editNotice(chatID, st.msgID, text) {
			notices.put(key, st)
			return
		}
		// The card is gone or too old to edit: fall through and post a fresh one.
	}

	st = noticeState{body: body, count: 1, at: time.Now()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sc, err := bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: tu.ID(chatID), Text: t.noticeText(body, 1), ParseMode: "HTML", ReplyMarkup: t.hideButton(),
	})
	if err != nil {
		logger.Warning("Failed to send a notice:", err)
		return
	}
	st.msgID = sc.MessageID
	notices.put(key, st)
}

// noticeText renders the event plus its repeat counter.
func (t *Tgbot) noticeText(body string, count int) string {
	text := body
	if count > 1 {
		text += t.I18nBot("tgbot.notices.repeat", "Count=="+itoa(count), "Time=="+time.Now().Format("15:04:05"))
	}
	return text
}

// editNotice rewrites a live card. false means it could not be edited and the
// caller must post a new one.
func (t *Tgbot) editNotice(chatID int64, msgID int, text string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := bot.EditMessageText(ctx, &telego.EditMessageTextParams{
		ChatID: tu.ID(chatID), MessageID: msgID, Text: text, ParseMode: "HTML",
		ReplyMarkup: t.hideButton(),
	})
	if err == nil || isTelegramNotModifiedError(err) {
		return true
	}
	logger.Debug("Notice not editable, posting a new one:", err)
	return false
}

// dropNoticeFor forgets the card a hidden message belonged to, so the next
// event of that kind starts a fresh card.
func (t *Tgbot) dropNoticeFor(chatID int64, msgID int) {
	notices.mu.Lock()
	defer notices.mu.Unlock()
	for key, st := range notices.notices {
		if key.chatID == chatID && st.msgID == msgID {
			delete(notices.notices, key)
		}
	}
}

// sendDocumentWithCaption uploads a file together with its summary: a document
// cannot hold a screen, so the caption is what carries the information, and one
// message does the work of two.
func (t *Tgbot) sendDocumentWithCaption(chatID int64, data []byte, name, caption string) bool {
	if bot == nil || data == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := bot.SendDocument(ctx, &telego.SendDocumentParams{
		ChatID:      tu.ID(chatID),
		Document:    tu.FileFromBytes(data, name),
		Caption:     caption,
		ParseMode:   "HTML",
		ReplyMarkup: t.hideButton(),
	})
	if err != nil {
		logger.Warning("Failed to upload a file with a caption:", err)
		return false
	}
	return true
}

// trimCaption cuts a summary down to what a caption can carry, on a line break
// so a sentence is never left half-written.
func trimCaption(text string, limit int) string {
	if limit <= 0 || len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	cut := string(runes[:limit])
	if idx := strings.LastIndex(cut, "\n"); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimRight(cut, "\r\n")
}
