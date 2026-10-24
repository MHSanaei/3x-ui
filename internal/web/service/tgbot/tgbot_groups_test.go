package tgbot

import (
	"context"
	"testing"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

const testAdminID = 1

func privateMessage(id int, text string) telego.Update {
	return telego.Update{UpdateID: id, Message: &telego.Message{
		MessageID: id, Text: text,
		Chat: telego.Chat{ID: testAdminID, Type: telego.ChatTypePrivate},
		From: &telego.User{ID: testAdminID, FirstName: "a"},
	}}
}

func groupMessage(id int, text string) telego.Update {
	return telego.Update{UpdateID: id, Message: &telego.Message{
		MessageID: id, Text: text,
		Chat: telego.Chat{ID: -900, Type: telego.ChatTypeSupergroup},
		From: &telego.User{ID: testAdminID, FirstName: "a"},
	}}
}

func groupCallback(id int, data string) telego.Update {
	return telego.Update{UpdateID: id, CallbackQuery: &telego.CallbackQuery{
		ID: "cq", Data: data,
		From: telego.User{ID: testAdminID, FirstName: "a"},
		Message: &telego.Message{
			MessageID: id,
			Chat:      telego.Chat{ID: -900, Type: telego.ChatTypeSupergroup},
		},
	}}
}

// A group shares one screen and keyboard among all members, so admins and clients overwrite each other;
// the guard must answer a group and stop it before any screen is drawn.
func TestGroupUpdatesAreRefused(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	h, err := th.NewBotHandler(liveBot(), make(chan telego.Update, 1))
	if err != nil {
		t.Fatalf("NewBotHandler: %v", err)
	}
	refuseGroups(h, tb)

	for _, update := range []telego.Update{
		groupMessage(1, "/start"),
		groupCallback(2, cbHome),
		{UpdateID: 3, InlineQuery: &telego.InlineQuery{ID: "q", ChatType: telego.ChatTypeGroup}},
	} {
		if err := h.BaseGroup().HandleUpdate(context.Background(), liveBot(), update); err != nil {
			t.Fatalf("HandleUpdate: %v", err)
		}
	}

	got := calls()
	if indexOf(got, "sendPhoto") >= 0 || indexOf(got, "editMessageText") >= 0 {
		t.Fatalf("a group drew a screen: %v", methods(got))
	}
	want := tb.I18nBot("tgbot.messages.privateOnly")
	sawNotice := false
	for _, c := range got {
		if c.Method == "sendMessage" && c.text() == want {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Fatalf("the group was not told why nothing happened: %v", methods(got))
	}
	if indexOf(got, "answerCallbackQuery") < 0 {
		t.Fatalf("the group callback was left unanswered: %v", methods(got))
	}
	if indexOf(got, "answerInlineQuery") < 0 {
		t.Fatalf("the group inline query was left unanswered: %v", methods(got))
	}
}

// A private update must fall through the guard untouched: the close-keyboard
// route runs synchronously, so its delete is visible without a worker goroutine.
func TestPrivateChatIsNotRefused(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	h, err := th.NewBotHandler(liveBot(), make(chan telego.Update, 1))
	if err != nil {
		t.Fatalf("NewBotHandler: %v", err)
	}
	refuseGroups(h, tb)
	h.HandleMessage(func(*th.Context, telego.Message) error { return nil },
		th.TextEqual(tb.I18nBot("tgbot.buttons.closeKeyboard")))

	if err := h.BaseGroup().HandleUpdate(context.Background(), liveBot(),
		privateMessage(1, tb.I18nBot("tgbot.buttons.closeKeyboard"))); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}

	got := calls()
	for _, c := range got {
		if c.text() == tb.I18nBot("tgbot.messages.privateOnly") {
			t.Fatalf("a private chat was refused: %v", methods(got))
		}
	}
}

// updateChatType is the guard's only input, so it is pinned per update shape.
func TestUpdateChatType(t *testing.T) {
	cases := []struct {
		name string
		up   telego.Update
		want string
	}{
		{"private message", privateMessage(1, "x"), telego.ChatTypePrivate},
		{"group message", groupMessage(1, "x"), telego.ChatTypeSupergroup},
		{"inline from private chat (sender)", telego.Update{InlineQuery: &telego.InlineQuery{
			ID: "q", ChatType: "sender",
		}}, telego.ChatTypePrivate},
		{"inline from a group", telego.Update{InlineQuery: &telego.InlineQuery{
			ID: "q", ChatType: telego.ChatTypeGroup,
		}}, telego.ChatTypeGroup},
		{"empty inline chat type", telego.Update{InlineQuery: &telego.InlineQuery{ID: "q"}}, telego.ChatTypePrivate},
		{"unknown update", telego.Update{}, ""},
	}
	for _, tc := range cases {
		if got := updateChatType(tc.up); got != tc.want {
			t.Errorf("%s: updateChatType = %q, want %q", tc.name, got, tc.want)
		}
	}
}
