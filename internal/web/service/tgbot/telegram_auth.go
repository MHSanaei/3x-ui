package tgbot

import (
	"html"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/telegramauth"
)

const (
	telegramAuthApprovePrefix = "auth_ok_"
	telegramAuthDenyPrefix    = "auth_no_"
)

func isTelegramAuthCallback(data string) bool {
	return strings.HasPrefix(data, telegramAuthApprovePrefix) || strings.HasPrefix(data, telegramAuthDenyPrefix)
}

func (t *Tgbot) telegramAuthCommand(message *telego.Message, command, code string) string {
	if message.Chat.Type != "private" || message.From == nil || message.Chat.ID != message.From.ID {
		return t.I18nBot("tgbot.authPrivateOnly")
	}
	if code == "" {
		return t.I18nBot("tgbot.authInvalidCode")
	}
	var err error
	switch command {
	case "link":
		err = telegramauth.Default.ConsumeLink(code, message.From.ID)
		if err == nil {
			logger.Infof("Telegram account linked to panel admin: telegram_id=%d", message.From.ID)
			return t.I18nBot("tgbot.authLinked")
		}
	case "login":
		var ip string
		ip, err = telegramauth.Default.PrepareLogin(code, message.From.ID)
		if err == nil {
			keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.authConfirmButton")).WithCallbackData(telegramAuthApprovePrefix+code),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.authDenyButton")).WithCallbackData(telegramAuthDenyPrefix+code),
			))
			t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.authConfirmRequest",
				"Code=="+html.EscapeString(code), "IP=="+html.EscapeString(ip)), keyboard)
			return ""
		}
	}
	return t.I18nBot("tgbot.authInvalidCode")
}

func (t *Tgbot) telegramAuthCallback(query *telego.CallbackQuery) {
	if query.Message == nil || query.Message.GetChat().Type != telego.ChatTypePrivate || query.Message.GetChat().ID != query.From.ID {
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.authPrivateOnly"))
		return
	}
	code, approve := strings.CutPrefix(query.Data, telegramAuthApprovePrefix)
	if !approve {
		code, _ = strings.CutPrefix(query.Data, telegramAuthDenyPrefix)
	}
	var err error
	response := "tgbot.authDenied"
	if approve {
		err = telegramauth.Default.ApproveLogin(code, query.From.ID)
		response = "tgbot.authApproved"
	} else {
		err = telegramauth.Default.RejectLogin(code, query.From.ID)
	}
	if err != nil {
		response = "tgbot.authInvalidCode"
	} else if approve {
		logger.Infof("Telegram panel login approved: telegram_id=%d", query.From.ID)
	}
	message := t.I18nBot(response)
	t.sendCallbackAnswerTgBot(query.ID, message)
	t.SendMsgToTgbot(query.From.ID, message)
}
