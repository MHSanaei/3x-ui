package tgbot

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// userLevel decides what the bot admits to existing at all: a Telegram account
// that no admin has bound to a client must not be able to explore the bot.
type userLevel int

const (
	levelStranger userLevel = iota
	levelClient
	levelAdmin
)

// levelOf runs on every update, a stranger's included, so it reads the indexed
// tg_id column of the clients table rather than expanding every inbound's JSON.
func (t *Tgbot) levelOf(tgUserID int64) userLevel {
	if checkAdmin(tgUserID) {
		return levelAdmin
	}
	if tgUserID <= 0 {
		return levelStranger
	}
	records, err := t.clientService.GetRecordsByTgID(tgUserID)
	if err != nil || len(records) == 0 {
		return levelStranger
	}
	return levelClient
}

// Commands are allowlisted rather than denied one by one: a command added later
// stays out of reach of non-admins until it is deliberately listed here.
var commandsByLevel = map[userLevel]map[string]bool{
	// /id stays open because an admin binding by hand still asks for the ChatID.
	levelStranger: {"start": true, "id": true},
	levelClient:   {"start": true, "help": true, "status": true, "id": true, "usage": true},
}

func commandAllowed(level userLevel, command string) bool {
	if level == levelAdmin {
		return true
	}
	return commandsByLevel[level][command]
}

// Authorization keys on the sender but conversation state keys on the chat, and
// those are the same identity only in a private chat.
func isPrivateChat(chat telego.Chat) bool {
	return chat.Type == telego.ChatTypePrivate
}

var loggedNonPrivateChats sync.Map

// ignoredChat reports whether an update is dropped for its chat type. Each such
// chat is logged once, so an admin used to a group can find why the bot is quiet.
func ignoredChat(chat telego.Chat) bool {
	if isPrivateChat(chat) {
		return false
	}
	if _, seen := loggedNonPrivateChats.LoadOrStore(chat.ID, struct{}{}); !seen {
		logger.Infof("tgbot: ignoring %s chat %d; the bot answers only in private chats", chat.Type, chat.ID)
	}
	return true
}

// gateCommand reports whether a command reaches answerCommand, and as whom. A
// stranger's refused command gets no reply, so the bot reveals nothing to probe.
func (t *Tgbot) gateCommand(message *telego.Message) (isAdmin bool, ok bool) {
	level := t.levelOf(message.From.ID)
	command, _, _ := tu.ParseCommand(message.Text)
	if commandAllowed(level, command) {
		return level == levelAdmin, true
	}
	if level == levelClient {
		t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.commands.unknown"))
	}
	return false, false
}

// gateCallback answers a stranger's tap without acting on it: a stranger holds
// no keyboard of ours, so any callback data from one is forged or stale.
func (t *Tgbot) gateCallback(query *telego.CallbackQuery) (isAdmin bool, ok bool) {
	level := t.levelOf(query.From.ID)
	if level == levelStranger {
		t.sendCallbackAnswerTgBot(query.ID, "")
		return false, false
	}
	return level == levelAdmin, true
}
