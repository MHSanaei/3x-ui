package tgbot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// sendResponse sends the response message based on the onlyMessage flag.
func (t *Tgbot) sendResponse(chatId int64, msg string, onlyMessage bool, level userLevel) {
	if onlyMessage {
		t.SendMsgToTgbot(chatId, msg)
	} else {
		t.SendAnswer(chatId, msg, level)
	}
}

// The console is a hub of five sections rather than one wall of buttons: the
// top level says what the bot can do, and each section holds the detail.
func (t *Tgbot) adminKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.sectionClients")).WithCallbackData(t.encodeQuery("admin_clients")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.sectionReports")).WithCallbackData(t.encodeQuery("admin_reports")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.serverMenu")).WithCallbackData(t.encodeQuery("server")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.sectionMessaging")).WithCallbackData(t.encodeQuery("admin_messaging")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.sectionSettings")).WithCallbackData(t.encodeQuery("admin_settings")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToUserPanel")).WithCallbackData(t.encodeQuery("user_panel")),
		),
	)
}

// Who the customers are. Every fleet-wide change lives behind Bulk actions, so
// nothing here can touch more than the one client the admin picked.
func (t *Tgbot) adminClientsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientRoster")).WithCallbackData(t.encodeQuery("client_roster")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.addClient")).WithCallbackData(t.encodeQuery("add_client")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.allClients")).WithCallbackData(t.encodeQuery("get_inbounds")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.onlines")).WithCallbackData(t.encodeQuery("onlines")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.inviteLinks")).WithCallbackData(t.encodeQuery("invite_links")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.bulkActions")).WithCallbackData(t.encodeQuery("bulk_menu")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		),
	)
}

// Read-only lists only. Anything that changes state belongs in Clients or
// Server, and the bot's own switches belong in Bot Settings.
func (t *Tgbot) adminReportsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.SortedTrafficUsageReport")).WithCallbackData(t.encodeQuery("get_sorted_traffic_usage_report")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getInbounds")).WithCallbackData(t.encodeQuery("inbounds")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.depleteSoon")).WithCallbackData(t.encodeQuery("deplete_soon")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getBanLogs")).WithCallbackData(t.encodeQuery("get_banlogs")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		),
	)
}

func (t *Tgbot) adminMessagingKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.broadcast")).WithCallbackData(t.encodeQuery("broadcast")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.remindNow")).WithCallbackData(t.encodeQuery("remind_now")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.setHelpText")).WithCallbackData(t.encodeQuery("set_help")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		),
	)
}

// How the bot itself behaves, kept apart from the reports it produces.
func (t *Tgbot) adminSettingsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.notifications")).WithCallbackData(t.encodeQuery("notify_settings")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.botFeatures")).WithCallbackData(t.encodeQuery("admin_features")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.dailyHour")).WithCallbackData(t.encodeQuery("settings_hour")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.bindLimit")).WithCallbackData(t.encodeQuery("settings_bindings")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("commands")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		),
	)
}

// clientKeyboard is what a customer sees, and what an admin sees first. Every way of
// getting or rotating a config lives behind My Configs, so the grid never changes.
func (t *Tgbot) clientKeyboard(level userLevel) *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.myConfigs")).WithCallbackData(t.encodeQuery("client_configs")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientUsage")).WithCallbackData(t.encodeQuery("client_traffic")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.help")).WithCallbackData(t.encodeQuery("client_help")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.messageAdmin")).WithCallbackData(t.encodeQuery("client_pm")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.settings")).WithCallbackData(t.encodeQuery("client_settings")),
		),
	}
	// The Admin row is appended only for admins, so nothing hints at a second
	// panel to everyone else.
	if level == levelAdmin {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.adminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		))
	}
	return tu.InlineKeyboardGrid(rows)
}

// SendAnswer sends a response message with the keyboard the caller's level earns.
func (t *Tgbot) SendAnswer(chatId int64, msg string, level userLevel) {
	if level == levelStranger {
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	t.SendMsgToTgbot(chatId, msg, t.clientKeyboard(level))
}

const telegramPageLimit = 2000

func pageMessage(message string, limit int) []string {
	if len(message) <= limit {
		return []string{message}
	}

	pages := make([]string, 0)
	for _, block := range strings.Split(message, "\r\n\r\n") {
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
func (t *Tgbot) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) {
	if !isRunning {
		return
	}

	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return
	}

	allMessages := pageMessage(msg, telegramPageLimit)
	for n, message := range allMessages {
		params := telego.SendMessageParams{
			ChatID:              tu.ID(chatId),
			Text:                message,
			ParseMode:           "HTML",
			DisableNotification: t.silent,
		}
		// only add replyMarkup to last message
		if len(replyMarkup) > 0 && n == (len(allMessages)-1) {
			params.ReplyMarkup = replyMarkup[0]
		}

		// Retry logic with exponential backoff for connection errors
		maxRetries := 3
		for attempt := range maxRetries {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := bot.SendMessage(ctx, &params)
			cancel()

			if err == nil {
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
}

// Sends without a parse mode and surfaces the error, so relayed free-form text
// cannot fail on stray markup and the sender learns when delivery was refused.
func (t *Tgbot) sendDirect(chatId int64, text string) error {
	if !isRunning {
		return errors.New("telegram bot is not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: tu.ID(chatId),
		Text:   text,
	})
	return err
}

// Sends with the same HTML parse mode the bot uses everywhere, but surfaces the
// error so admin-authored markup can be rejected before it is stored.
func (t *Tgbot) sendHTMLDirect(chatId int64, text string, replyMarkup ...telego.ReplyMarkup) error {
	if !isRunning {
		return errors.New("telegram bot is not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	params := &telego.SendMessageParams{
		ChatID:              tu.ID(chatId),
		Text:                text,
		ParseMode:           "HTML",
		DisableNotification: t.silent,
	}
	if len(replyMarkup) > 0 {
		params.ReplyMarkup = replyMarkup[0]
	}
	_, err := bot.SendMessage(ctx, params)
	return err
}

// quiet returns a copy of the bot whose sends carry no alert, reading the setting once
// rather than per recipient. A lookup failure keeps the buzz rather than silencing it.
func (t *Tgbot) quiet() *Tgbot {
	silent, err := t.settingService.GetTgBotSilentNotices()
	if err != nil {
		logger.Warning("tgbot: silent notice setting lookup failed:", err)
		silent = false
	}
	scoped := *t
	scoped.silent = silent
	return &scoped
}

// SendMsgToTgbotAdmins sends a message to all admin Telegram chats.
func (t *Tgbot) SendMsgToTgbotAdmins(msg string, replyMarkup ...telego.ReplyMarkup) {
	if len(replyMarkup) > 0 {
		for _, adminId := range adminIds {
			t.SendMsgToTgbot(adminId, msg, replyMarkup[0])
		}
	} else {
		for _, adminId := range adminIds {
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

// SendMsgToTgbotDeleteAfter sends a message and deletes it after a specified delay.
func (t *Tgbot) SendMsgToTgbotDeleteAfter(chatId int64, msg string, delayInSeconds int, replyMarkup ...telego.ReplyMarkup) {
	// Determine if replyMarkup was passed; otherwise, set it to nil
	var replyMarkupParam telego.ReplyMarkup
	if len(replyMarkup) > 0 {
		replyMarkupParam = replyMarkup[0] // Use the first element
	}

	// Send the message
	sentMsg, err := bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      tu.ID(chatId),
		Text:        msg,
		ReplyMarkup: replyMarkupParam, // Use the correct replyMarkup value
	})
	if err != nil {
		logger.Warning("Failed to send message:", err)
		return
	}

	// Delete the sent message after the specified number of seconds.
	go t.deleteMessageAfterDelay(chatId, sentMsg.MessageID, delayInSeconds)
}

// deleteMessageAfterDelay waits delayInSeconds and then removes the message. It
// deliberately does not touch the conversation state: every caller that ends a
// wizard step already clears the state synchronously, and clearing it here — up
// to several seconds later — would wipe a state the user set for the next step
// in the meantime, silently dropping their following input.
func (t *Tgbot) deleteMessageAfterDelay(chatId int64, messageID, delayInSeconds int) {
	time.Sleep(time.Duration(delayInSeconds) * time.Second)
	t.deleteMessageTgBot(chatId, messageID)
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
