package tgbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const telegramPageLimit = 2000

func pageMessage(message string, limit int) []string {
	if len(message) <= limit {
		return []string{message}
	}

	pages := make([]string, 0)
	for block := range strings.SplitSeq(message, "\r\n\r\n") {
		for _, page := range splitMessageLines(block, limit) {
			last := len(pages) - 1
			if last >= 0 && len(pages[last])+len("\r\n\r\n")+len(page) <= limit {
				pages[last] += "\r\n\r\n" + page
				continue
			}
			pages = append(pages, page)
		}
	}
	if len(pages) > 0 && strings.TrimSpace(pages[len(pages)-1]) == "" {
		pages = pages[:len(pages)-1]
	}
	return pages
}

func splitMessageLines(block string, limit int) []string {
	if len(block) <= limit {
		return []string{block}
	}

	lines := strings.Split(block, "\r\n")
	pages := []string{lines[0]}
	for _, line := range lines[1:] {
		last := len(pages) - 1
		if len(pages[last])+len("\r\n")+len(line) > limit {
			pages = append(pages, line)
			continue
		}
		pages[last] += "\r\n" + line
	}
	return pages
}

// SendMsgToTgbot sends a message to the Telegram bot with optional reply markup.
// SendMsgToTgbot sends a message and returns the id of the last one it managed to
// deliver, or 0. A caller that may need to take its own message back (the
// broadcast prompt) reads it; every other caller ignores it.
func (t *Tgbot) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) int {
	if !t.IsRunning() {
		return 0
	}

	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return 0
	}
	lastID := 0

	allMessages := pageMessage(msg, telegramPageLimit)
	for n, message := range allMessages {
		params := telego.SendMessageParams{
			ChatID:    tu.ID(chatId),
			Text:      message,
			ParseMode: "HTML",
		}
		// only add replyMarkup to last message
		if len(replyMarkup) > 0 && n == (len(allMessages)-1) {
			params.ReplyMarkup = replyMarkup[0]
		}

		// Retry logic with exponential backoff for connection errors
		maxRetries := 3
		for attempt := range maxRetries {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			sent, err := bot.SendMessage(ctx, &params)
			cancel()

			if err == nil {
				if sent != nil {
					lastID = sent.MessageID
				}
				break // Success
			}

			// Check if error is a connection error
			errStr := err.Error()
			isConnectionError := strings.Contains(errStr, "connection") ||
				strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "closed")

			if isConnectionError && attempt < maxRetries-1 {
				// Exponential backoff: 1s, 2s, 4s
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				logger.Warningf("Connection error sending telegram message (attempt %d/%d), retrying in %v: %v",
					attempt+1, maxRetries, backoff, err)
				time.Sleep(backoff)
			} else {
				logger.Warning("Error sending telegram message:", err)
				break
			}
		}

		// Reduced delay to improve performance (only needed for rate limiting)
		if n < len(allMessages)-1 { // Only delay between messages, not after the last one
			time.Sleep(100 * time.Millisecond)
		}
	}
	return lastID
}

// SendMsgToTgbotAdmins sends a message to all admin Telegram chats.
func (t *Tgbot) SendMsgToTgbotAdmins(msg string, replyMarkup ...telego.ReplyMarkup) {
	admins := adminSnapshot()
	if len(replyMarkup) > 0 {
		for _, adminId := range admins {
			t.SendMsgToTgbot(adminId, msg, replyMarkup[0])
		}
	} else {
		for _, adminId := range admins {
			t.SendMsgToTgbot(adminId, msg)
		}
	}
}

// sendCallbackAnswerTgBot answers a callback query with a message.
func (t *Tgbot) sendCallbackAnswerTgBot(id string, message string) {
	params := telego.AnswerCallbackQueryParams{
		CallbackQueryID: id,
		Text:            message,
	}
	if err := bot.AnswerCallbackQuery(context.Background(), &params); err != nil {
		logger.Warning(err)
	}
}

// editMessageCallbackTgBot edits the reply markup of a message.
func (t *Tgbot) editMessageCallbackTgBot(chatId int64, messageID int, inlineKeyboard *telego.InlineKeyboardMarkup) {
	params := telego.EditMessageReplyMarkupParams{
		ChatID:      tu.ID(chatId),
		MessageID:   messageID,
		ReplyMarkup: inlineKeyboard,
	}
	if _, err := bot.EditMessageReplyMarkup(context.Background(), &params); err != nil {
		if isTelegramNotModifiedError(err) {
			logger.Debug("Telegram reply markup unchanged, skipping edit")
			return
		}
		logger.Warning(err)
	}
}

// editMessageTgBot edits the text and reply markup of a message.
func (t *Tgbot) editMessageTgBot(chatId int64, messageID int, text string, inlineKeyboard ...*telego.InlineKeyboardMarkup) {
	params := telego.EditMessageTextParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
	}
	if len(inlineKeyboard) > 0 {
		params.ReplyMarkup = inlineKeyboard[0]
	}
	if _, err := bot.EditMessageText(context.Background(), &params); err != nil {
		if isTelegramNotModifiedError(err) {
			logger.Debug("Telegram message text unchanged, skipping edit")
			return
		}
		logger.Warning(err)
	}
}

// Telegram answers a no-op edit with a 400 whose description carries this text;
// a refresh tap that changed nothing is not an operator-visible failure.
func isTelegramNotModifiedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not modified") ||
		strings.Contains(errStr, "No fields to modify")
}

// deleteMessageTgBot deletes a message from the chat.
func (t *Tgbot) deleteMessageTgBot(chatId int64, messageID int) {
	if bot == nil {
		return
	}
	params := telego.DeleteMessageParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
	}
	if err := bot.DeleteMessage(context.Background(), &params); err != nil {
		logger.Warning("Failed to delete message:", err)
	} else {
		logger.Info("Message deleted successfully")
	}
}

// TestConnection verifies the bot token is valid and the API is reachable.
func (t *Tgbot) TestConnection() error {
	tgBotMutex.Lock()
	b := bot
	tgBotMutex.Unlock()
	if b == nil {
		return fmt.Errorf("bot not initialized")
	}
	me, err := b.GetMe(context.Background())
	if err != nil {
		return fmt.Errorf("API unreachable: %w", err)
	}
	_ = me
	return nil
}
