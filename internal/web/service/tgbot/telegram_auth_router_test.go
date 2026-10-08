package tgbot

import (
	"errors"
	"testing"

	"github.com/mymmrac/telego"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/telegramauth"
)

func TestTelegramAuthStartPayloadSkipsClientInvite(t *testing.T) {
	tb, _ := newLevelTgbot(t)
	previous := telegramauth.Default
	telegramauth.Default = telegramauth.NewService()
	t.Cleanup(func() { telegramauth.Default = previous })

	user := model.User{Username: "panel-admin"}
	if err := database.GetDB().Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	linkCode, _, err := telegramauth.Default.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	tb.answerCommand(commandFrom(777, "/start link_"+linkCode), 777, false)
	if err := database.GetDB().First(&user, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if user.TelegramID != 777 {
		t.Fatalf("Telegram ID = %d, want 777", user.TelegramID)
	}

	loginCode, _, err := telegramauth.Default.StartLogin("browser-csrf", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	tb.answerCommand(commandFrom(777, "/start login_"+loginCode), 777, false)
	if len(telegramAuthApprovePrefix+loginCode) > 64 || len(telegramAuthDenyPrefix+loginCode) > 64 {
		t.Fatal("Telegram callback data exceeds 64 bytes")
	}
	got, pending, err := telegramauth.Default.CompleteLogin(loginCode, "browser-csrf")
	if err != nil || !pending || got != nil {
		t.Fatalf("login before confirmation = %v, %v, %v", got, pending, err)
	}
	callback := &telego.CallbackQuery{
		ID: "confirm", From: telego.User{ID: 777}, Data: telegramAuthApprovePrefix + loginCode,
		Message: &telego.Message{Chat: telego.Chat{ID: 777, Type: telego.ChatTypePrivate}},
	}
	groupCallback := *callback
	groupCallback.Message = &telego.Message{Chat: telego.Chat{ID: -777, Type: telego.ChatTypeSupergroup}}
	if _, ok := tb.gateCallback(&groupCallback); ok {
		t.Fatal("group callback could confirm panel login")
	}
	if _, ok := tb.gateCallback(callback); !ok {
		t.Fatal("linked account could not confirm login")
	}
	tb.answerCallback(callback, false)
	got, pending, err = telegramauth.Default.CompleteLogin(loginCode, "browser-csrf")
	if err != nil || pending || got == nil || got.Id != user.Id {
		t.Fatalf("completed login = %v, %v, %v", got, pending, err)
	}

	deniedCode, _, err := telegramauth.Default.StartLogin("second-browser", "192.0.2.2")
	if err != nil {
		t.Fatal(err)
	}
	tb.answerCommand(commandFrom(777, "/login "+deniedCode), 777, false)
	callback.ID, callback.Data = "deny", telegramAuthDenyPrefix+deniedCode
	tb.answerCallback(callback, false)
	if _, _, err := telegramauth.Default.CompleteLogin(deniedCode, "second-browser"); !errors.Is(err, telegramauth.ErrInvalidCode) {
		t.Fatalf("denied login completed: %v", err)
	}
}
