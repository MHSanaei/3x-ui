package tgbot

import (
	"context"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// recoverBotPanic must be deferred by every bot handler entry point: telego's
// dispatch has no recovery of its own, so one bad update would kill the panel.
func recoverBotPanic() {
	if r := recover(); r != nil {
		logger.Error("Recovered panic in Telegram bot handler:", r)
	}
}

// runBotHandler runs a bot handler on a worker slot and recovers panics: a bad
// callback must not take down the whole panel, the way a cron panic would not.
func runBotHandler(fn func()) {
	messageWorkerPool <- struct{}{}
	defer func() { <-messageWorkerPool }()
	defer recoverBotPanic()
	fn()
}

// chooseInboundClient fetches the inbound once and reuses the row: the inline
// keyboard outlives the inbound, so a stale tap must answer an error, not panic.
// The client list is drawn as a screen, with the pager if it does not fit.
func (t *Tgbot) chooseInboundClient(callbackQuery *telego.CallbackQuery, chatId int64, inboundID int, action string) {
	inbound, err := t.inboundService.GetInbound(inboundID)
	if err != nil {
		logger.Warning("chooseInboundClient GetInbound failed:", err)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getInboundsFailed"))
		return
	}
	clients, err := t.inboundService.GetClients(inbound)
	if err != nil || len(clients) == 0 {
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getClientsFailed"))
		return
	}
	rows := [][]telego.InlineKeyboardButton{}
	for _, client := range clients {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery(action+" "+client.Email))))
	}
	rows = append(rows, t.backRow())
	t.answerSilent(callbackQuery.ID)
	t.renderScreen(chatId, t.newScreen("clients",
		t.inboundSummaryText(inbound)+"\n"+t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), rows...))
}

// OnReceive starts the message receiving loop for the Telegram bot.
func (t *Tgbot) OnReceive() {
	params := telego.GetUpdatesParams{
		Timeout: 20, // Reduced timeout to detect connection issues faster
	}

	// Strict singleton: never start a second long-polling loop.
	tgBotMutex.Lock()
	if botCancel != nil || isRunning {
		tgBotMutex.Unlock()
		logger.Warning("TgBot OnReceive called while already running; ignoring.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	botCancel = cancel
	isRunning = true
	// Add to WaitGroup before releasing the lock so StopBot() can't return
	// before this receiver goroutine is accounted for.
	botWG.Add(1)
	tgBotMutex.Unlock()

	// Get updates channel using the context with shorter timeout for better error recovery
	updates, _ := bot.UpdatesViaLongPolling(ctx, &params)
	go func() {
		defer botWG.Done()
		h, _ := th.NewBotHandler(bot, updates)
		tgBotMutex.Lock()
		botHandler = h
		tgBotMutex.Unlock()

		// The menu marks these as returning to a root screen, so a screen that
		// carries one already offers a way out.
		registerTerminalCallbacks(cbHome, cbCatServer, cbCatClients, cbCatTraffic, cbCatMaintenance)

		// A picked inline item lands as the user's own message: it is caught
		// before every wizard and generic text handler (first match wins).
		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			defer recoverBotPanic()
			t.handleListMarker(&message)
			return nil
		}, th.TextContains(":"), th.AnyMessageWithFrom())

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			defer recoverBotPanic()
			userStateMgr.clear(messageActor(message))
			t.clearScreen(message)
			return nil
		}, th.TextEqual(t.I18nBot("tgbot.buttons.closeKeyboard")))

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			defer recoverBotPanic()
			if !t.isCommandForCurrentBot(&message) {
				return nil
			}

			// Use goroutine with worker pool for concurrent command processing
			go runBotHandler(func() {
				userStateMgr.clear(messageActor(message))
				if isAdmin, ok := t.gateCommand(&message); ok {
					t.answerCommand(&message, message.Chat.ID, isAdmin)
				}
				// The command itself is litter next to the screen it opens: the
				// bot's answer is the message worth keeping. Commands for another
				// bot never get here, so those are left alone.
				t.deleteIncoming(&message)
			})
			return nil
		}, th.AnyCommand())

		// Inline mode is the only list browser: no middleware is registered on
		// it, because a query fires on every keystroke and a middleware that
		// returns nil for an unmatched type silently swallows the list.
		h.HandleInlineQuery(func(ctx *th.Context, query telego.InlineQuery) error {
			defer recoverBotPanic()
			go runBotHandler(func() {
				t.handleInlineQuery(&query)
			})
			return nil
		})

		h.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
			// Use goroutine with worker pool for concurrent callback processing
			go runBotHandler(func() {
				userStateMgr.clear(callbackActor(&query))
				if isAdmin, ok := t.gateCallback(&query); ok {
					t.answerCallback(&query, isAdmin)
				}
			})
			return nil
		}, th.AnyCallbackQueryWithMessage())

		h.HandleMessage(func(ctx *th.Context, message telego.Message) error {
			defer recoverBotPanic()
			userStateMgr.maybePrune(time.Hour)
			actor := messageActor(message)
			if userState, exists := userStateMgr.get(actor); exists {
				if userState == broadcastAwaitingText {
					t.handleBroadcastInput(&message, actor)
					return nil
				}
				// Only a wizard step touches the draft, so only it takes the lock.
				draft := addClientDrafts.forActor(actor)
				draft.Lock()
				defer draft.Unlock()
				switch userState {
				case "awaiting_email":
					if draft.email == strings.TrimSpace(message.Text) {
						t.deleteIncoming(&message)
						userStateMgr.clear(actor)
						return nil
					}

					draft.email = strings.TrimSpace(message.Text)
					if t.isSingleWord(draft.email) {
						userStateMgr.set(actor, "awaiting_email")
						t.deleteIncoming(&message)
						t.wizardInvalidInput(message.Chat.ID, draft)
					} else {
						t.deleteIncoming(&message)
						userStateMgr.clear(actor)
						t.addClient(message.Chat.ID, draft, t.BuildClientDraftMessage(draft))
					}
				case "awaiting_comment":
					if draft.comment == strings.TrimSpace(message.Text) {
						t.deleteIncoming(&message)
						userStateMgr.clear(actor)
						return nil
					}

					draft.comment = strings.TrimSpace(message.Text)
					t.deleteIncoming(&message)
					userStateMgr.clear(actor)
					t.addClient(message.Chat.ID, draft, t.BuildClientDraftMessage(draft))
				case "awaiting_tg_id":
					input := strings.TrimSpace(message.Text)
					if input == "" || input == "-" || strings.EqualFold(input, "none") {
						draft.tgID = ""
						t.deleteIncoming(&message)
						userStateMgr.clear(actor)
						t.addClient(message.Chat.ID, draft, t.BuildClientDraftMessage(draft))
						return nil
					}
					if _, err := strconv.ParseInt(input, 10, 64); err != nil {
						t.deleteIncoming(&message)
						t.wizardInvalidInput(message.Chat.ID, draft)
						return nil
					}
					draft.tgID = input
					t.deleteIncoming(&message)
					userStateMgr.clear(actor)
					t.addClient(message.Chat.ID, draft, t.BuildClientDraftMessage(draft))
				}
			} else {
				if message.UsersShared != nil {
					if checkAdmin(message.From.ID) {
						for _, sharedUser := range message.UsersShared.Users {
							userID := sharedUser.UserID
							needRestart, err := t.clientService.SetClientTelegramUserID(&t.inboundService, message.UsersShared.RequestID, userID)
							if needRestart {
								t.xrayService.SetToNeedRestart()
							}
							output := t.I18nBot("tgbot.messages.userSaved")
							if err != nil {
								output = t.I18nBot("tgbot.messages.selectUserFailed")
							}
							t.sendNoticeNoKeyboard(message.Chat.ID, output)
						}
					} else {
						t.sendNoticeNoKeyboard(message.Chat.ID, t.I18nBot("tgbot.noResult"))
					}
				}
			}
			return nil
		}, th.AnyMessage())

		_ = h.Start()
	}()
}

// answerCommand processes incoming command messages from Telegram users.
func (t *Tgbot) answerCommand(message *telego.Message, chatId int64, isAdmin bool) {
	// onlyMessage is deliberately unused: every command path either renders a
	// screen or sends a notice, both of which carry their own keyboard.
	msg := ""

	command, _, commandArgs := tu.ParseCommand(message.Text)

	// Helper function to handle unknown commands.
	handleUnknownCommand := func() {
		msg += t.I18nBot("tgbot.commands.unknown")
	}

	// Handle the command.
	switch command {
	case "help":
		msg += t.I18nBot("tgbot.commands.help")
		msg += t.I18nBot("tgbot.commands.pleaseChoose")
	case "start":
		if len(commandArgs) > 0 {
			if !isAdmin && !t.allowInviteAttempt(message.From) {
				t.sendNotice(chatId, t.I18nBot("tgbot.messages.inviteRateLimited"))
				return
			}
			t.claimInvite(chatId, message.From.ID, commandArgs[0])
		}
		// A stranger learns only its ChatID, which is what an admin needs to bind it.
		if !isAdmin && t.levelOf(message.From.ID) == levelStranger {
			t.sendNotice(chatId, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(message.From.ID, 10)))
			return
		}
		t.clearBroadcastPrompt(messageActor(*message))
		t.screenHome(chatId, message.From.ID)
		return
	case "status":
		t.screenServer(chatId)
		return
	case "id":
		t.sendNotice(chatId, t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10)))
		return
	case "usage":
		if len(commandArgs) > 0 {
			if isAdmin {
				t.searchClientScreen(chatId, commandArgs[0])
			} else {
				t.getClientUsage(chatId, message.From.ID, commandArgs[0])
			}
		} else {
			t.getClientUsage(chatId, message.From.ID)
		}
		return
	case "inbound":
		if isAdmin && len(commandArgs) > 0 {
			t.searchInbound(chatId, commandArgs[0])
		} else {
			handleUnknownCommand()
		}
	case "restart":
		if isAdmin {
			if len(commandArgs) == 0 {
				if t.xrayService.IsXrayRunning() {
					err := t.xrayService.RestartXray(true)
					if err != nil {
						msg += t.I18nBot("tgbot.commands.restartFailed", "Error=="+err.Error())
					} else {
						msg += t.I18nBot("tgbot.commands.restartSuccess")
					}
				} else {
					msg += t.I18nBot("tgbot.commands.xrayNotRunning")
				}
			} else {
				handleUnknownCommand()
				msg += t.I18nBot("tgbot.commands.restartUsage")
			}
		} else {
			handleUnknownCommand()
		}
	case "clearall":
		if isAdmin {
			t.renderScreen(chatId, t.resetAllConfirm())
			return
		}
		handleUnknownCommand()
	case "broadcast":
		if isAdmin {
			t.startBroadcast(messageActor(*message))
			return
		}
		handleUnknownCommand()
	default:
		handleUnknownCommand()
	}

	if msg != "" {
		t.sendNotice(chatId, msg)
	}
}

func (t *Tgbot) isCommandForCurrentBot(message *telego.Message) bool {
	return isCommandForBot(message.Text, botUsername())
}

func botUsername() string {
	if bot == nil {
		return ""
	}
	return bot.Username()
}

func isCommandForBot(text string, username string) bool {
	_, commandUsername, _ := tu.ParseCommand(text)
	return commandUsername == "" || username == "" || strings.EqualFold(commandUsername, username)
}

// answerCallback processes callback queries from inline keyboards.
func (t *Tgbot) answerCallback(callbackQuery *telego.CallbackQuery, isAdmin bool) {
	chatId := callbackQuery.Message.GetChat().ID
	actor := callbackActor(callbackQuery)

	// Only an admin's wizard callbacks touch a draft, so only they take its lock:
	// a report tap must not wait on a slot, a rejected chat must not be stored.
	var draft *clientDraft
	if isAdmin && isAddClientStep(callbackQuery.Data) {
		draft = addClientDrafts.forActor(actor)
		draft.Lock()
		defer draft.Unlock()
	}

	if isAdmin {
		// get query from hash storage
		decodedQuery, err := t.decodeQuery(callbackQuery.Data)
		if err != nil {
			// A button older than the 20-minute hash window is the common case
			// here; the answer clears it, the message outlives a failed send.
			t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.noQuery"))
			t.sendNotice(chatId, t.I18nBot("tgbot.noQuery"))
			return
		}
		dataArray := strings.Split(decodedQuery, " ")

		if len(dataArray) >= 2 && len(dataArray[1]) > 0 {
			email := dataArray[1]
			switch dataArray[0] {
			case "broadcast_confirm":
				t.confirmBroadcast(actor, dataArray[1], callbackQuery.Message.GetMessageID(), callbackQuery.ID)
			case "get_clients_for_sub":
				inboundIdInt, err := strconv.Atoi(dataArray[1])
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.chooseInboundClient(callbackQuery, chatId, inboundIdInt, "client_sub_links")
			case "get_clients_for_individual":
				inboundIdInt, err := strconv.Atoi(dataArray[1])
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.chooseInboundClient(callbackQuery, chatId, inboundIdInt, "client_individual_links")
			case "get_clients_for_qr":
				inboundIdInt, err := strconv.Atoi(dataArray[1])
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.chooseInboundClient(callbackQuery, chatId, inboundIdInt, "client_qr_links")
			case "client_sub_links":
				t.screenClientLinks(chatId, email)
				return
			case "client_individual_links":
				t.screenIndividualLinks(chatId, email)
				return
			case "client_qr_links":
				t.screenClientQR(chatId, email)
				return
			case "client_get_usage":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.messages.email", "Email=="+email))
				t.searchClientScreen(chatId, email)
			case "client_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clientRefreshSuccess", "Email=="+email))
				t.searchClientScreen(chatId, email)
			case "client_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.searchClientScreen(chatId, email)
			case "ips_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.IpRefreshSuccess", "Email=="+email))
				t.screenClientIps(chatId, email)
			case "ips_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.screenClientIps(chatId, email)
			case "tgid_refresh":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.TGIdRefreshSuccess", "Email=="+email))
				t.screenClientTG(chatId, email, false)
			case "tgid_cancel":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
				t.screenClientTG(chatId, email, false)
			case "reset_traffic":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmResetTraffic")).WithCallbackData(t.encodeQuery("reset_traffic_c "+email)),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "reset_traffic_c":
				err := t.inboundService.ResetClientTrafficByEmail(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetTrafficSuccess", "Email=="+email))
					t.searchClientScreen(chatId, email)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				}
			case "limit_traffic":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 1")),
						tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 5")),
						tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 10")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 20")),
						tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 30")),
						tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 40")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 50")),
						tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 60")),
						tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 80")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 100")),
						tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 150")),
						tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 200")),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "limit_traffic_c":
				if len(dataArray) == 3 {
					limitTraffic, err := strconv.Atoi(dataArray[2])
					if err == nil {
						needRestart, err := t.clientService.ResetClientTrafficLimitByEmail(&t.inboundService, email, limitTraffic)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.setTrafficLimitSuccess", "Email=="+email))
							t.searchClientScreen(chatId, email)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "limit_traffic_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "limit_traffic", dataArgs: email + " ", cancelData: "client_cancel " + email, confirmLabelKey: "tgbot.buttons.confirmNumberAdd"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "add_client_limit_traffic_c":
				limitTraffic, _ := strconv.ParseInt(dataArray[1], 10, 64)
				draft.totalGB = limitTraffic * 1024 * 1024 * 1024
				messageId := callbackQuery.Message.GetMessageID()
				message_text := t.BuildClientDraftMessage(draft)

				t.addClient(callbackQuery.Message.GetChat().ID, draft, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_limit_traffic_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "add_client_limit_traffic", cancelData: "add_client_default_traffic_exp", confirmLabelKey: "tgbot.buttons.confirmNumberAdd"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
			case "reset_exp":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("reset_exp_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 7")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 10")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 14")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 20")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 30")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 90")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 180")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 365")),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "reset_exp_c":
				if len(dataArray) == 3 {
					days, err := strconv.ParseInt(dataArray[2], 10, 64)
					if err == nil {
						var date int64
						if days > 0 {
							traffic, err := t.inboundService.GetClientTrafficByEmail(email)
							if err != nil {
								logger.Warning(err)
								t.sendNotice(chatId, t.I18nBot("tgbot.wentWrong"))
								return
							}
							if traffic == nil {
								t.sendNotice(chatId, t.I18nBot("tgbot.noResult"))
								return
							}

							if traffic.ExpiryTime > 0 {
								if traffic.ExpiryTime-time.Now().Unix()*1000 < 0 {
									date = -(days * 24 * 60 * 60000)
								} else {
									date = traffic.ExpiryTime + days*24*60*60000
								}
							} else {
								date = traffic.ExpiryTime - days*24*60*60000
							}

						}
						needRestart, err := t.clientService.ResetClientExpiryTimeByEmail(&t.inboundService, email, date)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.expireResetSuccess", "Email=="+email))
							t.searchClientScreen(chatId, email)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "reset_exp_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "reset_exp", dataArgs: email + " ", cancelData: "client_cancel " + email, confirmLabelKey: "tgbot.buttons.confirmNumber"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "add_client_reset_exp_c":
				// The wizard's presets and its custom keypad land in this one case, so a
				// second tap replaces the term it set; 0 is the Unlimited button.
				days, _ := strconv.ParseInt(dataArray[1], 10, 64)
				draft.expiryTime = -days * 24 * 60 * 60000

				messageId := callbackQuery.Message.GetMessageID()
				message_text := t.BuildClientDraftMessage(draft)

				t.addClient(callbackQuery.Message.GetChat().ID, draft, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_reset_exp_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "add_client_reset_exp", cancelData: "add_client_default_traffic_exp", confirmLabelKey: "tgbot.buttons.confirmNumberAdd"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
			case "ip_limit":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelIpLimit")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 0")),
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("ip_limit_in "+email+" 0")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 1")),
						tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 2")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 3")),
						tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 4")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 5")),
						tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 6")),
						tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 7")),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 8")),
						tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 9")),
						tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 10")),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "ip_limit_c":
				if len(dataArray) == 3 {
					count, err := strconv.Atoi(dataArray[2])
					if err == nil {
						needRestart, err := t.clientService.ResetClientIpLimitByEmail(&t.inboundService, email, count)
						if needRestart {
							t.xrayService.SetToNeedRestart()
						}
						if err == nil {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetIpSuccess", "Email=="+email, "Count=="+strconv.Itoa(count)))
							t.searchClientScreen(chatId, email)
							return
						}
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "ip_limit_in":
				if len(dataArray) >= 3 {
					oldInputNumber, err := strconv.Atoi(dataArray[2])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 4 {
							num, err := strconv.Atoi(dataArray[3])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "ip_limit", dataArgs: email + " ", cancelData: "client_cancel " + email, confirmLabelKey: "tgbot.buttons.confirmNumber"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
				t.searchClientScreen(chatId, email)
			case "add_client_ip_limit_c":
				if len(dataArray) == 2 {
					count, _ := strconv.Atoi(dataArray[1])
					draft.limitIP = count
				}

				messageId := callbackQuery.Message.GetMessageID()
				message_text := t.BuildClientDraftMessage(draft)

				t.addClient(callbackQuery.Message.GetChat().ID, draft, message_text, messageId)
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
			case "add_client_ip_limit_in":
				if len(dataArray) >= 2 {
					oldInputNumber, err := strconv.Atoi(dataArray[1])
					inputNumber := oldInputNumber
					if err == nil {
						if len(dataArray) == 3 {
							num, err := strconv.Atoi(dataArray[2])
							if err == nil {
								inputNumber = updateNumericInput(inputNumber, num)
							}
							if inputNumber == oldInputNumber {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
								return
							}
							if inputNumber >= 999999 {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
								return
							}
						}
						inlineKeyboard := t.numericKeypad(numericKeypadSpec{dataBase: "add_client_ip_limit", cancelData: "add_client_default_ip_limit", confirmLabelKey: "tgbot.buttons.confirmNumber"}, inputNumber)
						t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
						return
					}
				}
			case "clear_ips":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("ips_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmClearIps")).WithCallbackData(t.encodeQuery("clear_ips_c "+email)),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "clear_ips_c":
				err := t.inboundService.ClearClientIps(email)
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clearIpSuccess", "Email=="+email))
					t.screenClientIps(chatId, email)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.screenClientIps(chatId, email)
				}
			case "ip_log":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getIpLog", "Email=="+email))
				t.screenClientIps(chatId, email)
			case "tgid_pick":
				userStateMgr.set(actor, "awaiting_tg_pick")
				t.answerSilent(callbackQuery.ID)
				t.screenClientTG(chatId, email, false)
				t.sendTGPicker(chatId, t.clientTrafficID(email))
			case "tg_user":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getUserInfo", "Email=="+email))
				t.screenClientTG(chatId, email, false)
			case "client_invite_link":
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.inviteLink"))
				t.screenInviteLink(chatId, email)
			case "tgid_remove":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("tgid_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmRemoveTGUser")).WithCallbackData(t.encodeQuery("tgid_remove_c "+email)),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "tgid_remove_c":
				traffic, err := t.inboundService.GetClientTrafficByEmail(email)
				if err != nil || traffic == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					return
				}
				needRestart, err := t.clientService.SetClientTelegramUserID(&t.inboundService, traffic.Id, EmptyTelegramUserID)
				if needRestart {
					t.xrayService.SetToNeedRestart()
				}
				if err == nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.removedTGUserSuccess", "Email=="+email))
					t.screenClientTG(chatId, email, false)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.screenClientTG(chatId, email, false)
				}
			case "toggle_enable":
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmToggle")).WithCallbackData(t.encodeQuery("toggle_enable_c "+email)),
					),
				)
				t.renderPickerOnClient(chatId, email, inlineKeyboard.InlineKeyboard)
			case "toggle_enable_c":
				enabled, needRestart, err := t.clientService.ToggleClientEnableByEmail(&t.inboundService, email)
				if needRestart {
					t.xrayService.SetToNeedRestart()
				}
				if err == nil {
					if enabled {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.enableSuccess", "Email=="+email))
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.disableSuccess", "Email=="+email))
					}
					t.searchClientScreen(chatId, email)
				} else {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClientScreen(chatId, email)
				}
			case "get_clients":
				inboundIdInt, err := strconv.Atoi(dataArray[1])
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.answerSilent(callbackQuery.ID)
				t.screenInboundClients(chatId, inboundIdInt)
			case "add_client_to":
				inboundIdInt, err := strconv.Atoi(dataArray[1])
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.resetDraft(draft)
				draft.receiverInboundID = inboundIdInt
				draft.receiverInboundIDs = []int{inboundIdInt}
				t.addClient(callbackQuery.Message.GetChat().ID, draft, t.BuildClientDraftMessage(draft))
			case "add_client_toggle_attach":
				inboundIdStr := dataArray[1]
				inboundIdInt, err := strconv.Atoi(inboundIdStr)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				found := -1
				for i, id := range draft.receiverInboundIDs {
					if id == inboundIdInt {
						found = i
						break
					}
				}
				if found >= 0 {
					draft.receiverInboundIDs = append(draft.receiverInboundIDs[:found], draft.receiverInboundIDs[found+1:]...)
				} else {
					draft.receiverInboundIDs = append(draft.receiverInboundIDs, inboundIdInt)
				}
				picker, err := t.getInboundsAttachPicker(draft)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return
				}
				t.renderDraftPicker(callbackQuery.Message.GetChat().ID, draft, picker.InlineKeyboard)
			default:
				// An unknown action with arguments is still a tap, and an
				// unanswered tap spins until Telegram times it out.
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
			}
			return
		} else {
			switch callbackQuery.Data {
			case "broadcast_cancel":
				t.cancelBroadcast(actor, callbackQuery.Message.GetMessageID(), callbackQuery.ID)
				// The broadcast step owns its own message; nothing else runs.
				return
			case "get_inbounds":
				t.screenInbounds(chatId, callbackQuery.From.ID)
			case "admin_client_sub_links", "admin_client_individual_links", "admin_client_qr_links":
				t.browseInboundsScreen(chatId, callbackQuery.From.ID)
			}
		}
	}

	if !isAdmin {
		// encodeQuery hashes any payload past 64 chars, so a long email's button
		// must be decoded before the gate can see which client it names.
		if decoded, err := t.decodeQuery(callbackQuery.Data); err == nil {
			callbackQuery.Data = decoded
		}
		if !isClientSelfCallback(callbackQuery.Data) {
			return
		}
	}

	switch callbackQuery.Data {
	case "home":
		t.screenHome(chatId, callbackQuery.From.ID)
	case cbCatServer, cbCatClients, cbCatTraffic, cbCatMaintenance:
		t.screenCategory(chatId, strings.TrimPrefix(callbackQuery.Data, "cat:"))
	case "act":
		// The old "Actions" screen folded into the Maintenance category.
		t.screenCategory(chatId, "maintenance")
	case "hide":
		t.hideMessage(callbackQuery)
	case "pg:next":
		t.turnPage(callbackQuery, 1)
	case "pg:prev":
		t.turnPage(callbackQuery, -1)
	case "pg:none":
		t.answerSilent(callbackQuery.ID)
	case "srv":
		t.answerSilent(callbackQuery.ID)
		t.screenServer(chatId)
	case "inb", "inbounds", "get_inbounds":
		t.screenInbounds(chatId, callbackQuery.From.ID)
	case "cli":
		t.answerSilent(callbackQuery.ID)
		t.screenAllClients(chatId)
	case "onl", "onlines", "onlines_refresh":
		t.answerSilent(callbackQuery.ID)
		t.screenOnlines(chatId)
	case "dep", "deplete_soon":
		t.screenDeplete(chatId)
	case "rep", "get_sorted_traffic_usage_report":
		t.screenReport(chatId)
	case "bkp", "get_backup":
		t.answerSilent(callbackQuery.ID)
		t.sendBackupScreen(chatId)
	case "ban", "get_banlogs":
		t.answerSilent(callbackQuery.ID)
		t.screenBanLogs(chatId)
	case "inline_help":
		t.answerSilent(callbackQuery.ID)
		t.screenInlineHelp(chatId)
	case "inline_recheck":
		t.refreshInlineCapability()
		if t.inlineSupported() {
			t.screenInbounds(chatId, callbackQuery.From.ID)
		} else {
			t.screenInlineHelp(chatId)
		}
	case "cmd", "commands", "client_commands":
		t.screenCommands(chatId, isAdmin)
	case "addc", "add_client":
		t.screenAddClientStart(chatId, callbackQuery.From.ID)
	case cbBroadcast:
		// The broadcast flow already existed behind /broadcast; the button is
		// the discoverable way in. It owns its own messages, so no screen here.
		t.answerSilent(callbackQuery.ID)
		t.startBroadcast(actor)
	case "xray_restart":
		t.restartXrayFromScreen(callbackQuery)
	case "rst_all", "reset_all_traffics":
		t.renderScreen(chatId, t.resetAllConfirm())
	case "client_traffic":
		t.answerSilent(callbackQuery.ID)
		t.getClientUsage(chatId, callbackQuery.From.ID)
	case "client_sub_links", "client_individual_links", "client_qr_links":
		// A client tapped its own link button: its identity already says which
		// subscription, so it opens directly. A second one still needs a choice.
		t.openOwnClientScreen(chatId, callbackQuery.From.ID, callbackQuery.Data)
	case "add_client_ch_default_email":
		userStateMgr.set(actor, "awaiting_email")
		t.wizardPrompt(chatId, draft, t.I18nBot("tgbot.messages.email_prompt", "ClientEmail=="+html.EscapeString(draft.email)))
	case "add_client_ch_default_comment":
		userStateMgr.set(actor, "awaiting_comment")
		t.wizardPrompt(chatId, draft, t.I18nBot("tgbot.messages.comment_prompt", "ClientComment=="+html.EscapeString(draft.comment)))
	case "add_client_ch_default_tg_id":
		userStateMgr.set(actor, "awaiting_tg_id")
		current := draft.tgID
		if current == "" {
			current = "—"
		}
		t.wizardPrompt(chatId, draft, t.I18nBot("tgbot.messages.tgid_prompt", "ClientTGID=="+html.EscapeString(current)))
	case "add_client_ch_default_traffic":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 1")),
				tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 5")),
				tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 20")),
				tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 30")),
				tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 40")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 50")),
				tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 60")),
				tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 80")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 100")),
				tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 150")),
				tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 200")),
			),
		)
		t.renderDraftPicker(chatId, draft, inlineKeyboard.InlineKeyboard)
	case "add_client_ch_default_exp":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_reset_exp_in 0")),
			),
			// No "Add" verb: these replace the term the draft carries, unlike the
			// renewal keyboard, whose reset_exp_c handler really does add to it.
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 7")),
				tu.InlineKeyboardButton("10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 14")),
				tu.InlineKeyboardButton("20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 20")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 30")),
				tu.InlineKeyboardButton("3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 90")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 180")),
				tu.InlineKeyboardButton("12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 365")),
			),
		)
		t.renderDraftPicker(chatId, draft, inlineKeyboard.InlineKeyboard)
	case "add_client_ch_default_ip_limit":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_ip_limit")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_ip_limit_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_ip_limit_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 1")),
				tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 2")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 3")),
				tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 4")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 5")),
				tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 6")),
				tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 7")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 8")),
				tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 9")),
				tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 10")),
			),
		)
		t.renderDraftPicker(chatId, draft, inlineKeyboard.InlineKeyboard)
	case "add_client_default_info":
		userStateMgr.clear(actor)
		t.addClient(chatId, draft, t.BuildClientDraftMessage(draft), callbackQuery.Message.GetMessageID())
	case "add_client_cancel":
		userStateMgr.clear(actor)
		addClientDrafts.reset(actor)
		t.renderScreen(chatId, t.newScreen("main", t.I18nBot("tgbot.messages.cancel"), t.homeRows()...))
	case "add_client_default_traffic_exp", "add_client_default_ip_limit":
		// The picker was cancelled: the draft comes back unchanged.
		t.addClient(chatId, draft, t.BuildClientDraftMessage(draft), callbackQuery.Message.GetMessageID())
	case "add_client_attach_more":
		picker, err := t.getInboundsAttachPicker(draft)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		t.renderDraftPicker(chatId, draft, picker.InlineKeyboard)
	case "add_client_attach_done":
		if draft.receiverInboundID == 0 && len(draft.receiverInboundIDs) > 0 {
			draft.receiverInboundID = draft.receiverInboundIDs[0]
		}
		if draft.receiverInboundID == 0 {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getInboundsFailed"))
			return
		}
		t.addClient(chatId, draft, t.BuildClientDraftMessage(draft), callbackQuery.Message.GetMessageID())
	case "add_client_submit_disable", "add_client_submit_enable":
		// Both submits land here and differ only in the enable flag; the data is
		// short enough never to be hashed, so its raw form is the discriminator.
		draft.enable = callbackQuery.Data == "add_client_submit_enable"
		if _, err := t.SubmitAddClient(draft); err != nil {
			t.sendNotice(chatId, t.I18nBot("tgbot.messages.error_add_client", "error=="+err.Error()))
			return
		}
		email := draft.email
		addClientDrafts.reset(actor)
		// The new client's card is the screen: the links and the QR hang off it,
		// so nobody has to go looking for the client they just created.
		t.searchClientScreen(chatId, email)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
	case "reset_all_traffics_cancel":
		t.renderScreen(chatId, t.newScreen("main", t.I18nBot("tgbot.messages.cancel"), t.homeRows()...))
	case "reset_all_traffics_c":
		emails, err := t.inboundService.GetAllEmails()
		if err != nil {
			t.sendNotice(chatId, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}

		// One report per tap, not one message per client: a large panel would
		// otherwise burst past Telegram's rate limit; the notice pages it.
		var report strings.Builder
		for _, email := range emails {
			if err := t.inboundService.ResetClientTrafficByEmail(email); err == nil {
				report.WriteString(t.I18nBot("tgbot.messages.SuccessResetTraffic", "ClientEmail=="+email))
			} else {
				report.WriteString(t.I18nBot("tgbot.messages.FailedResetTraffic", "ClientEmail=="+email, "ErrorMessage=="+err.Error()))
			}
			report.WriteString("\r\n\r\n")
		}
		report.WriteString(t.I18nBot("tgbot.messages.FinishProcess"))

		// Escaped whole: one stray "<" in a remark or email otherwise makes
		// Telegram reject the page it landed on, losing ~15 clients at once.
		t.sendNotice(chatId, html.EscapeString(report.String()))
		// The confirmation has served its purpose: leaving it on screen left the
		// only button being the one just pressed.
		t.screenHome(chatId, callbackQuery.From.ID)
	default:
		action, email, ok := splitClientLinkCallback(callbackQuery.Data)
		if !ok {
			// Nothing matched: an unknown button still has to be answered, or it
			// keeps spinning until Telegram times the callback out.
			t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}
		// The keyboard outlives the chat it was sent to, so the email in it
		// cannot authorise itself: a non-admin only reaches their own clients.
		if !isAdmin && !t.clientOwnedByTgUser(callbackQuery.From.ID, email) {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}
		switch action {
		case "client_sub_links":
			t.screenClientLinks(chatId, email)
		case "client_individual_links":
			t.screenIndividualLinks(chatId, email)
		case "client_qr_links":
			t.screenClientQR(chatId, email)
		}
	}
}

// checkAdmin checks if the given Telegram ID is an admin.
func checkAdmin(tgId int64) bool {
	return slices.Contains(adminSnapshot(), tgId)
}

// isClientSelfCallback reports whether a callback is per-user rather than
// admin-only; the caller still has to prove the client is its own.
func isClientSelfCallback(data string) bool {
	switch data {
	// "hide" is how a recipient — client or admin — puts a message away. It
	// carries no target of its own (it is the message it sits on), so it is safe
	// for anyone to reach; without it a broadcast copy could not be dismissed.
	case "home", "hide", "client_traffic", "client_commands", "client_sub_links",
		"client_individual_links", "client_qr_links":
		return true
	}
	_, _, ok := splitClientLinkCallback(data)
	return ok
}

// splitClientLinkCallback splits "<action> <email>" for the per-client link
// callbacks; ok is false for every other data.
func splitClientLinkCallback(data string) (action, email string, ok bool) {
	for _, candidate := range []string{"client_sub_links", "client_individual_links", "client_qr_links"} {
		if rest, found := strings.CutPrefix(data, candidate+" "); found && rest != "" {
			return candidate, rest, true
		}
	}
	return "", "", false
}
