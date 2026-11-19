package tgbot

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// userLevel decides what the bot admits to existing at all: a Telegram account
// that no admin has bound to a client must not be able to explore the liveBot().
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

// sentForChat reports that no account stands behind message.From: a post by an
// anonymous admin or a channel carries a placeholder user shared by everybody.
func sentForChat(message *telego.Message) bool {
	return message.SenderChat != nil || message.From.IsBot
}

// gateCommand reports whether a command reaches answerCommand, and as whom. A
// stranger's refused command gets no reply, so the bot reveals nothing to probe.
func (t *Tgbot) gateCommand(message *telego.Message) (isAdmin bool, ok bool) {
	level := levelStranger
	if !sentForChat(message) {
		level = t.levelOf(message.From.ID)
	}
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
