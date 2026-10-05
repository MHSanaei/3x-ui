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

// botCaptionLimit is Telegram's caption ceiling; a longer caption is a hard API
// error, so a screen pages its body instead of risking a lost render.
const botCaptionLimit = 1024

// screen is the bot's only chat surface: one photo message whose caption and
// inline keyboard are edited on every user action. Nothing else is posted, so a
// chat never grows a second stale menu.
type screen struct {
	msgID    int
	kind     string
	photo    string // file_id to reuse for an edit; empty means upload or text
	hasPhoto bool   // whether the tracked message carries a picture
	header   string
	body     string
	pages    []string
	page     int
	rows     [][]telego.InlineKeyboardButton
	markup   *telego.InlineKeyboardMarkup
	text     string
	pageMax  int
}

// screenStore keeps the live screen per chat. The msgID inside it belongs to
// that chat, so a stale keyboard cannot address another chat's screen.
type screenStore struct {
	mu    sync.Mutex
	chats map[int64]*screen
}

var screens = &screenStore{chats: map[int64]*screen{}}

func (s *screenStore) get(chatID int64) (*screen, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.chats[chatID]
	return sc, ok
}

func (s *screenStore) put(chatID int64, sc *screen) {
	s.mu.Lock()
	s.chats[chatID] = sc
	s.mu.Unlock()
}

func (s *screenStore) clear(chatID int64) {
	s.mu.Lock()
	delete(s.chats, chatID)
	s.mu.Unlock()
}

func (s *screenStore) reset() {
	s.mu.Lock()
	s.chats = map[int64]*screen{}
	s.mu.Unlock()
}

// newScreen assembles a screen: breadcrumb header, body split into
// caption-sized pages, and the caller's keyboard rows (the pager is added at
// render time so it reflects the page the user actually sees).
func (t *Tgbot) newScreen(kind, body string, rows ...[]telego.InlineKeyboardButton) *screen {
	rows = t.ensureBackRow(rows)
	header := ""
	if crumb := t.breadcrumb(kind); crumb != "" {
		header = "<b>" + crumb + "</b>\r\n"
	}
	// Bodies arrive with either line ending; paging splits on CRLF, so a body
	// built with bare LF would silently stay one over-long page.
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	pages := pageMessage(body, botCaptionLimit-len(header))
	if len(pages) == 0 {
		pages = []string{""}
	}
	return &screen{kind: kind, header: header, body: body, pages: pages, rows: rows, pageMax: len(pages)}
}

// renderScreen is the single entry point for drawing a screen: EDIT first, and
// replace (send then delete) only when the edit is genuinely impossible.
func (t *Tgbot) renderScreen(chatID int64, sc *screen) {
	if sc == nil || bot == nil {
		return
	}
	t.attachArt(sc)
	sc.text = sc.header + sc.pages[min(sc.page, len(sc.pages)-1)]
	rows := sc.rows
	if row := t.pagerRow(sc); row != nil {
		rows = append(append([][]telego.InlineKeyboardButton{}, sc.rows...), row)
	}
	sc.markup = tu.InlineKeyboard(rows...)

	if prev, ok := t.screens().get(chatID); ok && prev.msgID != 0 {
		if t.editScreen(chatID, prev, sc) {
			sc.msgID = prev.msgID
			t.screens().put(chatID, sc)
			return
		}
	}
	t.replaceScreen(chatID, sc)
}

// attachArt pins the screen's picture: the bot's avatar when there is one, and
// otherwise the generated black tile, which is always available. A screen is
// therefore always a photo message, and an edit can always go through media.
func (t *Tgbot) attachArt(sc *screen) {
	// A stale cache is refreshed off the render path, so a picture uploaded to
	// @BotFather shows up without a panel restart and no screen waits on it.
	t.ensureScreenArt()
	sc.hasPhoto = true
	if id, ok := artFileID(sc.kind); ok {
		sc.photo = id
		return
	}
	sc.photo = ""
}

// screens keeps one call site for the store, so a test can swap it.
func (t *Tgbot) screens() *screenStore { return screens }

// editScreen edits the tracked message into the new screen. false means the
// caller must replace it: the message is gone, too old, or of another type.
func (t *Tgbot) editScreen(chatID int64, prev *screen, next *screen) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	switch {
	case next.hasPhoto && next.photo != "":
		_, err = bot.EditMessageMedia(ctx, &telego.EditMessageMediaParams{
			ChatID:    tu.ID(chatID),
			MessageID: prev.msgID,
			Media: &telego.InputMediaPhoto{
				Type: "photo", Media: tu.FileFromID(next.photo), Caption: next.text, ParseMode: "HTML",
			},
			ReplyMarkup: next.markup,
		})
	case next.hasPhoto:
		// No file_id yet (the avatar was never read): upload the fallback tile.
		upload, ok := artUpload(next.kind)
		if !ok {
			return false
		}
		_, err = bot.EditMessageMedia(ctx, &telego.EditMessageMediaParams{
			ChatID:    tu.ID(chatID),
			MessageID: prev.msgID,
			Media: &telego.InputMediaPhoto{
				Type: "photo", Media: upload, Caption: next.text, ParseMode: "HTML",
			},
			ReplyMarkup: next.markup,
		})
	case prev.hasPhoto:
		// A photo message cannot become a text one: drop the picture and let
		// its caption carry the screen.
		_, err = bot.EditMessageCaption(ctx, &telego.EditMessageCaptionParams{
			ChatID: tu.ID(chatID), MessageID: prev.msgID,
			Caption: next.text, ParseMode: "HTML", ReplyMarkup: next.markup,
		})
	default:
		_, err = bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      tu.ID(chatID),
			MessageID:   prev.msgID,
			Text:        next.text,
			ParseMode:   "HTML",
			ReplyMarkup: next.markup,
		})
	}
	if err == nil || isTelegramNotModifiedError(err) {
		return true
	}
	logger.Debug("Screen not editable, replacing:", err)
	return false
}

// replaceScreen sends the new screen BEFORE deleting the old one: with a slow
// send, delete-first leaves the chat empty and reads as broken.
func (t *Tgbot) replaceScreen(chatID int64, sc *screen) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prev, hadPrev := t.screens().get(chatID)
	var sent *telego.Message
	var err error
	switch {
	case sc.hasPhoto && sc.photo != "":
		sent, err = bot.SendPhoto(ctx, &telego.SendPhotoParams{
			ChatID: tu.ID(chatID), Photo: tu.FileFromID(sc.photo),
			Caption: sc.text, ParseMode: "HTML", ReplyMarkup: sc.markup,
		})
	case sc.hasPhoto:
		upload, ok := artUpload(sc.kind)
		if !ok {
			return
		}
		sent, err = bot.SendPhoto(ctx, &telego.SendPhotoParams{
			ChatID: tu.ID(chatID), Photo: upload,
			Caption: sc.text, ParseMode: "HTML", ReplyMarkup: sc.markup,
		})
	default:
		sent, err = bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: tu.ID(chatID), Text: sc.text, ParseMode: "HTML", ReplyMarkup: sc.markup,
		})
	}
	if err != nil {
		logger.Warning("Failed to send a screen:", err)
		return
	}
	if hadPrev && prev.msgID != 0 {
		t.deleteOwnMessage(chatID, prev.msgID)
	}
	sc.msgID = sent.MessageID
	// Telegram hands back the file_id of the uploaded tile: remembering it keeps
	// every later edit a cheap media edit instead of another upload.
	if sc.hasPhoto && sc.photo == "" && len(sent.Photo) > 0 {
		sc.photo = sent.Photo[len(sent.Photo)-1].FileID
		cacheArtFileID(sc.kind, sc.photo)
	}
	t.screens().put(chatID, sc)
}

// ensureBackRow guarantees every screen can be left. A screen that only offers
// its own actions is a trap — the reset confirmation spent its Cancel on the
// reset and left a screen whose single button was the thing already used. The
// check is by callback, not by label, so a screen is free to name the way back
// whatever fits ("Cancel", "Back", the category's name).
func (t *Tgbot) ensureBackRow(rows [][]telego.InlineKeyboardButton) [][]telego.InlineKeyboardButton {
	for _, row := range rows {
		for _, btn := range row {
			if btn.CallbackData == cbHome {
				return rows
			}
		}
	}
	return append(append([][]telego.InlineKeyboardButton{}, rows...), t.backRow())
}

// breadcrumb names the screen's place in the tree, so a user who was away can
// see where a stale keyboard left them.
func (t *Tgbot) breadcrumb(kind string) string {
	key := "tgbot.screens." + kind
	crumb := t.I18nBot(key)
	if crumb == "" || crumb == key {
		return ""
	}
	return t.I18nBot("tgbot.screens.panel") + " › " + crumb
}

// pagerRow adds the page controls when the body did not fit one caption. The
// label is localized; the arrows are callback data, never a bare icon-only tap
// target for a destructive action.
func (t *Tgbot) pagerRow(sc *screen) []telego.InlineKeyboardButton {
	if sc.pageMax <= 1 {
		return nil
	}
	prev := tu.InlineKeyboardButton("‹").WithCallbackData("pg:prev")
	if sc.page <= 0 {
		prev = tu.InlineKeyboardButton("·").WithCallbackData("pg:none")
	}
	next := tu.InlineKeyboardButton("›").WithCallbackData("pg:next")
	if sc.page >= sc.pageMax-1 {
		next = tu.InlineKeyboardButton("·").WithCallbackData("pg:none")
	}
	return tu.InlineKeyboardRow(prev, tu.InlineKeyboardButton(t.pageLabel(sc.page, sc.pageMax)).WithCallbackData("pg:none"), next)
}

// pageLabel renders "Стр. 2 из 5" through the locale, never a raw "2/5".
func (t *Tgbot) pageLabel(page, pages int) string {
	return t.I18nBot("tgbot.buttons.page", "Page=="+itoa(page+1), "Pages=="+itoa(pages))
}

// showPage re-renders the stored screen at another page of its body.
func (t *Tgbot) showPage(chatID int64, page int) {
	sc, ok := t.screens().get(chatID)
	if !ok || sc.pageMax <= 1 || page < 0 || page >= sc.pageMax {
		return
	}
	sc.page = page
	sc.pages = pageMessage(sc.body, botCaptionLimit-len(sc.header))
	t.renderScreen(chatID, sc)
}

// deleteScreenMessage is best effort: the user may have removed the screen, or
// Telegram may refuse past the 48-hour window. Neither is worth operator noise.
func (t *Tgbot) deleteScreenMessage(chatID int64, msgID int) { t.deleteOwnMessage(chatID, msgID) }

// deleteIncoming removes a message the user sent: the wizard's text replies and
// the pilot taps must not pile up in the chat.
func (t *Tgbot) deleteIncoming(message *telego.Message) {
	if message == nil || bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := bot.DeleteMessage(ctx, &telego.DeleteMessageParams{ChatID: tu.ID(message.Chat.ID), MessageID: message.MessageID}); err != nil {
		logger.Debug("Cannot delete a user message:", err)
	}
}

// deleteOwnMessage removes one of our messages without ever surfacing a
// failure: a user may have deleted it, and Telegram refuses past 48 hours.
func (t *Tgbot) deleteOwnMessage(chatID int64, msgID int) {
	if msgID == 0 || bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := bot.DeleteMessage(ctx, &telego.DeleteMessageParams{ChatID: tu.ID(chatID), MessageID: msgID}); err != nil {
		logger.Debug("Cannot delete a bot message:", err)
	}
}

// hideButton is the keyboard of a non-screen message: one tap clears it.
func (t *Tgbot) hideButton() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(t.hideRow())
}

// answerSilent acknowledges a callback without a toast, for taps whose only
// effect is visible in the message itself.
func (t *Tgbot) answerSilent(callbackID string) {
	if bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: callbackID}); err != nil {
		logger.Debug("Answer callback query failed:", err)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// I18nBotStatic is gone; labels are built on the Tgbot receiver.
