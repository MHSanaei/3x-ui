package tgbot

import (
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

// levelOf reads the same binding clientOwnedByTgUser checks, so "is a client"
// and "owns this client" cannot disagree about who is bound.
func (t *Tgbot) levelOf(tgUserID int64) userLevel {
	if checkAdmin(tgUserID) {
		return levelAdmin
	}
	if tgUserID <= 0 {
		return levelStranger
	}
	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
	if err != nil || len(traffics) == 0 {
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
